-- The notification sender (feed-to-notification-sender.md).

-- One row per organization: numbers and times only (G-025).
CREATE TABLE state (
  org     text PRIMARY KEY,
  edition bigint NOT NULL DEFAULT 1,
  place   bigint NOT NULL DEFAULT 0,
  clock   timestamptz                 -- the newest clock note; moved by nothing else
);

CREATE TABLE notifications (
  org        text NOT NULL,
  name       text NOT NULL,
  person     text NOT NULL,
  state      text NOT NULL CHECK (state IN ('held', 'sent', 'withdrawn', 'history')),
  due_at     timestamptz NOT NULL,
  arrived_at timestamptz NOT NULL,
  sent_at    timestamptz,
  delayed    boolean NOT NULL DEFAULT false,
  why        text NOT NULL,
  body       text NOT NULL,
  now        text CHECK (now IN ('read', 'deleted')),
  PRIMARY KEY (org, name)
);
CREATE INDEX notifications_person ON notifications (org, person, due_at DESC);
CREATE INDEX notifications_held ON notifications (org, due_at) WHERE state = 'held';
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;
CREATE POLICY notifications_org ON notifications
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));

-- What the fake provider was handed, one row per hand-over. Never wiped: this
-- is how a repeat would show.
CREATE TABLE deliveries (
  org  text NOT NULL,
  name text NOT NULL,
  at   timestamptz NOT NULL
);
CREATE INDEX deliveries_org ON deliveries (org, name);
ALTER TABLE deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY deliveries_org ON deliveries
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));
