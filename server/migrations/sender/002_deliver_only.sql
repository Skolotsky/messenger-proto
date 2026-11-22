-- The sender only delivers (D-034): the hold, its clock and the withdrawn state
-- have moved to the feed. An update for a name not on the list yet waits here.

DELETE FROM notifications WHERE state IN ('held', 'withdrawn');
ALTER TABLE notifications DROP CONSTRAINT notifications_state_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_state_check CHECK (state IN ('sent', 'history'));
DROP INDEX notifications_held;
ALTER TABLE notifications DROP COLUMN arrived_at;
ALTER TABLE notifications ADD COLUMN text_number bigint NOT NULL DEFAULT 0; -- the note that last set the text
ALTER TABLE state DROP COLUMN clock;

CREATE TABLE updates_waiting (
  org         text NOT NULL,
  name        text NOT NULL,
  text        text,
  text_number bigint NOT NULL DEFAULT 0,
  now         text CHECK (now IN ('read', 'deleted')),
  PRIMARY KEY (org, name)
);
ALTER TABLE updates_waiting ENABLE ROW LEVEL SECURITY;
ALTER TABLE updates_waiting FORCE ROW LEVEL SECURITY;
CREATE POLICY updates_waiting_org ON updates_waiting
  USING (org = current_setting('app.org', true))
  WITH CHECK (org = current_setting('app.org', true));
