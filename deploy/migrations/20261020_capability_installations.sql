BEGIN;
CREATE TABLE IF NOT EXISTS capability_installations (
 id bigserial PRIMARY KEY,
 capability_key varchar(128) NOT NULL,
 capability_version_id bigint NOT NULL,
 version bigint,
 method varchar(32) NOT NULL,
 source varchar(1024),
 location varchar(256) NOT NULL,
 digest varchar(128) NOT NULL,
 artifact_sha256 varchar(64) NOT NULL,
 evidence_source varchar(128) NOT NULL,
 installed_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_capability_installations_capability_key ON capability_installations(capability_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_capability_installations_capability_version_id ON capability_installations(capability_version_id);
COMMIT;
