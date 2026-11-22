-- Which organizations the demonstration has made, and which one is current (D-033).
-- Only this program knows. A row is operator bookkeeping about an organization, holds
-- none of its content (G-025), and is removed last when the organization is deleted.
CREATE SEQUENCE organization_number;

CREATE TABLE organizations (
    id             text PRIMARY KEY,
    name           text NOT NULL,
    state          text NOT NULL CHECK (state IN ('preparing', 'current', 'retired')),
    created_at     timestamptz NOT NULL,
    through        bigint,            -- the seal's number; NULL until the seed is complete
    approx_changes bigint NOT NULL DEFAULT 0,
    retire_told    boolean NOT NULL DEFAULT false  -- Conversations has confirmed the retirement
);

CREATE UNIQUE INDEX one_current ON organizations ((true)) WHERE state = 'current';
CREATE INDEX organizations_to_tell ON organizations (id) WHERE state = 'retired' AND NOT retire_told;
