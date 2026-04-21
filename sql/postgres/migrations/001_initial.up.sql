CREATE TABLE IF NOT EXISTS users(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY (START WITH 1),
	username VARCHAR (255) UNIQUE NOT NULL,
	password TEXT NOT NULL,
	real_name TEXT,
	email TEXT UNIQUE NOT NULL,
	roles TEXT[],
	disabled_at TIMESTAMPTZ,
	last_login_at TIMESTAMPTZ,
	last_login_from INET,
	password_set_at TIMESTAMPTZ NOT NULL,
	prefs JSONB
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users(username);

CREATE TABLE IF NOT EXISTS api_keys(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	owner_id INTEGER REFERENCES users(id) NOT NULL,
	name VARCHAR (255),
	created_at TIMESTAMP NOT NULL,
	expires TIMESTAMP,
	disabled BOOLEAN NOT NULL,
	kind INTEGER NOT NULL,
	api_key TEXT UNIQUE,
	jwt_id UUID UNIQUE,
	scopes TEXT[],
	UNIQUE (owner_id, name)
);

CREATE TABLE IF NOT EXISTS systems(
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	learned BOOLEAN NOT NULL DEFAULT FALSE
);

-- NB: if the column defaults are updated here, they must also be updated in the UpsertTalkgroup query
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
	alert_rules JSONB,
	weight REAL NOT NULL DEFAULT 1.0,
	learned BOOLEAN NOT NULL DEFAULT FALSE,
	ignored BOOLEAN NOT NULL DEFAULT FALSE,
	UNIQUE (system_id, tgid)
);

CREATE TYPE talkgroup_tuple AS (
	system INT4,
	tg INT4
);

CREATE INDEX talkgroups_system_tgid_idx ON talkgroups (system_id, tgid);

CREATE INDEX IF NOT EXISTS talkgroup_id_tags ON talkgroups USING GIN (tags);

CREATE TABLE IF NOT EXISTS talkgroup_versions(
	-- version metadata
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	time TIMESTAMPTZ NOT NULL,
	created_by INTEGER REFERENCES users(id),
	deleted BOOLEAN,
	-- talkgroup snapshot
	system_id INT4, 
	tgid INT4,
	name TEXT,
	alpha_tag TEXT,
	tg_group TEXT,
	frequency INTEGER,
	metadata JSONB,
	tags TEXT[],
	alert BOOLEAN,
	alert_rules JSONB,
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
	metadata JSONB,
	context_window TSTZRANGE,
	url_tag TEXT UNIQUE
);

CREATE TYPE audio_mime AS ENUM ('audio/mpeg', 'audio/wav');

CREATE TABLE IF NOT EXISTS calls(
	id UUID,
	submitter INTEGER REFERENCES users(id) ON DELETE SET NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	call_date TIMESTAMPTZ NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	duration INTEGER,
	audio_type audio_mime,
	audio_ref JSONB,
	frequency INTEGER NOT NULL,
	frequencies INTEGER[],
	patches INTEGER[],
	talker_alias TEXT,
	tg_label TEXT,
	tg_alpha_tag TEXT,
	tg_group TEXT,
	source INTEGER NOT NULL,
	transcript TEXT,
	dangling_at TIMESTAMP,
	PRIMARY KEY (id, call_date),
	FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system_id, tgid)
) PARTITION BY RANGE (call_date);

CREATE INDEX IF NOT EXISTS calls_transcript_idx ON calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS calls_call_date_tg_idx ON calls(call_date, talkgroup, system);
CREATE INDEX IF NOT EXISTS calls_call_date_brin_idx ON calls USING brin (call_date);
CREATE INDEX IF NOT EXISTS calls_talker_alias ON calls(talker_alias);

CREATE TABLE swept_calls (
	id UUID PRIMARY KEY,
	submitter INTEGER REFERENCES users(id) ON DELETE SET NULL,
	system INTEGER NOT NULL,
	talkgroup INTEGER NOT NULL,
	call_date TIMESTAMPTZ NOT NULL,
	audio_name TEXT,
	audio_blob BYTEA,
	duration INTEGER,
	audio_type audio_mime,
	audio_ref JSONB,
	frequency INTEGER NOT NULL,
	frequencies INTEGER[],
	patches INTEGER[],
	talker_alias TEXT,
	tg_label TEXT,
	tg_alpha_tag TEXT,
	tg_group TEXT,
	source INTEGER NOT NULL,
	transcript TEXT,
	dangling_at TIMESTAMP,
	FOREIGN KEY (system, talkgroup) REFERENCES talkgroups(system_id, tgid)
);

CREATE INDEX IF NOT EXISTS swept_calls_transcript_idx ON swept_calls USING GIN (to_tsvector('english', transcript));
CREATE INDEX IF NOT EXISTS swept_calls_call_date_tg_idx ON swept_calls(system, talkgroup, call_date);
CREATE INDEX IF NOT EXISTS swept_calls_talker_alias ON calls(talker_alias);


CREATE TABLE IF NOT EXISTS settings(
	name TEXT PRIMARY KEY,
	updated_by INTEGER REFERENCES users(id),
	value JSONB
);

CREATE TABLE IF NOT EXISTS incidents(
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	owner_id INTEGER NOT NULL,
	description TEXT,
	created_at TIMESTAMPTZ,
	start_time TIMESTAMPTZ,
	end_time TIMESTAMPTZ,
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

CREATE TABLE IF NOT EXISTS shares(
	id TEXT PRIMARY KEY,
	entity_type TEXT NOT NULL,
	entity_id UUID NOT NULL,
	entity_date TIMESTAMPTZ,
	owner_id INTEGER NOT NULL REFERENCES users(id),
	expiration TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS audio_ref_journal(
	id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	call_id UUID,
	backend TEXT NOT NULL,
	ref JSONB,
	prune_after TIMESTAMPTZ,
	last_try TIMESTAMPTZ NOT NULL, -- when tries = 0, last_try means creation time of the entry
	tries INTEGER NOT NULL,
	CHECK(ref IS NOT NULL OR call_id IS NOT NULL),
	UNIQUE NULLS NOT DISTINCT (call_id, backend, ref)
);

CREATE TABLE IF NOT EXISTS webpush_subscriptions(
	id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	user_id INTEGER REFERENCES users(id) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	expiration TIMESTAMPTZ,
	subscription JSONB NOT NULL,
	UNIQUE(subscription)
);

CREATE INDEX IF NOT EXISTS webpush_subscriptions_user_id_idx ON webpush_subscriptions(user_id);

CREATE TABLE IF NOT EXISTS talkgroup_notification_subscriptions(
	user_id INTEGER REFERENCES users(id),
	system_id INT4 NOT NULL,
	tgid INT4 NOT NULL,
	PRIMARY KEY(user_id, system_id, tgid),
	FOREIGN KEY (system_id, tgid) REFERENCES talkgroups (system_id, tgid)
);

CREATE INDEX IF NOT EXISTS talkgroup_notification_subscriptions_system_id_tgid_idx ON talkgroup_notification_subscriptions(system_id, tgid);

CREATE INDEX IF NOT EXISTS talkgroup_notification_subscriptions_user_id ON talkgroup_notification_subscriptions(user_id);

CREATE TABLE IF NOT EXISTS system_notification_subscriptions(
	user_id INTEGER REFERENCES users(id),
	system_id INTEGER REFERENCES systems(id),
	PRIMARY KEY (user_id, system_id)
);

CREATE INDEX IF NOT EXISTS system_notification_subscriptions_system_idx ON system_notification_subscriptions(system_id);
