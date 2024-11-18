ALTER TABLE calls DROP CONSTRAINT IF EXISTS calls_system_talkgroup_fkey;

ALTER TABLE talkgroups DROP COLUMN IF EXISTS learned;

ALTER TABLE talkgroups ALTER COLUMN id DROP IDENTITY IF EXISTS;
ALTER TABLE talkgroups ALTER COLUMN id SET DATA TYPE UUID USING (gen_random_uuid());
DROP SEQUENCE IF EXISTS talkgroups_id_seq;

CREATE OR REPLACE FUNCTION learn_talkgroup()
RETURNS TRIGGER AS $$
BEGIN
	IF NOT EXISTS (
		 SELECT tg.system_id, tg.tgid, tg.name, tg.alpha_tag FROM talkgroups tg WHERE tg.system_id = NEW.system AND tg.tgid = NEW.talkgroup
		 UNION
		 SELECT tgl.system_id, tgl.tgid, tgl.name, tgl.alpha_tag FROM talkgroups_learned tgl WHERE tgl.system_id = NEW.system AND tgl.tgid = NEW.talkgroup
	) THEN
		 INSERT INTO talkgroups_learned(system_id, tgid, name, alpha_tag) VALUES(
			NEW.system, NEW.talkgroup, NEW.tg_label, NEW.tg_alpha_tag
		 ) ON CONFLICT DO NOTHING;
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER learn_tg AFTER INSERT ON calls FOR EACH ROW EXECUTE FUNCTION learn_talkgroup();
