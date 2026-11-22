// Package orgsapart holds G-025: nothing of one organization is ever shown to, sent
// to or changed by another. Two organizations have the same people, channel names,
// message IDs, action IDs and change numbers, and differ only in a secret word in
// their text. The control panel's list of organizations is for the operator and is
// the stated exception; it is not asked here.
package orgsapart

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/e2e/stack"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	people   = []string{"anna", "marco", "lena", "release-agent"}
	channels = []string{"#launch", "#ops"}
)

const onlyBolt = "msg_only_bolt"

type org struct {
	*stack.Org
	secret  string
	numbers []int64
}

func populate(t testing.TB, o *org) {
	t.Helper()
	do := func(as, kind string, f map[string]any) { o.numbers = append(o.numbers, o.Do(as, kind, f)) }
	root := "msg_shared"
	do("anna", "send", stack.Message(root, "#launch", "@Marco the "+o.secret+" plan", nil, "marco"))
	do("marco", "send", stack.Message("msg_reply", "#launch", "reply about "+o.secret, &root))
	do("lena", "react", map[string]any{"message": root, "emoji": "eyes", "on": true})
	do("anna", "send", stack.Message("msg_edited", "#ops", "first words", nil))
	do("anna", "edit", stack.Edit("msg_edited", "edited "+o.secret))
	do("anna", "send", stack.Message("msg_deleted", "#ops", "gone "+o.secret, nil))
	do("anna", "delete", stack.Delete("msg_deleted"))
	do("anna", "send", stack.Message("msg_lena", "#launch", "@Lena look at "+o.secret, nil, "lena"))
	do("anna", "send", stack.Message("msg_agent", "#launch", "@release-agent ship "+o.secret, nil, "release-agent"))
}

// surfaces is every question a screen, an agent, a neighbouring part or the
// notification app can ask for this organization.
func surfaces(o *org, foreign string) []string {
	s := o.S
	base := "/orgs/" + o.Name
	var out []string
	door := func(rest string) { out = append(out, s.Door+base+rest) }
	launch := url.PathEscape("#launch")
	door("/people")
	door("/suggest?q=a&as=anna")
	for _, c := range channels {
		door("/conversations/" + url.PathEscape(c) + "/messages?as=anna")
	}
	door("/conversations/" + launch + "/messages?as=anna&thread=msg_shared")
	door("/agents/release-agent/events?after=0&wait=0")
	door("/agents/release-agent/conversations/" + launch + "/messages")
	for _, p := range people {
		pp := "/people/" + url.PathEscape(p)
		door(pp + "/start")
		door(pp + "/feed")
		door(pp + "/settings")
		door(pp + "/preview?change=" + url.QueryEscape(`{"kind":"dial","conversation":"#launch","thread":null,"level":"fyi"}`))
		door(pp + "/search?q=" + url.QueryEscape(foreign))
		door(pp + "/search?q=" + url.QueryEscape(o.secret))
		for _, tab := range []string{"direct", "mentions", "threads", "reactions"} {
			door(pp + "/history?tab=" + tab)
			out = append(out, s.Feed+base+pp+"/history?tab="+tab)
		}
		out = append(out, s.Feed+base+pp+"/rows", s.Feed+base+pp+"/settings",
			s.Search+base+"/query?q="+url.QueryEscape(foreign)+"&as="+url.QueryEscape(p),
			s.Sender+base+"/notifications?person="+url.QueryEscape(p))
	}
	out = append(out, s.Feed+base+"/place", s.Feed+base+"/updates?after=0&edition=1&limit=500", s.Search+base+"/place", s.Sender+base+"/place")
	for _, d := range s.Deliveries {
		out = append(out, d.URL+base+"/status", d.URL+base+"/parked")
	}
	return out
}

func nothingOf(t testing.TB, o *org, foreign string) {
	t.Helper()
	for _, address := range surfaces(o, foreign) {
		status, body, err := stack.Call(http.MethodGet, address, nil)
		if err != nil {
			t.Fatalf("GET %s: %v", address, err)
		}
		if strings.Contains(string(body), foreign) {
			t.Fatalf("%s holds something of the other organization: GET %s answered %d with %.300s", o.Name, address, status, body)
		}
		// A question that was not answered proves nothing. Only the preview may say
		// "not available right now" (screen-to-conversations.md).
		if status != http.StatusOK && !(status == http.StatusServiceUnavailable && strings.Contains(address, "/preview")) {
			t.Fatalf("GET %s answered %d, so it shows nothing about keeping organizations apart: %.200s", address, status, body)
		}
	}
}

