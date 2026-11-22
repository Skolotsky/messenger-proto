package conversations

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strconv"

	"contexia/internal/shared/contract"
)

const (
	feedPage      = 200
	maxFeedReplay = 20 // pages one replay may walk before "load fresh"
)

// updatesPage is the feed's answer to "row changes after N": a page, or the news
// that the feed was rebuilt.
type updatesPage struct {
	Updates    []contract.RowUpdate `json:"updates"`
	Next       *int64               `json:"next"`
	NewEdition *int64               `json:"new_edition"`
}

func (h *orgHub) askUpdates(ctx context.Context, after, edition int64) (updatesPage, error) {
	var page updatesPage
	q := url.Values{"after": {strconv.FormatInt(after, 10)}, "edition": {strconv.FormatInt(edition, 10)}, "limit": {strconv.Itoa(feedPage)}}
	err := h.l.feed.JSON(ctx, "GET", "/orgs/"+string(h.org)+"/updates?"+q.Encode(), nil, &page)
	return page, err
}

func (h *orgHub) runFeed() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.feedWake:
		}
		if h.l.feed == nil {
			continue
		}
		if err := h.passFeed(h.ctx); err != nil && h.ctx.Err() == nil {
			slog.Debug("reading the feed for screens", "org", h.org, "err", err)
		}
	}
}

// passFeed asks by organization, never per person: it reads the row log once
// from the furthest place any screen has and hands each update to its reader's
// screens. A feed that is down costs a retry at the next wake and nothing else.
func (h *orgHub) passFeed(ctx context.Context) error {
	if err := h.tailFeed(ctx); err != nil {
		return err
	}
	for _, c := range h.connections() {
		c.mu.Lock()
		n := c.resumeFeed
		c.resumeFeed = nil
		c.mu.Unlock()
		if n == nil {
			continue
		}
		if err := h.replayFeed(ctx, c, *n); err != nil {
			c.mu.Lock()
			if c.resumeFeed == nil {
				c.resumeFeed = n
			}
			c.mu.Unlock()
			return err
		}
	}
	return h.tailFeed(ctx)
}

func (h *orgHub) tailFeed(ctx context.Context) error {
	if h.feedPos == nil || h.feedEdition == nil {
		return nil
	}
	for {
		page, err := h.askUpdates(ctx, *h.feedPos, *h.feedEdition)
		if err != nil {
			return err
		}
		if page.NewEdition != nil {
			h.newEdition(*page.NewEdition)
			return nil
		}
		for _, u := range page.Updates {
			h.sendFeedUpdate(u)
			at := u.Number
			h.feedPos = &at
		}
		if page.Next == nil || len(page.Updates) == 0 || *page.Next < *h.feedPos {
			return nil
		}
		next := *page.Next
		h.feedPos = &next
	}
}

// newEdition: the feed was rebuilt, so every screen loads the feed view fresh and
// says its new numbers.
func (h *orgHub) newEdition(edition int64) {
	h.feedEdition = &edition
	h.feedPos = nil
	b, _ := json.Marshal(contract.LiveTooFarBehind{TooFarBehind: "feed"})
	for _, c := range h.connections() {
		c.mu.Lock()
		following := c.feedAt != nil
		c.feedAt, c.edition = nil, nil
		c.mu.Unlock()
		if following {
			c.enqueue(outMessage{body: b})
		}
	}
}

func (h *orgHub) sendFeedUpdate(u contract.RowUpdate) {
	for _, c := range h.connections() {
		if c.person != u.Person {
			continue
		}
		c.mu.Lock()
		if c.feedAt != nil && c.edition != nil && *c.edition == *h.feedEdition && u.Number > *c.feedAt {
			parts := u.Parts
			if parts == nil {
				parts = []contract.FeedPart{}
			}
			b, err := json.Marshal(contract.LiveFeedUpdate{Stream: "feed", Number: u.Number, After: u.After, Edition: *c.edition, Parts: parts})
			if err == nil {
				n := u.Number
				c.feedAt = &n
				c.enqueue(outMessage{body: b})
			}
		}
		c.mu.Unlock()
	}
}

// replayFeed brings one screen from its number up to the reader's place.
func (h *orgHub) replayFeed(ctx context.Context, c *conn, n contract.ResumeNumbers) error {
	if n.Feed == nil || n.FeedEdition == nil {
		c.mu.Lock()
		c.feedAt, c.edition = nil, nil
		c.mu.Unlock()
		return nil
	}
	if h.feedEdition != nil && *h.feedEdition != *n.FeedEdition {
		// One of the two is out of date; the feed says which.
		page, err := h.askUpdates(ctx, *n.Feed, *n.FeedEdition)
		if err != nil {
			return err
		}
		if page.NewEdition != nil { // the screen's: it loads the view fresh
			h.tellFeedTooFar(c)
			if *page.NewEdition != *h.feedEdition {
				h.newEdition(*page.NewEdition)
			}
			return nil
		}
		// The reader's: the screen already loaded the rebuilt feed, the others have not.
		c.mu.Lock()
		c.feedAt, c.edition = nil, nil
		c.mu.Unlock()
		h.newEdition(*n.FeedEdition)
	}
	if h.feedEdition == nil {
		edition := *n.FeedEdition
		h.feedEdition = &edition
	}
	if h.feedPos == nil {
		at := *n.Feed
		h.feedPos = &at
	}
	target := *h.feedPos
	collected := []contract.RowUpdate{}
	after := *n.Feed
	for pages := 0; after < target; pages++ {
		if pages >= maxFeedReplay {
			h.tellFeedTooFar(c)
			return nil
		}
		page, err := h.askUpdates(ctx, after, *n.FeedEdition)
		if err != nil {
			return err
		}
		if page.NewEdition != nil {
			h.tellFeedTooFar(c)
			return nil
		}
		for _, u := range page.Updates {
			if u.Number > target {
				break
			}
			if u.Person == c.person {
				collected = append(collected, u)
			}
		}
		if len(collected) > replayLimit {
			h.tellFeedTooFar(c)
			return nil
		}
		if page.Next == nil || len(page.Updates) == 0 || *page.Next <= after {
			break
		}
		after = *page.Next
	}
	c.mu.Lock()
	at, edition := *n.Feed, *n.FeedEdition
	c.feedAt, c.edition = &at, &edition
	c.mu.Unlock()
	for _, u := range collected {
		h.sendFeedUpdate(u)
	}
	return nil
}

func (h *orgHub) tellFeedTooFar(c *conn) {
	c.mu.Lock()
	c.feedAt, c.edition = nil, nil
	c.mu.Unlock()
	b, _ := json.Marshal(contract.LiveTooFarBehind{TooFarBehind: "feed"})
	c.enqueue(outMessage{body: b})
}
