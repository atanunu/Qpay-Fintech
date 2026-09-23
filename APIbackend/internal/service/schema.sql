-- Version 1. Apply through the explicit migrate command, not API startup.
CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE users(
 id text PRIMARY KEY, email text NOT NULL UNIQUE CHECK(email=lower(email)), name text NOT NULL,
 password_hash text NOT NULL, pin_hash text NOT NULL DEFAULT '',
 role text NOT NULL CHECK(role IN('customer','admin','finance','compliance','support','platform')),
 status text NOT NULL DEFAULT 'active' CHECK(status IN('active','restricted','closed')),
 verified boolean NOT NULL DEFAULT false, tier integer NOT NULL DEFAULT 0 CHECK(tier BETWEEN 0 AND 3),
 mfa_enabled boolean NOT NULL DEFAULT false, mfa_secret text NOT NULL DEFAULT '', mfa_pending text NOT NULL DEFAULT '', mfa_pending_expires timestamptz,
 mfa_counter bigint NOT NULL DEFAULT -1, version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions(
 id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id), access_hash text NOT NULL UNIQUE,
 audience text NOT NULL CHECK(audience IN('customer','staff')), client text NOT NULL CHECK(client IN('web','mobile')),
 csrf_hash text NOT NULL, csrf_enc text NOT NULL, device_name text NOT NULL, mfa_ready boolean NOT NULL,
 expires_at timestamptz NOT NULL, refresh_expires_at timestamptz NOT NULL, revoked boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE refresh_tokens(token_hash text PRIMARY KEY, session_id text NOT NULL REFERENCES sessions(id), consumed boolean NOT NULL DEFAULT false, expires_at timestamptz NOT NULL);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE challenges(id text PRIMARY KEY,user_id text NOT NULL REFERENCES users(id),purpose text NOT NULL CHECK(purpose IN('verify','reset')),code_hash text NOT NULL,expires_at timestamptz NOT NULL,attempts integer NOT NULL DEFAULT 0,consumed boolean NOT NULL DEFAULT false,created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX challenges_user ON challenges(user_id,purpose,created_at DESC);
CREATE TABLE mfa_recovery_codes(user_id text NOT NULL REFERENCES users(id),code_hash text NOT NULL,consumed boolean NOT NULL DEFAULT false,PRIMARY KEY(user_id,code_hash));
CREATE TABLE rate_limits(bucket text PRIMARY KEY, window_at timestamptz NOT NULL, hits integer NOT NULL);
CREATE TABLE accounts(
 id text PRIMARY KEY, owner_id text UNIQUE REFERENCES users(id), kind text NOT NULL CHECK(kind IN('wallet','clearing','revenue')),
 currency text NOT NULL CHECK(currency='NGN'), balance bigint NOT NULL DEFAULT 0, reserved bigint NOT NULL DEFAULT 0 CHECK(reserved>=0),
 CHECK(kind<>'wallet' OR (balance>=reserved AND balance<=9000000000000000))
);
INSERT INTO accounts(id,kind,currency) VALUES ('house:clearing','clearing','NGN'),('house:fees','revenue','NGN');
CREATE TABLE journals(id text PRIMARY KEY,reference text NOT NULL UNIQUE,currency text NOT NULL CHECK(currency='NGN'),kind text NOT NULL,created_txid bigint NOT NULL DEFAULT txid_current(),created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE postings(id bigserial PRIMARY KEY,journal_id text NOT NULL REFERENCES journals(id),account_id text NOT NULL REFERENCES accounts(id),amount bigint NOT NULL CHECK(amount<>0));
CREATE INDEX postings_account ON postings(account_id,id);
CREATE FUNCTION forbid_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable financial or audit record' USING ERRCODE='23514'; END $$;
CREATE TRIGGER immutable_journals BEFORE UPDATE OR DELETE ON journals FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER immutable_postings BEFORE UPDATE OR DELETE ON postings FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE FUNCTION apply_posting() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE journal_currency text; account_currency text; journal_tx bigint;
BEGIN
 SELECT currency,created_txid INTO journal_currency,journal_tx FROM journals WHERE id=NEW.journal_id;
 IF journal_tx<>txid_current() THEN RAISE EXCEPTION 'cannot add to an existing journal' USING ERRCODE='23514'; END IF;
 SELECT currency INTO account_currency FROM accounts WHERE id=NEW.account_id FOR UPDATE;
 IF journal_currency IS DISTINCT FROM account_currency THEN RAISE EXCEPTION 'currency mismatch' USING ERRCODE='23514'; END IF;
 UPDATE accounts SET balance=balance+NEW.amount WHERE id=NEW.account_id;
 RETURN NEW;
END $$;
CREATE TRIGGER posting_projection BEFORE INSERT ON postings FOR EACH ROW EXECUTE FUNCTION apply_posting();
CREATE FUNCTION balanced_journal() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE n bigint; total numeric; target text;
BEGIN
 IF TG_TABLE_NAME='journals' THEN target:=NEW.id; ELSE target:=NEW.journal_id; END IF;
 SELECT count(*),coalesce(sum(amount::numeric),0) INTO n,total FROM postings WHERE journal_id=target;
 IF n<2 OR total<>0 THEN RAISE EXCEPTION 'journal must have at least two balanced postings' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER journal_balance AFTER INSERT ON journals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION balanced_journal();
CREATE CONSTRAINT TRIGGER posting_balance AFTER INSERT ON postings DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION balanced_journal();
CREATE TABLE policies(id integer PRIMARY KEY CHECK(id=1),version bigint NOT NULL DEFAULT 1,per_payment bigint NOT NULL CHECK(per_payment>0),daily bigint NOT NULL CHECK(daily>=per_payment),internal_fee bigint NOT NULL DEFAULT 0 CHECK(internal_fee>=0),payments_enabled boolean NOT NULL DEFAULT false);
-- Deliberately disabled baseline, not a representation of regulatory tier limits.
INSERT INTO policies(id,per_payment,daily) VALUES(1,10000000,50000000);
CREATE TABLE enquiries(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),kind text NOT NULL CHECK(kind IN('bank','bill')),destination_enc text NOT NULL,amount bigint NOT NULL DEFAULT 0,name text NOT NULL,expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE beneficiaries(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),enquiry_id text NOT NULL REFERENCES enquiries(id),label text NOT NULL,active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX beneficiaries_owner ON beneficiaries(owner_id,id);
CREATE TABLE quotes(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),kind text NOT NULL CHECK(kind IN('internal','bank','bill')),destination_enc text NOT NULL,amount bigint NOT NULL CHECK(amount>0),fee bigint NOT NULL CHECK(fee>=0),total bigint NOT NULL CHECK(total=amount+fee),currency text NOT NULL CHECK(currency='NGN'),narration text NOT NULL,policy_version bigint NOT NULL,expires_at timestamptz NOT NULL,used boolean NOT NULL DEFAULT false,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE authorisations(token_hash text PRIMARY KEY,quote_id text NOT NULL REFERENCES quotes(id),session_id text NOT NULL REFERENCES sessions(id),expires_at timestamptz NOT NULL,consumed boolean NOT NULL DEFAULT false);
CREATE TABLE payments(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),recipient_id text REFERENCES users(id),quote_id text NOT NULL UNIQUE REFERENCES quotes(id),idempotency_key text NOT NULL,request_hash text NOT NULL,kind text NOT NULL CHECK(kind IN('internal','bank','bill')),status text NOT NULL CHECK(status IN('accepted','submitted','pending','succeeded','failed','pending_review')),amount bigint NOT NULL CHECK(amount>0),fee bigint NOT NULL CHECK(fee>=0),total bigint NOT NULL CHECK(total=amount+fee),currency text NOT NULL CHECK(currency='NGN'),destination_enc text NOT NULL,narration text NOT NULL,provider_reference text NOT NULL DEFAULT '',fulfilment_status text NOT NULL DEFAULT 'not_applicable' CHECK(fulfilment_status IN('not_applicable','pending','ready')),fulfilment_enc text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),UNIQUE(owner_id,idempotency_key));
CREATE INDEX payments_owner ON payments(owner_id,id DESC);
CREATE TABLE holds(id text PRIMARY KEY,payment_id text NOT NULL UNIQUE REFERENCES payments(id),account_id text NOT NULL REFERENCES accounts(id),amount bigint NOT NULL CHECK(amount>0),status text NOT NULL DEFAULT 'active' CHECK(status IN('active','captured','released')),created_at timestamptz NOT NULL DEFAULT now());
CREATE FUNCTION apply_hold() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.status<>'active' THEN RAISE EXCEPTION 'hold must start active' USING ERRCODE='23514'; END IF;
  UPDATE accounts SET reserved=reserved+NEW.amount WHERE id=NEW.account_id;
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.account_id<>OLD.account_id OR NEW.amount<>OLD.amount OR NEW.payment_id<>OLD.payment_id OR NEW.id<>OLD.id OR OLD.status<>'active' OR NEW.status='active' THEN RAISE EXCEPTION 'invalid hold transition' USING ERRCODE='23514'; END IF;
  UPDATE accounts SET reserved=reserved-NEW.amount WHERE id=NEW.account_id;
 ELSE RAISE EXCEPTION 'holds cannot be deleted' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER hold_projection BEFORE INSERT OR UPDATE OR DELETE ON holds FOR EACH ROW EXECUTE FUNCTION apply_hold();
CREATE TABLE jobs(id text PRIMARY KEY,kind text NOT NULL CHECK(kind='payment'),object_id text NOT NULL UNIQUE REFERENCES payments(id),status text NOT NULL DEFAULT 'queued' CHECK(status IN('queued','leased','done','dead')),attempts integer NOT NULL DEFAULT 0,available_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,lease_token text NOT NULL DEFAULT '',last_error text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX jobs_due ON jobs(status,available_at);
CREATE TABLE observations(id text PRIMARY KEY,payment_id text NOT NULL REFERENCES payments(id),status text NOT NULL,reference text NOT NULL,amount bigint NOT NULL,currency text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TRIGGER immutable_observations BEFORE UPDATE OR DELETE ON observations FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TABLE funding(id text PRIMARY KEY,source text NOT NULL,reference text NOT NULL,owner_id text NOT NULL REFERENCES users(id),amount bigint NOT NULL CHECK(amount>0),currency text NOT NULL CHECK(currency='NGN'),fingerprint text NOT NULL,journal_id text NOT NULL REFERENCES journals(id),created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(source,reference));
CREATE TABLE notification_intents(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),workflow text NOT NULL,reference text NOT NULL,occurrence text NOT NULL,recipient_email text NOT NULL,payload_enc text NOT NULL,secret boolean NOT NULL DEFAULT false,state text NOT NULL DEFAULT 'queued' CHECK(state IN('queued','leased','submitted','unknown','suppressed','expired','dead')),attempts integer NOT NULL DEFAULT 0,available_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,read_at timestamptz,expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(owner_id,workflow,reference,occurrence));
CREATE INDEX notifications_owner ON notification_intents(owner_id,id DESC);
CREATE TABLE preferences(owner_id text PRIMARY KEY REFERENCES users(id),optional_email boolean NOT NULL DEFAULT false,marketing_email boolean NOT NULL DEFAULT false,updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE suppressions(email_hash text PRIMARY KEY,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE delivery_events(id text PRIMARY KEY,notification_id text REFERENCES notification_intents(id),provider text NOT NULL,provider_message_id text NOT NULL,event_type text NOT NULL,event_fingerprint text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE kyc_cases(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),status text NOT NULL DEFAULT 'submitted' CHECK(status IN('submitted','approved','declined','information_required')),evidence_ref text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE UNIQUE INDEX one_open_kyc ON kyc_cases(owner_id) WHERE status IN('submitted','information_required');
CREATE TABLE proposals(id text PRIMARY KEY,maker_id text NOT NULL REFERENCES users(id),action text NOT NULL CHECK(action IN('kyc_approve','kyc_decline','restrict','restore','policy')),target text NOT NULL,target_version bigint NOT NULL,payload jsonb NOT NULL,reason text NOT NULL,status text NOT NULL DEFAULT 'pending' CHECK(status IN('pending','approved','rejected')),checker_id text REFERENCES users(id),expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),CHECK(checker_id IS NULL OR checker_id<>maker_id));
CREATE TABLE support_cases(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),payment_id text REFERENCES payments(id),kind text NOT NULL CHECK(kind IN('support','complaint','dispute')),subject text NOT NULL,status text NOT NULL DEFAULT 'open' CHECK(status IN('open','awaiting_customer','resolved','escalated')),created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE case_messages(id text PRIMARY KEY,case_id text NOT NULL REFERENCES support_cases(id),author_id text NOT NULL REFERENCES users(id),body text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE reconciliation_runs(id text PRIMARY KEY,actor_id text NOT NULL REFERENCES users(id),source text NOT NULL CHECK(source IN('provider','bank')),created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE reconciliation_items(id text PRIMARY KEY,run_id text NOT NULL REFERENCES reconciliation_runs(id),reference text NOT NULL,amount bigint NOT NULL,currency text NOT NULL,status text NOT NULL,matched boolean NOT NULL,reason text NOT NULL,UNIQUE(run_id,reference));
CREATE TABLE audit_events(id text PRIMARY KEY,actor text NOT NULL,action text NOT NULL,target text NOT NULL,details jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TRIGGER immutable_audit BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE simulated_operations(reference text PRIMARY KEY,request_hash text NOT NULL,result jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE FUNCTION immutable_quote() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-'used') IS DISTINCT FROM (to_jsonb(OLD)-'used') OR OLD.used OR NOT NEW.used THEN RAISE EXCEPTION 'immutable quote' USING ERRCODE='23514'; END IF; RETURN NEW; END $$;
CREATE TRIGGER quote_snapshot BEFORE UPDATE ON quotes FOR EACH ROW EXECUTE FUNCTION immutable_quote();
CREATE TRIGGER quote_delete BEFORE DELETE ON quotes FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE FUNCTION immutable_payment_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-ARRAY['status','provider_reference','fulfilment_status','fulfilment_enc','updated_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','provider_reference','fulfilment_status','fulfilment_enc','updated_at']) THEN RAISE EXCEPTION 'immutable payment intent' USING ERRCODE='23514'; END IF; RETURN NEW; END $$;
CREATE TRIGGER payment_snapshot BEFORE UPDATE ON payments FOR EACH ROW EXECUTE FUNCTION immutable_payment_snapshot();
CREATE TRIGGER payment_delete BEFORE DELETE ON payments FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER immutable_funding BEFORE UPDATE OR DELETE ON funding FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
