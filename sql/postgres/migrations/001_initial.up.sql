CREATE TABLE IF NOT EXISTS users(
	id SERIAL PRIMARY KEY,
	username VARCHAR (255) UNIQUE NOT NULL,
	password TEXT NOT NULL,
	email TEXT NOT NULL,
	is_admin BOOLEAN NOT NULL,
	prefs JSONB
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users(username);

CREATE TABLE IF NOT EXISTS api_keys(
	id SERIAL PRIMARY KEY,
	owner INTEGER REFERENCES users(id) NOT NULL,
	created_at TIMESTAMP NOT NULL,
	expires TIMESTAMP,
	disabled BOOLEAN,
	api_key TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS systems(
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS talkgroups(
	system_id INTEGER REFERENCES systems(id) NOT NULL,
	tgid INTEGER,
	name TEXT,
	tg_group TEXT,
	frequency INTEGER,
	metadata JSONB,
	tags TEXT[] NOT NULL DEFAULT '{}',
	PRIMARY KEY (system_id, tgid)
);

CREATE INDEX IF NOT EXISTS talkgroup_id_tags ON talkgroups USING GIN (tags);

CREATE TABLE IF NOT EXISTS talkgroups_learned(
	id SERIAL PRIMARY KEY,
	system_id INTEGER REFERENCES systems(id) NOT NULL,
	tgid INTEGER NOT NULL,
	name TEXT NOT NULL,
	group_tag TEXT,
	ignored BOOLEAN,
	UNIQUE (system_id, tgid, name)
);

CREATE OR REPLACE FUNCTION learn_talkgroup()
RETURNS TRIGGER AS $$
BEGIN
	IF NOT EXISTS (
		SELECT tg.system_id, tg.tgid, tg.name FROM talkgroups tg WHERE tg.system_id = NEW.system AND tg.tgid = NEW.talkgroup
		UNION
		SELECT tgl.system_id, tgl.tgid, tgl.name FROM talkgroups_learned tgl WHERE tgl.system_id = NEW.system AND tgl.tgid = NEW.talkgroup
	) THEN
		INSERT INTO talkgroups_learned(system_id, tgid, name, group_tag) VALUES(
			NEW.system, NEW.talkgroup, NEW.tg_label, NEW.tg_tag
		) ON CONFLICT DO NOTHING;
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS calls(
	id UUID PRIMARY KEY,
	submitter INTEGER REFERENCES api_keys(id) ON DELETE SET NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	call_date TIMESTAMP NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	audio_type TEXT,
	audio_url TEXT,
	frequency INTEGER NOT NULL,
	frequencies INTEGER[],
	patches INTEGER[],
	tg_label TEXT,
	tg_tag TEXT,
	tg_group TEXT,
	source INTEGER NOT NULL,
	transcript TEXT
);


CREATE OR REPLACE TRIGGER learn_tg AFTER INSERT ON calls
FOR EACH ROW EXECUTE FUNCTION learn_talkgroup();

CREATE INDEX IF NOT EXISTS calls_transcript_idx ON calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS calls_call_date_tg_idx ON calls(system, talkgroup, call_date);

CREATE TABLE IF NOT EXISTS settings(
	name TEXT PRIMARY KEY,
	updated_by INTEGER REFERENCES users(id),
	value JSONB
);

CREATE TABLE IF NOT EXISTS incidents(
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	start_time TIMESTAMP,
	end_time TIMESTAMP,
	location JSONB,
	metadata JSONB
);

CREATE INDEX IF NOT EXISTS incidents_name_description_idx ON incidents USING GIN (
	(to_tsvector('english', name) || to_tsvector('english', coalesce(description, ''))
	)
);

CREATE TABLE IF NOT EXISTS incidents_calls(
	incident_id UUID REFERENCES incidents(id) ON UPDATE CASCADE ON DELETE CASCADE,
	call_id UUID REFERENCES calls(id) ON UPDATE CASCADE,
	notes JSONB,
	PRIMARY KEY (incident_id, call_id)
);