// ownIsVisible keeps the check honest: the same questions do reach this
// organization's own content.
func ownIsVisible(t testing.TB, o *org) {
	t.Helper()
	base := o.S.Door + "/orgs/" + o.Name
	for _, address := range []string{
		base + "/conversations/" + url.PathEscape("#launch") + "/messages?as=anna",
		base + "/people/marco/search?q=" + url.QueryEscape(o.secret),
		base + "/agents/release-agent/events?after=0&wait=0",
		o.S.Sender + "/orgs/" + o.Name + "/notifications?person=lena",
	} {
		status, body, err := stack.Call(http.MethodGet, address, nil)
		if err != nil || status != http.StatusOK || !strings.Contains(string(body), o.secret) {
			t.Fatalf("%s cannot see its own content at %s: %d %v %.200s", o.Name, address, status, err, body)
		}
	}
}

// namingTheOther tries to act on, or read, a message that exists only in the other
// organization.
func namingTheOther(t testing.TB, a *org) {
	t.Helper()
	before := len(a.Log())
	thread := onlyBolt
	for name, try := range map[string]func() (int, []byte){
		"edit":   func() (int, []byte) { return a.Act("anna", "edit", stack.Edit(onlyBolt, "taken over")) },
		"delete": func() (int, []byte) { return a.Act("anna", "delete", stack.Delete(onlyBolt)) },
		"react": func() (int, []byte) {
			return a.Act("anna", "react", map[string]any{"message": onlyBolt, "emoji": "eyes", "on": true})
		},
		"reply": func() (int, []byte) {
			return a.Act("anna", "send", stack.Message("msg_intruder", "#launch", "into their thread", &thread))
		},
		"agent reply": func() (int, []byte) {
			st, b, _ := stack.Call(http.MethodPost, a.Address("/agents/release-agent/messages"),
				map[string]any{"id": "a-intruder", "conversation": "#launch", "thread": onlyBolt, "text": "into their thread"})
			return st, b
		},
		"agent reaction": func() (int, []byte) {
			st, b, _ := stack.Call(http.MethodPost, a.Address("/agents/release-agent/reactions"),
				map[string]any{"id": "a-intruder-2", "message": onlyBolt, "emoji": "eyes", "on": true})
			return st, b
		},
		"reply text": func() (int, []byte) {
			st, b, _ := stack.Call(http.MethodGet, a.Address("/replies/"+onlyBolt), nil)
			return st, b
		},
	} {
		status, body := try()
		if status < 400 || status >= 500 {
			t.Fatalf("%s naming the other organization's message was not refused: %d %s", name, status, body)
		}
		if strings.Contains(string(body), "bolt") {
			t.Fatalf("%s: the refusal tells something of the other organization: %s", name, body)
		}
	}
	for _, e := range a.Log()[before:] {
		if e.Kind != "tick" {
			t.Fatalf("a refused call wrote %s at number %d", e.Kind, e.Number)
		}
	}
}

type table struct{ schema, name string }

