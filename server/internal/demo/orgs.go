package demo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"contexia/internal/democontent"
	"contexia/internal/shared/call"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
)

const (
	seedBatch  = 200
	stepBudget = 60 * time.Second // how long one step of Delete or Rebuild waits out an outage
)

// refusal is a request the demo program answers with 409.
type refusal string

func (r refusal) Error() string { return "refused: " + string(r) }

func (s *Service) current(ctx context.Context) (*contract.CurrentOrganization, error) {
	var c contract.CurrentOrganization
	err := s.cfg.Pool.QueryRow(ctx, `SELECT id, name FROM organizations WHERE state = 'current'`).Scan(&c.Org, &c.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) stateOf(ctx context.Context, org ids.Org) (string, error) {
	var state string
	err := s.cfg.Pool.QueryRow(ctx, `SELECT state FROM organizations WHERE id = $1`, string(org)).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return state, err
}

func (s *Service) organizations(ctx context.Context) ([]contract.OrganizationEntry, error) {
	rows, err := s.cfg.Pool.Query(ctx, `SELECT id, name, state, created_at, approx_changes FROM organizations ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (contract.OrganizationEntry, error) {
		var e contract.OrganizationEntry
		var at time.Time
		err := r.Scan(&e.Org, &e.Name, &e.State, &at, &e.ApproxChanges)
		e.CreatedAt = at.UTC().Format(time.RFC3339)
		return e, err
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []contract.OrganizationEntry{}
	}
	return list, nil
}

// abandonUnfinished retires any organization whose seed a crash left half done:
// its log is not empty, so it can never be finished, and it was never shown.
func (s *Service) abandonUnfinished(ctx context.Context) error {
	_, err := s.cfg.Pool.Exec(ctx, `UPDATE organizations SET state = 'retired' WHERE state = 'preparing' AND through IS NULL`)
	return err
}

func (s *Service) tellRetired(ctx context.Context) {
	rows, err := s.cfg.Pool.Query(ctx, `SELECT id FROM organizations WHERE state = 'retired' AND NOT retire_told`)
	if err != nil {
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return
	}
	for _, id := range list {
		if err := s.conv.JSON(ctx, http.MethodPost, "/orgs/"+id+"/retire", nil, nil); err != nil {
			slog.Warn("demo: retire not confirmed yet", "org", id, "err", err)
			continue
		}
		_, _ = s.cfg.Pool.Exec(ctx, `UPDATE organizations SET retire_told = true WHERE id = $1`, id)
	}
}

// prepared returns an organization seeded and sealed, seeding one if there is none.
func (s *Service) prepared(ctx context.Context) (ids.Org, int64, error) {
	s.prepareMu.Lock()
	defer s.prepareMu.Unlock()
	var id string
	var through int64
	err := s.cfg.Pool.QueryRow(ctx, `SELECT id, through FROM organizations WHERE state = 'preparing' AND through IS NOT NULL
		ORDER BY created_at LIMIT 1`).Scan(&id, &through)
	if err == nil {
		return ids.Org(id), through, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", 0, err
	}
	return s.seedNew(ctx)
}

func (s *Service) seedNew(ctx context.Context) (ids.Org, int64, error) {
	var n int64
	if err := s.cfg.Pool.QueryRow(ctx, `SELECT nextval('organization_number')`).Scan(&n); err != nil {
		return "", 0, err
	}
	base := s.cfg.Content.Org
	org, err := ids.ParseOrg(fmt.Sprintf("%s-%d", base.Org, n))
	if err != nil {
		return "", 0, err
	}
	now := s.cfg.Clock.Now()
	if _, err := s.cfg.Pool.Exec(ctx, `INSERT INTO organizations (id, name, state, created_at) VALUES ($1, $2, 'preparing', $3)`,
		string(org), base.Name, now); err != nil {
		return "", 0, err
	}
	content := *s.cfg.Content
	content.Org.Org = string(org)
	options := democontent.Options{End: s.cfg.Content.LatestEnd(now)}
	if fault("vary_seed") {
		options.Seed = uint64(n) + 7
	}
	history, err := content.Build(options)
	if err != nil {
		return "", 0, err
	}
	through, err := s.seed(ctx, org, base.Name, history.Changes)
	if err != nil {
		// Half a seed can never be finished: it is not offered again.
		_, _ = s.cfg.Pool.Exec(context.WithoutCancel(ctx), `UPDATE organizations SET state = 'retired' WHERE id = $1`, string(org))
		return "", 0, fmt.Errorf("seed %s: %w", org, err)
	}
	if _, err := s.cfg.Pool.Exec(ctx, `UPDATE organizations SET through = $2, approx_changes = $2 WHERE id = $1`, string(org), through); err != nil {
		return "", 0, err
	}
	return org, through, nil
}

var expectedNumber = regexp.MustCompile(`out of order: expected number (\d+)`)

// seed sends the history through the seed session in batches. A batch whose answer
// was lost may already be saved: the session then says where it stands, and sending
// goes on from there.
func (s *Service) seed(ctx context.Context, org ids.Org, name string, changes []contract.Change) (int64, error) {
	base := "/orgs/" + string(org) + "/seed/"
	var opened contract.SeedOpened
	if err := s.retrying(ctx, stepBudget, func() error {
		return s.conv.JSON(ctx, http.MethodPost, base+"open", contract.SeedOpenRequest{Name: &name}, &opened)
	}); err != nil {
		return 0, err
	}
	for from := 0; from < len(changes); {
		to := min(from+seedBatch, len(changes))
		err := s.retrying(ctx, stepBudget, func() error {
			return s.conv.JSON(ctx, http.MethodPost, base+opened.Session, contract.SeedBatch{Changes: changes[from:to]}, nil)
		})
		var refused *call.Refused
		if errors.As(err, &refused) {
			if m := expectedNumber.FindStringSubmatch(refused.Body); m != nil {
				next, _ := strconv.Atoi(m[1])
				if next-1 > from && next-1 <= to {
					from = next - 1 // numbers run from 1 without a gap, so this is the index
					continue
				}
			}
		}
		if err != nil {
			return 0, err
		}
		from = to
	}
	var sealed struct {
		Through int64 `json:"through"`
	}
	if err := s.retrying(ctx, stepBudget, func() error {
		return s.conv.JSON(ctx, http.MethodPost, base+opened.Session+"/seal", nil, &sealed)
	}); err != nil {
		return 0, err
	}
	return sealed.Through, nil
}

// waitLevel waits until search and the feed have taken everything through the
// given number, with nothing parked.
func (s *Service) waitLevel(ctx context.Context, org ids.Org, through int64) error {
	deadline := s.cfg.Clock.Now().Add(s.cfg.LevelLimit)
	var last string
	for {
		var status contract.DeliveryStatus
		err := s.dlv.JSON(ctx, http.MethodGet, "/orgs/"+string(org)+"/status", nil, &status)
		if err == nil {
			level := 0
			last = ""
			for _, r := range status.Receivers {
				if r.Name != "search" && r.Name != "feed" {
					continue
				}
				if r.Place >= through && r.Parked == 0 {
					level++
				} else {
					last += fmt.Sprintf(" %s at %d of %d (%d parked, %s)", r.Name, r.Place, through, r.Parked, r.State)
				}
			}
			if level == 2 {
				return nil
			}
		} else {
			last = " " + err.Error()
		}
		if !s.cfg.Clock.Now().Before(deadline) {
			return fmt.Errorf("%s is not level yet:%s", org, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.cfg.Clock.After(200 * time.Millisecond):
		}
	}
}

// Reset makes a fresh organization current and retires the previous one (D-033).
func (s *Service) Reset(ctx context.Context) (*contract.CurrentOrganization, error) {
	s.resetMu.Lock()
	defer s.resetMu.Unlock()
	org, through, err := s.prepared(ctx)
	if err != nil {
		return nil, err
	}
	if !fault("skip_level") {
		if err := s.waitLevel(ctx, org, through); err != nil {
			return nil, err
		}
	}
	var previous, name string
	err = pgx.BeginFunc(ctx, s.cfg.Pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE organizations SET state = 'retired' WHERE state = 'current' RETURNING id`).Scan(&previous)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return tx.QueryRow(ctx, `UPDATE organizations SET state = 'current' WHERE id = $1 RETURNING name`, string(org)).Scan(&name)
	})
	if err != nil {
		return nil, err
	}
	running, pace := false, 1.0
	if previous != "" {
		running, pace = s.stopTeam(previous)
		// Current has moved first: a screen told "gone" must find the new one.
		// If Conversations stays out longer than this, Run keeps telling it.
		err := s.retrying(ctx, 10*time.Second, func() error {
			return s.conv.JSON(ctx, http.MethodPost, "/orgs/"+previous+"/retire", nil, nil)
		})
		if err == nil {
			_, err = s.cfg.Pool.Exec(ctx, `UPDATE organizations SET retire_told = true WHERE id = $1`, previous)
		}
		if err != nil {
			slog.Warn("demo: retire not confirmed yet", "org", previous, "err", err)
		}
	}
	if running {
		t := s.teamFor(org)
		t.command(contract.ScriptRequest{Do: "pace", Pace: &pace})
		t.command(contract.ScriptRequest{Do: "start"})
	}
	if s.cfg.PrepareAhead {
		s.prepareInBackground()
	}
	return &contract.CurrentOrganization{Org: string(org), Name: name}, nil
}

