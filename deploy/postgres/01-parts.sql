-- One login and one schema per part (D-005, G-008). A schema is reachable only
-- by its owner. Delivery works on Conversations' log: its rights are granted
-- table by table with those tables, never on the schema's contents as a whole.
-- These passwords protect nothing: the database is local to the Compose network.
REVOKE ALL ON SCHEMA public FROM PUBLIC;

CREATE ROLE conversations LOGIN PASSWORD 'conversations_dev' NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE delivery      LOGIN PASSWORD 'delivery_dev'      NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE feed          LOGIN PASSWORD 'feed_dev'          NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE search        LOGIN PASSWORD 'search_dev'        NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE sender        LOGIN PASSWORD 'sender_dev'        NOSUPERUSER NOCREATEDB NOCREATEROLE;
CREATE ROLE demo          LOGIN PASSWORD 'demo_dev'          NOSUPERUSER NOCREATEDB NOCREATEROLE;

CREATE SCHEMA conversations AUTHORIZATION conversations;
CREATE SCHEMA feed          AUTHORIZATION feed;
CREATE SCHEMA search        AUTHORIZATION search;
CREATE SCHEMA sender        AUTHORIZATION sender;
CREATE SCHEMA demo          AUTHORIZATION demo;

GRANT USAGE ON SCHEMA conversations TO delivery;

ALTER ROLE conversations SET search_path = conversations;
ALTER ROLE delivery      SET search_path = conversations;
ALTER ROLE feed          SET search_path = feed;
ALTER ROLE search        SET search_path = search;
ALTER ROLE sender        SET search_path = sender;
ALTER ROLE demo          SET search_path = demo;
