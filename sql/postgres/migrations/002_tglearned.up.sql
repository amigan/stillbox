DROP TRIGGER IF EXISTS learn_tg ON calls;
DROP FUNCTION IF EXISTS learn_talkgroup();

CREATE SEQUENCE IF NOT EXISTS talkgroups_id_seq START WITH 1;
ALTER TABLE talkgroups ALTER COLUMN id SET DATA TYPE INTEGER USING (nextval('talkgroups_id_seq'));
ALTER TABLE talkgroups ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY;
DROP SEQUENCE IF EXISTS talkgroups_id_seq;

ALTER TABLE talkgroups ADD COLUMN IF NOT EXISTS learned BOOLEAN NOT NULL DEFAULT FALSE;

-- calls fkey constraint requires us to migrate all calls' talkgroup tuples to exist
INSERT INTO talkgroups (system_id, tgid, learned)
SELECT DISTINCT system_id, tgid, TRUE FROM talkgroups_learned ON CONFLICT DO NOTHING;

INSERT INTO talkgroups (system_id, tgid, learned)
SELECT DISTINCT system, talkgroup, TRUE FROM calls ON CONFLICT DO NOTHING;

INSERT INTO talkgroups_learned (system_id, tgid, name, tg_group, alpha_tag)
SELECT DISTINCT c.system, c.talkgroup, c.tg_label, c.tg_group, c.tg_alpha_tag
FROM calls c
JOIN talkgroups t ON (t.system_id = c.system AND t.tgid = c.talkgroup AND t.learned IS TRUE)
ON CONFLICT DO NOTHING;

ALTER TABLE calls ADD CONSTRAINT calls_system_talkgroup_fkey FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system_id, tgid);
