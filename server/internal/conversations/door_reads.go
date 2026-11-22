package conversations

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"contexia/internal/shared/call"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// The door's words when a part is away or behind (door-reads.md).
const (
	noticeSearchDown  = "Search is unavailable right now. Your messages are safe."
	noticeCatchingUp  = "Catching up"
	notAvailableFeed  = "the feed is catching up"
	behindAfter       = 10 * time.Second
	searchKindsClause = `AND kind IN ('message.created', 'message.changed', 'message.deleted')`
)

// behind is how long the oldest change this receiver is sent has been waiting
// past its delivery place. A quiet channel never looks late: changes the receiver
// is not sent do not count, and a receiver with nothing waiting is not behind.
func (s *Store) behind(ctx context.Context, org ids.Org, receiver string) (time.Duration, error) {
	var waiting *time.Time
	clause := ""
	if receiver == "search" {
		clause = searchKindsClause
	}
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		var place int64
		err := tx.QueryRow(ctx, `SELECT place FROM delivery_places WHERE org = $1 AND receiver = $2`, string(org), receiver).Scan(&place)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var at time.Time
		err = tx.QueryRow(ctx, `SELECT at FROM change_log WHERE org = $1 AND number > $2 `+clause+` ORDER BY number LIMIT 1`,
			string(org), place).Scan(&at)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		waiting = &at
		return nil
	})
	if err != nil || waiting == nil {
		return 0, err
	}
	return max(s.Clock.Now().Sub(*waiting), 0), nil
}

func missingNotice(lag time.Duration) string {
	minutes := int(math.Ceil(lag.Minutes()))
	if minutes <= 1 {
		return "the last minute may be missing"
	}
	return fmt.Sprintf("the last %d minutes may be missing", minutes)
}

// person answers 404 for someone the organization does not have. The door checks
// it itself: a 404 from a receiver may only mean it has not heard of them yet,
// and a screen must not take that for a deleted organization.
func (l *Live) person(w http.ResponseWriter, r *http.Request) (ids.Org, string, bool) {
	org, ok := orgOf(w, r)
	if !ok {
		return "", "", false
	}
	person := r.PathValue("person")
	known, err := l.s.personKnown(r.Context(), org, person)
	if err != nil {
		fail(w, r, err)
		return "", "", false
	}
	if !known {
		send(w, http.StatusNotFound, contract.Refusal{Refused: refusedUnknownPerson})
		return "", "", false
	}
	return org, person, true
}

func personPath(org ids.Org, person, rest string) string {
	return "/orgs/" + string(org) + "/people/" + url.PathEscape(person) + "/" + rest
}

func (l *Live) handleFeed(w http.ResponseWriter, r *http.Request) {
	org, person, ok := l.person(w, r)
	if !ok {
		return
	}
	notice := noticeCatchingUp
	unavailable := contract.ScreenFeedAnswer{Rows: []contract.FeedRow{}, Notice: &notice}
	if l.feed == nil {
		send(w, http.StatusOK, unavailable)
		return
	}
	var rows contract.FeedRowsAnswer
	if err := l.feed.JSON(r.Context(), "GET", personPath(org, person, "rows"), nil, &rows); err != nil {
		send(w, http.StatusOK, unavailable)
		return
	}
	if rows.Rows == nil {
		rows.Rows = []contract.FeedRow{}
	}
	answer := contract.ScreenFeedAnswer{Edition: &rows.Edition, Through: &rows.Through, Number: &rows.Number, Rows: rows.Rows, Counts: &rows.Counts}
	if lag, err := l.s.behind(r.Context(), org, "feed"); err == nil && lag > behindAfter {
		answer.Notice = &notice
	}
	send(w, http.StatusOK, answer)
}

// passThrough answers with what the feed said, or "not available right now".
func (l *Live) passThrough(w http.ResponseWriter, r *http.Request, path string, into any) {
	if l.feed == nil {
		send(w, http.StatusServiceUnavailable, contract.NotAvailable{NotAvailable: notAvailableFeed})
		return
	}
	if err := l.feed.JSON(r.Context(), "GET", path, nil, into); err != nil {
		var refused *call.Refused
		if errors.As(err, &refused) && refused.Status == http.StatusBadRequest {
			send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad question"})
			return
		}
		send(w, http.StatusServiceUnavailable, contract.NotAvailable{NotAvailable: notAvailableFeed})
		return
	}
	send(w, http.StatusOK, into)
}

func (l *Live) handleHistory(w http.ResponseWriter, r *http.Request) {
	org, person, ok := l.person(w, r)
	if !ok {
		return
	}
	q := url.Values{"tab": {r.URL.Query().Get("tab")}}
	if after := r.URL.Query().Get("after"); after != "" {
		q.Set("after", after)
	}
	var answer contract.HistoryAnswer
	l.passThrough(w, r, personPath(org, person, "history?"+q.Encode()), &answer)
}

func (l *Live) handleSettings(w http.ResponseWriter, r *http.Request) {
	org, person, ok := l.person(w, r)
	if !ok {
		return
	}
	var answer contract.SettingsAnswer
	l.passThrough(w, r, personPath(org, person, "settings"), &answer)
}

// handlePreview says "not available" while the feed is behind: what it would
// count is not current (screen-to-conversations.md).
func (l *Live) handlePreview(w http.ResponseWriter, r *http.Request) {
	org, person, ok := l.person(w, r)
	if !ok {
		return
	}
	if lag, err := l.s.behind(r.Context(), org, "feed"); err == nil && lag > behindAfter {
		send(w, http.StatusServiceUnavailable, contract.NotAvailable{NotAvailable: notAvailableFeed})
		return
	}
	q := url.Values{"change": {r.URL.Query().Get("change")}}
	var answer contract.PreviewAnswer
	l.passThrough(w, r, personPath(org, person, "preview?"+q.Encode()), &answer)
}

func (l *Live) handleSearch(w http.ResponseWriter, r *http.Request) {
	org, person, ok := l.person(w, r)
	if !ok {
		return
	}
	notice := noticeSearchDown
	if l.search == nil {
		send(w, http.StatusOK, contract.ScreenSearchAnswer{Notice: &notice})
		return
	}
	q := url.Values{"q": {r.URL.Query().Get("q")}, "as": {person}}
	if after := r.URL.Query().Get("after"); after != "" {
		q.Set("after", after)
	}
	var found contract.SearchAnswer
	if err := l.search.JSON(r.Context(), "GET", "/orgs/"+string(org)+"/query?"+q.Encode(), nil, &found); err != nil {
		send(w, http.StatusOK, contract.ScreenSearchAnswer{Notice: &notice})
		return
	}
	if found.Results == nil {
		found.Results = []contract.SearchResult{}
	}
	answer := contract.ScreenSearchAnswer{Answer: &found}
	if lag, err := l.s.behind(r.Context(), org, "search"); err == nil && lag > behindAfter {
		behind := missingNotice(lag)
		answer.Notice = &behind
	}
	send(w, http.StatusOK, answer)
}
