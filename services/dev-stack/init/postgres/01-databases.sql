-- Extra databases inside the shared Postgres. Project DBs are created on
-- demand by `dev-stack db-create`; these are for stack services themselves.
SELECT 'CREATE DATABASE langfuse' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'langfuse')\gexec
SELECT 'CREATE DATABASE metabase' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'metabase')\gexec
