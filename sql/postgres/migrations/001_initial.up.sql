CREATE TABLE IF NOT EXISTS users(
	id SERIAL PRIMARY KEY,
	username VARCHAR (255) UNIQUE NOT NULL,
	password TEXT NOT NULL,
	email TEXT NOT NULL,
	is_admin BOOLEAN,
	prefs JSONB
);

CREATE INDEX IF NOT EXISTS users_username ON users(username);

CREATE TABLE IF NOT EXISTS apikeys(
	id SERIAL PRIMARY KEY,
	owner INTEGER REFERENCES users(id),
	api_key TEXT UNIQUE NOT NULL
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
	PRIMARY KEY (tgid, system)
);

CREATE TABLE IF NOT EXISTS calls(
	id UUID PRIMARY KEY,
	submitter INTEGER REFERENCES apikeys(id) NOT NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	date TIMESTAMP NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	audio_type TEXT,
	audio_url TEXT,
	frequency INTEGER,
	frequencies JSONB,
	patches JSONB,
	tg_label TEXT,
	source TEXT,
	FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system, id)
);

CREATE INDEX IF NOT EXISTS calls_date_tg ON calls(talkgroup, date);

CREATE TABLE IF NOT EXISTS settings(
	name TEXT PRIMARY KEY,
	updated_by INTEGER REFERENCES users(id),
	value JSONB
);
