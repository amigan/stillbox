CREATE TABLE IF NOT EXISTS talkgroups_learned(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	system_id INTEGER REFERENCES systems(id) NOT NULL,
	tgid INTEGER NOT NULL,
	name TEXT NOT NULL,
	alpha_tag TEXT,
	tg_group TEXT,
	ignored BOOLEAN,
	UNIQUE (system_id, tgid, name)
);

DROP TABLE IF EXISTS talkgroup_versions;

INSERT INTO talkgroups_learned(
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	ignored
) SELECT
	tg.system_id,
	tg.tgid,
	tg.name,
	tg.alpha_tag,
	tg.tg_group
	tg.ignored
FROM talkgroups tg;

ALTER TABLE talkgroups DROP COLUMN ignored;
