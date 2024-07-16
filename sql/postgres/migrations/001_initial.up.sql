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

CREATE TABLE IF NOT EXISTS systems(
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS talkgroups(
	id INTEGER NOT NULL,
	system INTEGER REFERENCES systems(id) NOT NULL,
	name TEXT,
	frequency INTEGER,
	group_id INTEGER,
	auto_created BOOLEAN,
	metadata JSONB,
	PRIMARY KEY (tgid, system)
);

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
CREATE INDEX IF NOT EXISTS calls_date_tg_idx ON calls(talkgroup, date);

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

CREATE INDEX IF NOT EXISTS calls_name_description_idx ON calls USING GIN (
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
