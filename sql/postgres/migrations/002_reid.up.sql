ALTER TABLE talkgroups ALTER COLUMN system_id DROP EXPRESSION;

ALTER TABLE talkgroups ALTER COLUMN tgid DROP EXPRESSION;

ALTER TABLE talkgroups ALTER COLUMN id SET DATA TYPE UUID USING (gen_random_uuid());

CREATE INDEX IF NOT EXISTS talkgroups_system_tgid_idx ON talkgroups (system_id, tgid);