func policed(t testing.TB, s *stack.Stack) (with, without []table) {
	t.Helper()
	rows, err := s.Admin.Query(context.Background(), `
		SELECT c.table_schema, c.table_name, EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname = c.table_schema AND p.tablename = c.table_name)
		FROM information_schema.columns c JOIN information_schema.tables x USING (table_schema, table_name)
		WHERE c.column_name = 'org' AND x.table_type = 'BASE TABLE' AND c.table_schema IN ('conversations', 'feed', 'search', 'sender')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x table
		var has bool
		if err := rows.Scan(&x.schema, &x.name, &has); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(x.name, "standin_") {
			continue
		}
		if has {
			with = append(with, x)
		} else {
			without = append(without, x)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return with, without
}

// plantedQuery is a query that forgets the organization. With the row rules on it
// must return nothing of the other; with them off the same query must find it.
func plantedQuery(t testing.TB, s *stack.Stack, viewer, foreign string) {
	t.Helper()
	ctx := context.Background()
	with, without := policed(t, s)
	if len(with) == 0 {
		t.Fatal("no table has row rules")
	}
	leakedWithRulesOff := 0
	for _, x := range with {
		pool, err := pgxpool.New(ctx, s.LoginURL(x.schema))
		if err != nil {
			t.Fatal(err)
		}
		planted := fmt.Sprintf(`SELECT count(*) FROM %s.%s x WHERE x::text LIKE '%%%s%%'`, x.schema, x.name, foreign)
		var on int
		err = orgdb.InOrg(ctx, orgdb.Single{Pool: pool}, ids.Org(viewer), func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, planted).Scan(&on)
		})
		if err != nil {
			pool.Close()
			t.Fatalf("%s.%s: %v", x.schema, x.name, err)
		}
		if on != 0 {
			pool.Close()
			t.Fatalf("%s.%s: a query that leaves out the organization returned %d rows of the other with the row rules on", x.schema, x.name, on)
		}
		// Row rules off inside a transaction that is rolled back.
		tx, err := pool.Begin(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		var off int
		_, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`)
		if err == nil {
			_, err = tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.%s DISABLE ROW LEVEL SECURITY`, x.schema, x.name))
		}
		if err == nil {
			err = tx.QueryRow(ctx, planted).Scan(&off)
		}
		_ = tx.Rollback(ctx)
		pool.Close()
		if err != nil {
			t.Fatalf("%s.%s with the rules off: %v", x.schema, x.name, err)
		}
		leakedWithRulesOff += off
	}
	if leakedWithRulesOff == 0 {
		t.Fatal("with the row rules off the planted query found nothing, so this check proves nothing")
	}
	t.Logf("planted query: 0 foreign rows in %d tables with the rules on, %d with them off", len(with), leakedWithRulesOff)

	// The bookkeeping that spans organizations holds numbers and times only.
	for _, x := range without {
		var n int
		q := fmt.Sprintf(`SELECT count(*) FROM %s.%s x WHERE x::text LIKE '%%%s%%'`, x.schema, x.name, foreign)
		if err := s.Admin.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s.%s has no row rules and holds content of an organization (%d rows)", x.schema, x.name, n)
		}
	}
}

func setup(t testing.TB) (*stack.Stack, *org, *org) {
	t.Helper()
	s := stack.Start(t, stack.Options{})
	acme := &org{Org: s.Seed("acme", people, channels), secret: "acmesecret"}
	bolt := &org{Org: s.Seed("bolt", people, channels), secret: "boltsecret"}
	populate(t, acme)
	populate(t, bolt)
	if fmt.Sprint(acme.numbers) != fmt.Sprint(bolt.numbers) {
		t.Fatalf("the two organizations should have the same numbers: %v and %v", acme.numbers, bolt.numbers)
	}
	bolt.Do("anna", "send", stack.Message(onlyBolt, "#launch", "only here "+bolt.secret, nil))
	for _, o := range []*org{acme, bolt} {
		stack.Until(t, 15*time.Second, o.Name+": Lena's notification is sent", func() bool {
			for _, n := range o.NotificationsFor("lena") {
				if strings.Contains(n.Name, "msg_lena") && n.State == "sent" {
					return true
				}
			}
			return false
		})
		o.Settle(t, 30*time.Second)
	}
	return s, acme, bolt
}

func TestNothingOfOneOrganizationReachesAnother(t *testing.T) {
	s, acme, bolt := setup(t)

	t.Run("every door", func(t *testing.T) {
		ownIsVisible(t, acme)
		ownIsVisible(t, bolt)
		nothingOf(t, acme, bolt.secret)
		nothingOf(t, bolt, acme.secret)
	})

	t.Run("live connections", func(t *testing.T) {
		a, b := acme.OpenScreen("marco"), bolt.OpenScreen("marco")
		bolt.Do("anna", "send", stack.Message("msg_live", "#launch", "@Marco boltlive", nil, "marco"))
		acme.Do("anna", "send", stack.Message("msg_live", "#launch", "@Marco acmelive", nil, "marco"))
		stack.Until(t, 10*time.Second, "each screen has its own organization's message", func() bool {
			return a.Saw("acmelive") && b.Saw("boltlive")
		})
		for _, leak := range []struct {
			sc   *stack.Screen
			text string
		}{{a, "boltlive"}, {a, bolt.secret}, {b, "acmelive"}, {b, acme.secret}} {
			if leak.sc.Saw(leak.text) {
				t.Fatalf("a live connection carried %q from the other organization", leak.text)
			}
		}
	})

	t.Run("a call naming the other's message is refused", func(t *testing.T) { namingTheOther(t, acme) })

	t.Run("the planted query", func(t *testing.T) {
		plantedQuery(t, s, "acme", bolt.secret)
		plantedQuery(t, s, "bolt", acme.secret)
	})
}
