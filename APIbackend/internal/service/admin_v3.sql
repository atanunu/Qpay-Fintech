-- Additive version 3: staff console. Financial history and earlier checksums are untouched.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK(role IN('customer','admin','finance','compliance','support','platform','auditor'));
CREATE TABLE staff_invitations (
 id text PRIMARY KEY, email text NOT NULL, name text NOT NULL,
 role text NOT NULL CHECK(role IN('admin','finance','compliance','support','platform','auditor')),
 maker_id text NOT NULL REFERENCES users(id), checker_id text REFERENCES users(id),
 state text NOT NULL DEFAULT 'proposed' CHECK(state IN('proposed','approved','accepted','revoked','rejected')),
 token_hash text UNIQUE, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 accepted_user_id text REFERENCES users(id), reason text NOT NULL,
 CHECK(checker_id IS NULL OR checker_id<>maker_id)
);
CREATE UNIQUE INDEX staff_open_invitation ON staff_invitations(email) WHERE state IN('proposed','approved');
CREATE TABLE admin_work_items (
 id text PRIMARY KEY,kind text NOT NULL CHECK(kind IN('risk','incident','exception','refund','return')),
 subject text NOT NULL, target text NOT NULL DEFAULT '', severity text NOT NULL CHECK(severity IN('low','medium','high','critical')),
 status text NOT NULL DEFAULT 'open' CHECK(status IN('open','investigating','awaiting_evidence','resolved')),
 assigned_to text REFERENCES users(id), created_by text NOT NULL REFERENCES users(id),
 version bigint NOT NULL DEFAULT 1, due_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_work_queue ON admin_work_items(kind,status,created_at);
CREATE TABLE admin_notes (
 id text PRIMARY KEY,resource text NOT NULL, target text NOT NULL,actor_id text NOT NULL REFERENCES users(id),
 body_enc text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER immutable_admin_notes BEFORE UPDATE OR DELETE ON admin_notes FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TABLE admin_controls (
 id text PRIMARY KEY, kind text NOT NULL CHECK(kind IN('staff_status','staff_role','staff_recovery','product','resume')),
 target text NOT NULL,value text NOT NULL,target_version bigint NOT NULL,maker_id text NOT NULL REFERENCES users(id),
 checker_id text REFERENCES users(id),status text NOT NULL DEFAULT 'pending' CHECK(status IN('pending','approved','rejected')),
 reason text NOT NULL,expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(checker_id IS NULL OR checker_id<>maker_id)
);
CREATE FUNCTION immutable_admin_proposal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-ARRAY['status','checker_id']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','checker_id']) OR OLD.status<>'pending' THEN RAISE EXCEPTION 'immutable admin proposal'; END IF; RETURN NEW;
END $$;
CREATE TRIGGER admin_proposal_snapshot BEFORE UPDATE ON admin_controls FOR EACH ROW EXECUTE FUNCTION immutable_admin_proposal();
CREATE TRIGGER admin_proposal_delete BEFORE DELETE ON admin_controls FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TABLE product_controls(id text PRIMARY KEY,enabled boolean NOT NULL DEFAULT true,version bigint NOT NULL DEFAULT 1,updated_at timestamptz NOT NULL DEFAULT now());
ALTER TABLE support_cases ADD COLUMN version bigint NOT NULL DEFAULT 1;
CREATE TABLE admin_export_events(id text PRIMARY KEY,actor_id text NOT NULL REFERENCES users(id),resource text NOT NULL,search text NOT NULL,reason text NOT NULL,row_count integer NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE staff_elevations(session_id text PRIMARY KEY REFERENCES sessions(id),expires_at timestamptz NOT NULL);

CREATE TABLE staff_recovery_grants(id text PRIMARY KEY,control_id text UNIQUE NOT NULL REFERENCES admin_controls(id),user_id text NOT NULL REFERENCES users(id),target_version bigint NOT NULL,token_hash text UNIQUE NOT NULL,expires_at timestamptz NOT NULL,consumed_at timestamptz,created_at timestamptz NOT NULL DEFAULT now());
ALTER TABLE reconciliation_runs ADD COLUMN import_digest text;
CREATE UNIQUE INDEX reconciliation_import_digest ON reconciliation_runs(import_digest) WHERE import_digest IS NOT NULL;
CREATE TRIGGER financial_proposal_snapshot BEFORE UPDATE ON proposals FOR EACH ROW EXECUTE FUNCTION immutable_admin_proposal();
CREATE TRIGGER financial_proposal_delete BEFORE DELETE ON proposals FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

ALTER TABLE proposals DROP CONSTRAINT proposals_action_check;
ALTER TABLE proposals ADD CONSTRAINT proposals_action_check CHECK(action IN('kyc_approve','kyc_decline','kyc_request_info','restrict','restore','policy'));
