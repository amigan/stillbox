CREATE TABLE IF NOT EXISTS users(
	id SERIAL PRIMARY KEY,
	username VARCHAR (255) UNIQUE NOT NULL,
	password TEXT NOT NULL,
	email TEXT NOT NULL,
	is_admin BOOLEAN,
	prefs JSONB
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users(username);

CREATE TABLE IF NOT EXISTS api_keys(
	id SERIAL PRIMARY KEY,
	owner INTEGER REFERENCES users(id),
	created_at TIMESTAMP NOT NULL,
	expires TIMESTAMP,
	disabled BOOLEAN,
	api_key UUID UNIQUE NOT NULL
);

CREATE INDEX IF NOT EXISTS api_keys_api_key ON api_keys(api_key);

CREATE TABLE IF NOT EXISTS systems(
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS talkgroups(
	system_id INTEGER REFERENCES systems(id) NOT NULL,
	tgid INTEGER,
	name TEXT,
	frequency INTEGER,
	auto_created BOOLEAN,
	metadata JSONB,
	PRIMARY KEY (system_id, tgid)
);

CREATE TABLE IF NOT EXISTS talkgroups_tags(
	system_id INTEGER NOT NULL,
	talkgroup_id INTEGER NOT NULL,
	tags TEXT[] NOT NULL DEFAULT '{}',
	FOREIGN KEY (system_id, talkgroup_id) REFERENCES talkgroups (system_id, tgid),
	PRIMARY KEY (system_id, talkgroup_id)
);
CREATE INDEX IF NOT EXISTS talkgroup_tags_id_tags ON talkgroups_tags USING GIN (tags);

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
	frequency INTEGER,
	frequencies JSONB,
	patches JSONB,
	tg_label TEXT,
	source TEXT,
	transcript TEXT
);


CREATE INDEX IF NOT EXISTS calls_transcript_idx ON calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS calls_call_date_tg_idx ON calls(talkgroup, call_date);

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
	-- CONSTRAINT incident_call_pkey PRIMARY KEY (incident_id, call_id)
	PRIMARY KEY (incident_id, call_id)
);
