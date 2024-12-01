CREATE TABLE IF NOT EXISTS users(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	username VARCHAR (255) UNIQUE NOT NULL,
	password TEXT NOT NULL,
	email TEXT NOT NULL,
	is_admin BOOLEAN NOT NULL,
	prefs JSONB
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users(username);

CREATE TABLE IF NOT EXISTS api_keys(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
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
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	system_id INT4 REFERENCES systems(id) NOT NULL,
	tgid INT4 NOT NULL,
	name TEXT,
	alpha_tag TEXT,
	tg_group TEXT,
	frequency INTEGER,
	metadata JSONB,
	tags TEXT[] NOT NULL DEFAULT '{}',
	alert BOOLEAN NOT NULL DEFAULT 'true',
	alert_config JSONB,
	weight REAL NOT NULL DEFAULT 1.0,
	learned BOOLEAN NOT NULL DEFAULT FALSE,
	ignored BOOLEAN NOT NULL DEFAULT FALSE,
	UNIQUE (system_id, tgid)
);

CREATE INDEX talkgroups_system_tgid_idx ON talkgroups (system_id, tgid);

CREATE INDEX IF NOT EXISTS talkgroup_id_tags ON talkgroups USING GIN (tags);

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

CREATE TABLE IF NOT EXISTS alerts(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	time TIMESTAMPTZ NOT NULL, 
	tgid INTEGER NOT NULL,
	system_id INTEGER REFERENCES systems(id) NOT NULL,
	weight REAL,
	score REAL,
	orig_score REAL,
	notified BOOLEAN NOT NULL DEFAULT 'false',
	metadata JSONB
);

CREATE TABLE IF NOT EXISTS calls(
	id UUID,
	submitter INTEGER REFERENCES api_keys(id) ON DELETE SET NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	call_date TIMESTAMPTZ NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	duration INTEGER,
	audio_type TEXT,
	audio_url TEXT,
	frequency INTEGER NOT NULL,
	frequencies INTEGER[],
	patches INTEGER[],
	tg_label TEXT,
	tg_alpha_tag TEXT,
	tg_group TEXT,
	source INTEGER NOT NULL,
	transcript TEXT,
	PRIMARY KEY (id, call_date),
	FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system_id, tgid)
) PARTITION BY RANGE (call_date);

CREATE INDEX IF NOT EXISTS calls_transcript_idx ON calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS calls_call_date_tg_idx ON calls(system, talkgroup, call_date);

CREATE TABLE swept_calls (
	id UUID PRIMARY KEY,
	submitter INTEGER REFERENCES api_keys(id) ON DELETE SET NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	call_date TIMESTAMPTZ NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	duration INTEGER,
	audio_type TEXT,
	audio_url TEXT,
	frequency INTEGER NOT NULL,
	frequencies INTEGER[],
	patches INTEGER[],
	tg_label TEXT,
	tg_alpha_tag TEXT,
	tg_group TEXT,
	source INTEGER NOT NULL,
	transcript TEXT,
	FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system_id, tgid)
);

CREATE INDEX IF NOT EXISTS swept_calls_transcript_idx ON swept_calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS swept_calls_call_date_tg_idx ON swept_calls(system, talkgroup, call_date);


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
	incident_id UUID NOT NULL REFERENCES incidents(id) ON UPDATE CASCADE ON DELETE CASCADE,
	call_id UUID NOT NULL,
	calls_tbl_id UUID NULL,
	swept_call_id UUID NULL REFERENCES swept_calls(id),
	call_date TIMESTAMPTZ NULL,
	notes JSONB,
	FOREIGN KEY (calls_tbl_id, call_date) REFERENCES calls(id, call_date),
	PRIMARY KEY (incident_id, call_id)
);
