-- The sender only delivers (D-034): the hold, its clock and the withdrawn state
-- have moved to the feed. An update for a name not sent yet waits in a 'waiting'
-- row, which is never listed and becomes the entry when its send arrives.

DELETE FROM notifications WHERE state IN ('held', 'withdrawn');
ALTER TABLE notifications DROP CONSTRAINT notifications_state_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_state_check CHECK (state IN ('sent', 'history', 'waiting'));
DROP INDEX notifications_held;
ALTER TABLE notifications ADD COLUMN text_number bigint NOT NULL DEFAULT 0; -- the note that last set the text
ALTER TABLE state DROP COLUMN clock;
