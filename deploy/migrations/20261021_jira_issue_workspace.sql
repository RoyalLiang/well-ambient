-- Additive only. Back up the database before applying; keep key and tables on app rollback.
BEGIN;
CREATE TABLE IF NOT EXISTS jira_issue_workspaces (
 id bigserial PRIMARY KEY,
 version bigint,
 config_json text,
 credentials text,
 catalog_json text,
 lease varchar(100),
 lease_until timestamptz,
 run_id bigint,
 cancelled boolean,
 daily_key varchar(100),
 updated_at timestamptz
);
CREATE TABLE IF NOT EXISTS jira_issue_runs (
 id bigserial PRIMARY KEY,
 kind varchar(24),
 config_version bigint,
 status varchar(24),
 message varchar(500),
 result_json text,
 item_count bigint,
 created_at timestamptz,
 updated_at timestamptz
);
CREATE TABLE IF NOT EXISTS jira_issue_links (
 id bigserial PRIMARY KEY,
 scope_key varchar(64),
 issue_id bigint,
 issue_iid bigint,
 jira_key varchar(64),
 jira_origin varchar(500),
 git_lab_origin varchar(500),
 project_key varchar(40),
 closed boolean,
 closed_code_hash varchar(64),
 created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_jira_issue_links_scope_key ON jira_issue_links(scope_key);
CREATE INDEX IF NOT EXISTS idx_jira_issue_links_jira_key ON jira_issue_links(jira_key);
COMMIT;
