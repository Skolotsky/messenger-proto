package conversations

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
)

// errLeaseLost says this copy was declared dead: it can save no watching under
// this identity.
var errLeaseLost = errors.New("this copy's lease is gone")

// watchEnd is the end of one row's watching, said by the screen.
type watchEnd struct {
	org        ids.Org
	connection string
	person     string
	key        watchKey
}

// connectionEnd is everything one connection had open.
type connectionEnd struct {
	org        ids.Org
	connection string
}

func (k watchKey) threadPtr() *string {
	if !k.hasThread {
		return nil
	}
	t := k.thread
	return &t
}

// watch handles "this row is open in front of me", and "no longer".
func (l *Live) watch(ctx context.Context, c *conn, note contract.WatchingNote) error {
	key := keyOf(note.Conversation, note.Thread)
	if !note.Open {
		c.mu.Lock()
		_, had := c.watching[key]
		delete(c.watching, key)
		c.mu.Unlock()
		if !had {
			return nil
		}
		return l.saveEnd(ctx, watchEnd{org: c.org, connection: c.id, person: c.person, key: key})
	}
	c.mu.Lock()
	_, had := c.watching[key]
	c.mu.Unlock()
	if had {
		return nil
	}
	started, err := l.saveStart(ctx, c, note)
	if errors.Is(err, errLeaseLost) {
		l.leaseLost()
		return nil
	}
	if err != nil || !started {
		return err
	}
	c.mu.Lock()
	c.watching[key] = struct{}{}
	c.mu.Unlock()
	return nil
}

// saveStart saves a start in a step that also confirms this copy's lease. The
// lease row is held for the step, so declaring the copy dead waits for it and the
// sweep then sees this watching.
func (l *Live) saveStart(ctx context.Context, c *conn, note contract.WatchingNote) (bool, error) {
	copyID := l.Copy()
	started := false
	err := l.s.Save(ctx, c.org, func(st *Step) error {
		if watchingChecksLease {
			var until time.Time
			var dead bool
			err := st.Tx.QueryRow(ctx, `SELECT valid_until, dead FROM copy_leases WHERE copy = $1 FOR SHARE`, copyID).Scan(&until, &dead)
			if errors.Is(err, pgx.ErrNoRows) {
				return errLeaseLost
			}
			if err != nil {
				return err
			}
			if dead || !until.After(l.s.Clock.Now()) {
				return errLeaseLost
			}
		}
		why, err := st.checkRow(note.Conversation, note.Thread)
		if err != nil || why != "" {
			return err // a row that does not exist is not watched
		}
		kind, err := st.conversationType(note.Conversation)
		if err != nil {
			return err
		}
		if kind == "direct" {
			people, err := st.participants(note.Conversation)
			if err != nil {
				return err
			}
			if !slices.Contains(people, c.person) {
				return nil
			}
		}
		tag, err := st.Tx.Exec(ctx, `INSERT INTO watching (org, connection, person, conversation, thread, copy)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
			string(c.org), c.id, c.person, note.Conversation, note.Thread, copyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if _, err := st.Tx.Exec(ctx, `UPDATE org_log SET watch_copies = array_append(watch_copies, $1) WHERE org = $2 AND NOT watch_copies @> ARRAY[$1]::text[]`, copyID, string(c.org)); err != nil {
			return err
		}
		_, err = st.Append(contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Person: c.person,
			Conversation: note.Conversation, Thread: note.Thread, Connection: c.id})
		started = err == nil
		return err
	})
	return started, err
}

// saveEnd writes the end of one watching, if it is still open.
func (l *Live) saveEnd(ctx context.Context, e watchEnd) error {
	return l.s.Save(ctx, e.org, func(st *Step) error {
		thread := e.key.threadPtr()
		tag, err := st.Tx.Exec(ctx, `DELETE FROM watching WHERE org = $1 AND connection = $2 AND conversation = $3 AND thread IS NOT DISTINCT FROM $4`,
			string(e.org), e.connection, e.key.conversation, thread)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = st.Append(contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Person: e.person,
			Conversation: e.key.conversation, Thread: thread, Connection: e.connection})
		return err
	})
}

// endAllWatching runs when a connection is gone. It ends what the database says
// this connection has open, so a start whose answer was lost with the connection
// is ended too. What cannot be saved now is tried again by the 5-second check.
func (c *conn) endAllWatching() {
	if !connectionEndsWatching {
		return // test-only fault
	}
	c.mu.Lock()
	c.watching = map[watchKey]struct{}{}
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), writeWithin)
	defer cancel()
	e := connectionEnd{org: c.org, connection: c.id}
	if err := c.l.endConnection(ctx, e); err != nil {
		slog.Warn("ending a connection's watchings, will retry", "org", c.org, "err", err)
		c.l.mu.Lock()
		c.l.ends = append(c.l.ends, e)
		c.l.mu.Unlock()
	}
}

// endConnection writes the end of everything one connection has open. Nothing of
// a retired or deleted organization needs ending.
func (l *Live) endConnection(ctx context.Context, e connectionEnd) error {
	err := l.s.Save(ctx, e.org, func(st *Step) error {
		rows, err := st.Tx.Query(ctx, `DELETE FROM watching WHERE org = $1 AND connection = $2 RETURNING person, conversation, thread`,
			string(e.org), e.connection)
		if err != nil {
			return err
		}
		type open struct {
			person, conversation string
			thread               *string
		}
		var list []open
		for rows.Next() {
			var o open
			if err := rows.Scan(&o.person, &o.conversation, &o.thread); err != nil {
				rows.Close()
				return err
			}
			list = append(list, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, o := range list {
			if _, err := st.Append(contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Person: o.person,
				Conversation: o.conversation, Thread: o.thread, Connection: e.connection}); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, ErrOrganizationGone) || errors.Is(err, ErrUnknownOrg) {
		return nil
	}
	return err
}

func (l *Live) retryEnds(ctx context.Context) {
	l.mu.Lock()
	pending := l.ends
	l.ends = nil
	l.mu.Unlock()
	for _, e := range pending {
		if err := l.endConnection(ctx, e); err != nil {
			l.mu.Lock()
			l.ends = append(l.ends, e)
			l.mu.Unlock()
		}
	}
}

// leaseLost: this copy was declared dead while it was frozen. Its watchings are
// already ended. It takes a fresh identity and drops its connections, so each
// screen reconnects and says again what it has open.
func (l *Live) leaseLost() {
	l.mu.Lock()
	l.identity = "copy_" + randomID()
	l.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), writeWithin)
	defer cancel()
	if err := l.RegisterLease(ctx); err != nil {
		slog.Error("new lease", "err", err)
	}
	for _, h := range l.allHubs() {
		for _, c := range h.connections() {
			c.mu.Lock()
			c.watching = map[watchKey]struct{}{}
			c.mu.Unlock()
			c.cancel()
			_ = c.ws.CloseNow()
		}
	}
}
