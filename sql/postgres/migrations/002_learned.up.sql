ALTER TABLE talkgroups ADD COLUMN ignored BOOLEAN NOT NULL DEFAULT FALSE;

INSERT INTO talkgroups(
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	alert,
	ignored,
	learned
) SELECT
	tgl.system_id,
	tgl.tgid,
	tgl.name,
	tgl.alpha_tag,
	tgl.tg_group,
	TRUE,
	tgl.ignored,
	TRUE
FROM talkgroups_learned tgl ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS talkgroup_versions(
	-- version metadata
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	time TIMESTAMPTZ NOT NULL,
	created_by INTEGER REFERENCES users(id),
	-- talkgroup snapshot
	system_id INT4 REFERENCES systems(id),
	tgid INT4,
	name TEXT,
	alpha_tag TEXT,
	tg_group TEXT,
	frequency INTEGER,
	metadata JSONB,
	tags TEXT[],
	alert BOOLEAN,
	alert_config JSONB,
	weight REAL,
	learned BOOLEAN,
	ignored BOOLEAN
);

-- Store current version
INSERT INTO talkgroup_versions(time,
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	frequency,
	metadata,
	tags,
	alert,
	alert_config,
	weight,
	learned
) SELECT NOW(),
	tg.system_id,
	tg.tgid,
	tg.name,
	tg.alpha_tag,
	tg.tg_group,
	tg.frequency,
	tg.metadata,
	tg.tags,
	tg.alert,
	tg.alert_config,
	tg.weight,
	tg.learned
FROM talkgroups tg;

DROP TABLE IF EXISTS talkgroups_learned;
