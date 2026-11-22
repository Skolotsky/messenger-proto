-- What the live connection and housekeeping keep (T-053). The lease and lock tables
-- span organizations by nature and hold identities and times only, never content (G-025).

-- A copy of Conversations holds a lease: renewed every 5 s, valid for 15. A start
-- of "watching" is saved only in a step that confirms it (G-024).
CREATE TABLE copy_leases (
  copy        text PRIMARY KEY,
  valid_until timestamptz NOT NULL,
  dead        boolean NOT NULL DEFAULT false
);
CREATE INDEX copy_leases_expiry ON copy_leases (valid_until) WHERE NOT dead;
CREATE INDEX copy_leases_dead ON copy_leases (copy) WHERE dead;

-- The copies that started a watching in this organization, on the organization's
-- own row, so it goes when the organization is deleted. The index holds only
-- organizations with some, so a dead copy's are found without reading the idle
-- ones (G-029).
ALTER TABLE org_log ADD COLUMN watch_copies text[] NOT NULL DEFAULT '{}';
CREATE INDEX org_log_watch_copies ON org_log USING gin (watch_copies) WHERE watch_copies <> '{}';

-- One copy runs housekeeping at a time; the lock passes on when its time runs out.
CREATE TABLE housekeeping_lock (
  id     int PRIMARY KEY CHECK (id = 1),
  holder text,
  until  timestamptz
);
INSERT INTO housekeeping_lock (id) VALUES (1);

ALTER TABLE watching ADD COLUMN copy text;
