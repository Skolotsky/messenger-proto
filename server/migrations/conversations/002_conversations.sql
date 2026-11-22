-- What Conversations owns: people, channels, messages, reactions and what
-- readers did (D-017). Every table is keyed by organization and held by row
-- rules (G-025). Every table that holds a fact can only change in a transaction
-- that also wrote a log entry (G-004).

CREATE TABLE people (
  org      text NOT NULL,
  id       text NOT NULL,
  name     text NOT NULL,
  is_agent boolean NOT NULL,
  line     text,
  PRIMARY KEY (org, id)
);

CREATE TABLE conversations (
  org  text NOT NULL,
  id   text NOT NULL,            -- "#launch" or "dm:anna:marco"
  type text NOT NULL CHECK (type IN ('channel', 'direct')),
  name text NOT NULL,
  PRIMARY KEY (org, id)
);

CREATE TABLE conversation_people (
  org          text NOT NULL,
  conversation text NOT NULL,
  person       text NOT NULL,
  PRIMARY KEY (org, conversation, person),
  FOREIGN KEY (org, conversation) REFERENCES conversations (org, id),
  FOREIGN KEY (org, person) REFERENCES people (org, id)
);
CREATE INDEX conversation_people_person ON conversation_people (org, person);

CREATE TABLE messages (
  org             text NOT NULL,
  id              text NOT NULL,
  number          bigint NOT NULL,   -- the log number of its message.created
  version         bigint NOT NULL,
  conversation    text NOT NULL,
  thread          text,
  author          text NOT NULL,
  author_is_agent boolean NOT NULL,
  participants    text[],
  text            text NOT NULL,
  mentions        text[] NOT NULL,
  level           text CHECK (level IN ('flag', 'info')),
  stopped         boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL,
  edited_at       timestamptz,
  deleted         boolean NOT NULL DEFAULT false,
  PRIMARY KEY (org, id),
  UNIQUE (org, number),
  FOREIGN KEY (org, conversation) REFERENCES conversations (org, id),
  FOREIGN KEY (org, author) REFERENCES people (org, id),
  FOREIGN KEY (org, thread) REFERENCES messages (org, id)
);
CREATE INDEX messages_top ON messages (org, conversation, number) WHERE thread IS NULL;
CREATE INDEX messages_thread ON messages (org, thread, number) WHERE thread IS NOT NULL;

CREATE TABLE reactions (
  org     text NOT NULL,
  message text NOT NULL,
  emoji   text NOT NULL,
  person  text NOT NULL,
  number  bigint NOT NULL,          -- the change that added it: keeps the order people reacted in
  PRIMARY KEY (org, message, emoji, person),
  FOREIGN KEY (org, message) REFERENCES messages (org, id),
  FOREIGN KEY (org, person) REFERENCES people (org, id)
);

-- What readers did. A row is a conversation plus an optional thread.
CREATE TABLE reader_reads (
  org          text NOT NULL,
  person       text NOT NULL,
  conversation text NOT NULL,
  thread       text,
  up_to        bigint NOT NULL,
  UNIQUE NULLS NOT DISTINCT (org, person, conversation, thread)
);

CREATE TABLE reader_kept (
  org          text NOT NULL,
  person       text NOT NULL,
  conversation text NOT NULL,
  thread       text,
  UNIQUE NULLS NOT DISTINCT (org, person, conversation, thread)
);

CREATE TABLE reader_dials (
  org          text NOT NULL,
  person       text NOT NULL,
  conversation text NOT NULL,
  thread       text,
  level        text NOT NULL,
  UNIQUE NULLS NOT DISTINCT (org, person, conversation, thread)
);

CREATE TABLE reader_focus_rules (
  org    text NOT NULL,
  person text NOT NULL,
  type   text NOT NULL,
  value  text NOT NULL,
  is_on  boolean NOT NULL,
  PRIMARY KEY (org, person, type, value)
);

CREATE TABLE reader_focus_people (
  org    text NOT NULL,
  person text NOT NULL,
  about  text NOT NULL,
  is_on  boolean NOT NULL,
  PRIMARY KEY (org, person, about)
);

-- A null means the person never touched that setting; defaults are the feed's.
CREATE TABLE reader_settings (
  org        text NOT NULL,
  person     text NOT NULL,
  auto_add   boolean,
  focus_only boolean,
  release    boolean,
  PRIMARY KEY (org, person)
);

CREATE TABLE reader_preferences (
  org    text NOT NULL,
  person text NOT NULL,
  key    text NOT NULL,
  value  text NOT NULL,
  PRIMARY KEY (org, person, key)
);

-- Who has a row open, per connection. Filled by the live connection (T-053);
-- read here so an arrival on a watched row is read in the same step.
CREATE TABLE watching (
  org          text NOT NULL,
  connection   text NOT NULL,
  person       text NOT NULL,
  conversation text NOT NULL,
  thread       text,
  UNIQUE NULLS NOT DISTINCT (org, connection, conversation, thread)
);
CREATE INDEX watching_row ON watching (org, conversation, thread);

-- The first answer to an action ID, per organization and caller. Not a fact:
-- a refusal is remembered here too and adds nothing to the log.
CREATE TABLE action_answers (
  org       text NOT NULL,
  caller    text NOT NULL,
  id        text NOT NULL,
  body_hash text NOT NULL,
  status    int NOT NULL,
  answer    jsonb NOT NULL,
  PRIMARY KEY (org, caller, id)
);

CREATE TABLE seed_sessions (
  org     text PRIMARY KEY,
  session text NOT NULL,
  sealed  boolean NOT NULL DEFAULT false
);

-- G-004: a fact changes only in a transaction that also wrote the log entry.
-- The save updates org_log's head last, so the entry at the head having been
-- written by this very transaction is what proves it. Checked at commit.
CREATE FUNCTION require_log_entry() RETURNS trigger
LANGUAGE plpgsql SET search_path = conversations AS $$
DECLARE
  o text;
BEGIN
  IF TG_OP = 'DELETE' THEN o := OLD.org; ELSE o := NEW.org; END IF;
  IF NOT EXISTS (
    SELECT 1 FROM org_log l
    JOIN change_log c ON c.org = l.org AND c.number = l.head
    WHERE l.org = o AND c.xmin = pg_current_xact_id()::xid
  ) THEN
    RAISE EXCEPTION 'G-004: % of organization % changed without a log entry in the same transaction', TG_TABLE_NAME, o
      USING ERRCODE = 'CX004';
  END IF;
  RETURN NULL;
END $$;

DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['people', 'conversations', 'conversation_people', 'messages', 'reactions',
                           'reader_reads', 'reader_kept', 'reader_dials', 'reader_focus_rules',
                           'reader_focus_people', 'reader_settings', 'reader_preferences', 'watching']
  LOOP
    EXECUTE format('CREATE CONSTRAINT TRIGGER require_log_entry AFTER INSERT OR UPDATE OR DELETE ON %I
                    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION require_log_entry()', t);
  END LOOP;
  FOREACH t IN ARRAY ARRAY['people', 'conversations', 'conversation_people', 'messages', 'reactions',
                           'reader_reads', 'reader_kept', 'reader_dials', 'reader_focus_rules',
                           'reader_focus_people', 'reader_settings', 'reader_preferences', 'watching',
                           'action_answers', 'seed_sessions']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY %I ON %I USING (org = current_setting('app.org', true))
                      WITH CHECK (org = current_setting('app.org', true))$p$, t || '_org', t);
  END LOOP;
END $$;
