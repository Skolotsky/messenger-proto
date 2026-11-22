-- The feed's numbered log of notification notes, and the sender worker's place
-- in it (feed-to-notification-sender.md). The feed writes the log; the delivery
-- workers read it and keep their place. Changing this file is a lead-session change.

CREATE TABLE note_heads (
  org     text PRIMARY KEY,
  reset   bigint NOT NULL DEFAULT 1,
  edition bigint NOT NULL DEFAULT 1,
  head    bigint NOT NULL DEFAULT 0
);

CREATE TABLE note_log (
  org     text NOT NULL,
  edition bigint NOT NULL,
  number  bigint NOT NULL,
  at      timestamptz NOT NULL,
  kind    text NOT NULL,
  body    jsonb NOT NULL,                -- the whole Note of contracts/schema
  PRIMARY KEY (org, edition, number)
);
ALTER TABLE note_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE note_log FORCE ROW LEVEL SECURITY;
CREATE POLICY note_log_org ON note_log
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));

-- Organizations with notes to deliver: only those (G-029).
CREATE TABLE note_due (
  org   text PRIMARY KEY,
  since timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE note_places (
  org           text PRIMARY KEY,
  edition       bigint NOT NULL DEFAULT 1,
  place         bigint NOT NULL DEFAULT 0,
  reset         bigint NOT NULL DEFAULT 1,
  claimed_by    text,
  claimed_until timestamptz,
  stopped_at    bigint                   -- a refused note holds this organization's notes here
);

CREATE TABLE note_parked (
  org       text NOT NULL,
  edition   bigint NOT NULL,
  number    bigint NOT NULL,
  reason    text NOT NULL,
  tries     int NOT NULL,
  parked_at timestamptz NOT NULL,
  PRIMARY KEY (org, edition, number)
);

GRANT SELECT ON note_heads, note_log TO delivery;
GRANT SELECT, INSERT, UPDATE, DELETE ON note_due, note_places, note_parked TO delivery;
