-- The change log and delivery bookkeeping: the one store shared by two parts on
-- purpose (G-008). Conversations writes the log; the delivery workers read it
-- and keep their own places. Changing this file is a lead-session change.

-- One row per organization. Numbers and times only, never content (G-025).
-- A save takes this row's lock to get its number, so an organization's numbers
-- commit in order and a reader never steps over one (G-005).
CREATE TABLE org_log (
  org            text PRIMARY KEY,
  head           bigint NOT NULL DEFAULT 0,
  head_at        timestamptz,
  last_change_at timestamptz,          -- newest entry that is not a tick
  tick_due_at    timestamptz           -- null when no tick is due
);
CREATE INDEX org_log_tick_due ON org_log (tick_due_at) WHERE tick_due_at IS NOT NULL;

CREATE TABLE change_log (
  org    text NOT NULL REFERENCES org_log (org),
  number bigint NOT NULL,
  at     timestamptz NOT NULL,
  kind   text NOT NULL,
  body   jsonb NOT NULL,               -- the whole Change of contracts/schema
  PRIMARY KEY (org, number)
);
ALTER TABLE change_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE change_log FORCE ROW LEVEL SECURITY;
CREATE POLICY change_log_org ON change_log
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));

-- Which receivers have work: holds only organizations that have some (G-029).
-- The save that writes a change adds a row per receiver in the same transaction;
-- a worker removes it, in a transaction that rechecks the head, once level.
CREATE TABLE delivery_due (
  receiver text NOT NULL CHECK (receiver IN ('search', 'feed')),
  org      text NOT NULL REFERENCES org_log (org),
  since    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (receiver, org)
);

-- A receiver's place in an organization's log.
CREATE TABLE delivery_places (
  org           text NOT NULL REFERENCES org_log (org),
  receiver      text NOT NULL CHECK (receiver IN ('search', 'feed')),
  place         bigint NOT NULL DEFAULT 0,
  wipe          bigint NOT NULL DEFAULT 0,
  claimed_by    text,
  claimed_until timestamptz,
  refusals      int NOT NULL DEFAULT 0,  -- different changes refused in a row
  paused        boolean NOT NULL DEFAULT false,
  PRIMARY KEY (org, receiver)
);

CREATE TABLE delivery_parked (
  org       text NOT NULL REFERENCES org_log (org),
  receiver  text NOT NULL,
  number    bigint NOT NULL,
  reason    text NOT NULL,
  tries     int NOT NULL,
  parked_at timestamptz NOT NULL,
  PRIMARY KEY (org, receiver, number)
);

-- Kept once per receiver and shared by all workers (conversations-to-search.md).
CREATE TABLE delivery_receivers (
  receiver      text PRIMARY KEY,
  down_since    timestamptz,
  retry_at      timestamptz,
  failures      int NOT NULL DEFAULT 0
);

GRANT SELECT ON org_log, change_log TO delivery;
GRANT SELECT, INSERT, UPDATE, DELETE ON delivery_due, delivery_places, delivery_parked, delivery_receivers TO delivery;
