-- The agent door (T-054, D-008, D-032). None of this is a fact on the log: events
-- are what an agent is told about facts, written in the step that saves them;
-- the text of a reply in progress and an agent's status stay off the log on
-- purpose (G-017).

-- Each agent's own event numbers. floor is the newest number trimmed away.
CREATE TABLE agent_heads (
  org   text NOT NULL,
  agent text NOT NULL,
  head  bigint NOT NULL DEFAULT 0,
  floor bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (org, agent)
);

CREATE TABLE agent_events (
  org     text NOT NULL,
  agent   text NOT NULL,
  number  bigint NOT NULL,
  at      timestamptz NOT NULL,
  message text NOT NULL,
  body    jsonb NOT NULL,        -- the whole AgentEvent of contracts/schema
  PRIMARY KEY (org, agent, number)
);
CREATE INDEX agent_events_message ON agent_events (org, message);
CREATE INDEX agent_events_at ON agent_events (org, agent, at);

CREATE TABLE agent_subscriptions (
  org          text NOT NULL,
  agent        text NOT NULL,
  conversation text NOT NULL,
  PRIMARY KEY (org, agent, conversation)
);
CREATE INDEX agent_subscriptions_conversation ON agent_subscriptions (org, conversation);

-- How many agents in a row stand behind an agent's message. A person's message
-- has no row here: it counts zero.
CREATE TABLE agent_chains (
  org     text NOT NULL,
  message text NOT NULL,
  chain   int NOT NULL,
  PRIMARY KEY (org, message)
);

-- A reply in progress, under the agent's own ID. text is what was last saved,
-- about once a second; a closed row only remembers that it closed.
CREATE TABLE agent_replies (
  org           text NOT NULL,
  agent         text NOT NULL,
  id            text NOT NULL,
  message       text NOT NULL,
  conversation  text NOT NULL,
  thread        text,
  because_of    bigint,
  started_at    timestamptz NOT NULL,
  last_piece_at timestamptz NOT NULL,
  due_at        timestamptz NOT NULL,
  text          text NOT NULL DEFAULT '',
  closed_at     timestamptz,
  PRIMARY KEY (org, agent, id),
  UNIQUE (org, message)
);
CREATE INDEX agent_replies_open ON agent_replies (org, conversation, due_at) WHERE closed_at IS NULL;

-- The latest status of an agent. Outside the numbering: the latest counts.
CREATE TABLE agent_status (
  org       text NOT NULL,
  agent     text NOT NULL,
  state     text NOT NULL CHECK (state IN ('working', 'waiting', 'done')),
  on_person text,
  PRIMARY KEY (org, agent)
);

CREATE INDEX people_agents ON people (org) WHERE is_agent;

-- Which organizations have housekeeping coming, and when. Times only, never
-- content (G-025), and read through the two indexes below, so idle organizations
-- cost nothing (G-029).
CREATE TABLE agent_org_due (
  org           text PRIMARY KEY,
  reply_due_at  timestamptz,
  events_due_at timestamptz
);
CREATE INDEX agent_org_due_replies ON agent_org_due (reply_due_at) WHERE reply_due_at IS NOT NULL;
CREATE INDEX agent_org_due_events ON agent_org_due (events_due_at) WHERE events_due_at IS NOT NULL;

DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['agent_heads', 'agent_events', 'agent_subscriptions', 'agent_chains', 'agent_replies', 'agent_status']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY %I ON %I USING (org = current_setting('app.org', true))
                      WITH CHECK (org = current_setting('app.org', true))$p$, t || '_org', t);
  END LOOP;
END $$;