func (s *Service) prepareInBackground() {
	go func() {
		ctx := s.life
		org, through, err := s.prepared(ctx)
		if err == nil {
			err = s.waitLevel(ctx, org, through)
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("demo: next organization is not ready", "err", err)
		}
	}()
}

// Delete removes an organization from every part, in the contract's order. Each
// step may be repeated, so a delete cut short is repaired by deleting again. The
// organization's own row goes last, so an interrupted delete stays listed.
func (s *Service) Delete(ctx context.Context, org ids.Org) error {
	state, err := s.stateOf(ctx, org)
	if err != nil {
		return err
	}
	switch state {
	case "":
		return nil // already deleted: repeating changes nothing
	case "current":
		return refusal("current")
	case "preparing":
		return refusal("preparing")
	}
	s.stopTeam(string(org))
	path := "/orgs/" + string(org)
	step := func(c *call.Client, method, p string) error {
		return s.retrying(ctx, stepBudget, func() error { return c.JSON(ctx, method, p, nil, nil) })
	}
	if err := step(s.conv, http.MethodPost, path+"/retire"); err != nil {
		return fmt.Errorf("retire: %w", err)
	}
	if err := step(s.dlv, http.MethodPost, path+"/stop"); err != nil {
		return fmt.Errorf("stop delivery: %w", err)
	}
	for _, part := range []string{"search", "sender", "feed"} {
		if err := step(s.recv[part], http.MethodDelete, path); err != nil {
			return fmt.Errorf("delete from %s: %w", part, err)
		}
	}
	if err := step(s.dlv, http.MethodDelete, path); err != nil {
		return fmt.Errorf("delete from delivery: %w", err)
	}
	if err := step(s.conv, http.MethodDelete, path); err != nil {
		return fmt.Errorf("delete from conversations: %w", err)
	}
	_, err = s.cfg.Pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1 AND state = 'retired'`, string(org))
	return err
}

// DeleteRetired deletes every retired organization, going on past one that fails.
func (s *Service) DeleteRetired(ctx context.Context) ([]string, error) {
	rows, err := s.cfg.Pool.Query(ctx, `SELECT id FROM organizations WHERE state = 'retired' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	deleted := []string{}
	var failed error
	for _, id := range list {
		if err := s.Delete(ctx, ids.Org(id)); err != nil {
			failed = errors.Join(failed, fmt.Errorf("%s: %w", id, err))
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, failed
}

// Rebuild wipes one receiver of one organization, sets its place to the start and
// waits until it has been refilled.
func (s *Service) Rebuild(ctx context.Context, org ids.Org, receiver string, rulesChanged bool) error {
	path := "/orgs/" + string(org)
	client := s.recv[receiver]
	var body any
	if receiver == "feed" && rulesChanged {
		body = contract.WipeRequest{RulesChanged: &rulesChanged}
	}
	wipe := path + "/control/wipe"
	if fault("wipe_all") {
		wipe = "/control/wipe"
	}
	if err := s.retrying(ctx, stepBudget, func() error {
		return client.JSON(ctx, http.MethodPost, wipe, body, nil)
	}); err != nil {
		return fmt.Errorf("wipe %s: %w", receiver, err)
	}
	if fault("no_restart") {
		return nil
	}
	restart := contract.RestartRequest{}
	if receiver == "feed" {
		var place struct {
			Edition int64 `json:"edition"`
		}
		if err := s.retrying(ctx, stepBudget, func() error {
			return client.JSON(ctx, http.MethodGet, path+"/place", nil, &place)
		}); err != nil {
			return fmt.Errorf("read the feed's edition: %w", err)
		}
		restart.Edition = &place.Edition
	}
	if err := s.retrying(ctx, stepBudget, func() error {
		return s.dlv.JSON(ctx, http.MethodPost, path+"/receivers/"+receiver+"/restart", restart, nil)
	}); err != nil {
		return fmt.Errorf("restart %s: %w", receiver, err)
	}
	return s.waitRefilled(ctx, org, receiver)
}

// waitRefilled waits until the receiver's place has reached the head it had to
// reach when asked: new changes may keep arriving behind it.
func (s *Service) waitRefilled(ctx context.Context, org ids.Org, receiver string) error {
	deadline := s.cfg.Clock.Now().Add(s.cfg.LevelLimit)
	target := int64(-1)
	for {
		var status contract.DeliveryStatus
		err := s.dlv.JSON(ctx, http.MethodGet, "/orgs/"+string(org)+"/status", nil, &status)
		if err == nil {
			for _, r := range status.Receivers {
				if string(r.Name) != receiver {
					continue
				}
				if target < 0 {
					target = r.Place + r.Waiting
				}
				if r.Place >= target {
					return nil
				}
			}
		}
		if !s.cfg.Clock.Now().Before(deadline) {
			return fmt.Errorf("%s of %s is still refilling", receiver, org)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.cfg.Clock.After(200 * time.Millisecond):
		}
	}
}
