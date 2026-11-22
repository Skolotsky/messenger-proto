-- The search index (conversations-to-search.md). A deleted message stays as a
-- row with no text, so a late edit cannot bring it back.

-- One row per organization: numbers only (G-025).
CREATE TABLE state (
  org         text PRIMARY KEY,
  reset       bigint NOT NULL DEFAULT 1,
  through     bigint NOT NULL DEFAULT 0,
  wipe        bigint NOT NULL DEFAULT 0,
  refill_high bigint                       -- while refilling after a wipe: the place it had
);

CREATE TABLE messages (
  org          text NOT NULL,
  id           text NOT NULL,
  number       bigint NOT NULL,
  version      bigint NOT NULL,
  conversation text NOT NULL DEFAULT '',
  thread       text,
  author       text NOT NULL DEFAULT '',
  edited       boolean NOT NULL DEFAULT false,
  participants text[],
  created_at   text NOT NULL DEFAULT '',
  body         text NOT NULL DEFAULT '',
  folded       text NOT NULL DEFAULT '',     -- body in lower case, same code points
  deleted      boolean NOT NULL DEFAULT false,
  PRIMARY KEY (org, id)
);
CREATE INDEX messages_newest ON messages (org, number DESC) WHERE NOT deleted;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
CREATE POLICY messages_org ON messages
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));
