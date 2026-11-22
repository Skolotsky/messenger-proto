-- The feed service (T-052). Everything here can be wiped and worked out again from
-- the log. feed_journal is the feed's own copy of what it took, in order, with what
-- became of each change: a restarted or second copy replays it to get the rules'
-- state back. The other tables are what the door reads, saved in the same
-- transaction as the place.

-- One row per organization: numbers only (G-025).
CREATE TABLE feed_state (
  org             text PRIMARY KEY,
  edition         bigint NOT NULL,
  through         bigint NOT NULL DEFAULT 0,
  row_head        bigint NOT NULL DEFAULT 0,
  history_through bigint NOT NULL DEFAULT 0,   -- a rebuild after a rule change: sends up to here are history (D-034)
  stopped         boolean NOT NULL DEFAULT false
);

CREATE TABLE feed_journal (
  org         text NOT NULL,
  number      bigint NOT NULL,
  disposition text NOT NULL,                   -- applied, counted, set_aside, fenced
  fence_conversation text NOT NULL DEFAULT '',
  fence_person       text NOT NULL DEFAULT '',
  body        text NOT NULL,                   -- text, not jsonb: a change jsonb cannot hold must still be set aside
  PRIMARY KEY (org, number)
);

CREATE TABLE feed_rows (
  org     text NOT NULL,
  person  text NOT NULL,
  row_key text NOT NULL,
  section int NOT NULL,
  last    bigint NOT NULL,
  body    text NOT NULL,
  PRIMARY KEY (org, person, row_key)
);
CREATE INDEX feed_rows_order ON feed_rows (org, person, section, last DESC, row_key);

CREATE TABLE feed_readers (
  org    text NOT NULL,
  person text NOT NULL,
  number bigint NOT NULL DEFAULT 0,           -- the last row update of this reader
  counts text NOT NULL,
  PRIMARY KEY (org, person)
);

-- Notifications on hold, kept with their rows (D-034).
CREATE TABLE feed_held (
  org          text NOT NULL,
  name         text NOT NULL,
  person       text NOT NULL,
  conversation text NOT NULL,
  message      text NOT NULL,
  item         text NOT NULL,
  at           text NOT NULL,
  PRIMARY KEY (org, name)
);

CREATE TABLE feed_row_updates (
  org    text NOT NULL,
  number bigint NOT NULL,
  person text NOT NULL,
  after  bigint NOT NULL,
  parts  text NOT NULL,
  PRIMARY KEY (org, number)
);

CREATE TABLE feed_fences (
  org          text NOT NULL,
  conversation text NOT NULL,
  person       text NOT NULL DEFAULT '',      -- empty: the whole conversation
  number       bigint NOT NULL,
  reason       text NOT NULL,
  PRIMARY KEY (org, conversation, person)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['feed_journal','feed_rows','feed_readers','feed_held','feed_row_updates','feed_fences'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY %I ON %I USING (org = current_setting('app.org', true)) WITH CHECK (org = current_setting('app.org', true))$p$, t || '_org', t);
  END LOOP;
END $$;
