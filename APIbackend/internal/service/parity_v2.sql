-- Additive version 2: customer parity. Never edit the version 1 journal migration.
CREATE TABLE customer_controls (
 owner_id text PRIMARY KEY REFERENCES users(id), handle text UNIQUE CHECK(handle IS NULL OR handle ~ '^[a-z][a-z0-9_]{3,23}$'),
 discoverable boolean NOT NULL DEFAULT false, frozen boolean NOT NULL DEFAULT false,
 per_limit bigint NOT NULL DEFAULT 9000000000000000 CHECK(per_limit>0 AND per_limit<=9000000000000000),
 daily_limit bigint NOT NULL DEFAULT 9000000000000000 CHECK(daily_limit>=per_limit AND daily_limit<=9000000000000000),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE bill_favourites (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), label text NOT NULL, product_id text NOT NULL,
 customer_enc text NOT NULL, amount bigint NOT NULL CHECK(amount>0 AND amount<=9000000000000000),
 currency text NOT NULL CHECK(currency='NGN'), favourite boolean NOT NULL DEFAULT true, active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bill_favourites_owner ON bill_favourites(owner_id,id);
ALTER TABLE beneficiaries ADD COLUMN favourite boolean NOT NULL DEFAULT false;
CREATE TABLE payment_annotations (
 owner_id text NOT NULL REFERENCES users(id), payment_id text NOT NULL REFERENCES payments(id),
 category text NOT NULL, note_enc text NOT NULL DEFAULT '', excluded boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(owner_id,payment_id)
);
CREATE TABLE budgets (
 owner_id text NOT NULL REFERENCES users(id), month date NOT NULL CHECK(extract(day FROM month)=1),
 category text NOT NULL, amount bigint NOT NULL CHECK(amount>0 AND amount<=9000000000000000),
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(owner_id,month,category)
);
CREATE TABLE reminders (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),title text NOT NULL,amount bigint NOT NULL DEFAULT 0 CHECK(amount>=0 AND amount<=9000000000000000),
 bill_id text REFERENCES bill_favourites(id), due_at timestamptz NOT NULL, anchor_day integer NOT NULL CHECK(anchor_day BETWEEN 1 AND 31),
 cadence text NOT NULL CHECK(cadence IN('once','weekly','monthly')), ends_at timestamptz,
 status text NOT NULL DEFAULT 'active' CHECK(status IN('active','paused','cancelled','completed')),
 created_at timestamptz NOT NULL DEFAULT now(),version bigint NOT NULL DEFAULT 1
);
CREATE INDEX reminders_due ON reminders(status,due_at);
CREATE TABLE reminder_occurrences (
 id text PRIMARY KEY,reminder_id text NOT NULL REFERENCES reminders(id),due_at timestamptz NOT NULL,
 status text NOT NULL CHECK(status IN('notified','dismissed')), created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(reminder_id,due_at)
);
CREATE TABLE money_requests (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),amount bigint NOT NULL CHECK(amount>0 AND amount<=9000000000000000),
 currency text NOT NULL CHECK(currency='NGN'),memo text NOT NULL,expires_at timestamptz NOT NULL,
 status text NOT NULL DEFAULT 'open' CHECK(status IN('open','cancelled','completed','expired')),
 idempotency_key text NOT NULL,fingerprint text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(owner_id,idempotency_key)
);
CREATE TABLE request_shares (
 id text PRIMARY KEY,request_id text NOT NULL REFERENCES money_requests(id),payer_id text NOT NULL REFERENCES users(id),
 amount bigint NOT NULL CHECK(amount>0),received bigint NOT NULL DEFAULT 0 CHECK(received>=0 AND received<=amount),
 status text NOT NULL DEFAULT 'open' CHECK(status IN('open','declined','paid')),UNIQUE(request_id,payer_id)
);
CREATE INDEX request_shares_payer ON request_shares(payer_id,request_id);
CREATE TABLE request_quote_links (
 quote_id text PRIMARY KEY REFERENCES quotes(id),share_id text NOT NULL REFERENCES request_shares(id),
 amount bigint NOT NULL CHECK(amount>0),payment_id text UNIQUE REFERENCES payments(id),created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE private_uploads (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),purpose text NOT NULL CHECK(purpose IN('kyc','support')),
 case_id text REFERENCES support_cases(id),object_key text NOT NULL UNIQUE,mime text NOT NULL,
 size bigint NOT NULL CHECK(size>0 AND size<=5242880),digest text NOT NULL,name_enc text NOT NULL,
 state text NOT NULL CHECK(state IN('stored','clean','rejected','local_unscanned','deleted')),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX private_uploads_owner ON private_uploads(owner_id,id);
CREATE TABLE identity_drafts (
 owner_id text PRIMARY KEY REFERENCES users(id),content_enc text NOT NULL,submitted_case text REFERENCES kyc_cases(id),
 version bigint NOT NULL DEFAULT 1,updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE kyc_documents (case_id text NOT NULL REFERENCES kyc_cases(id),upload_id text NOT NULL REFERENCES private_uploads(id),PRIMARY KEY(case_id,upload_id));
CREATE TABLE verified_identities (owner_id text PRIMARY KEY REFERENCES users(id),legal_name_enc text NOT NULL,case_id text NOT NULL REFERENCES kyc_cases(id),verified_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE account_changes (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),session_id text NOT NULL REFERENCES sessions(id),
 kind text NOT NULL CHECK(kind IN('email','phone')),new_value_enc text NOT NULL,old_value_hash text NOT NULL,
 old_code_hash text NOT NULL,new_code_hash text NOT NULL,attempts integer NOT NULL DEFAULT 0,
 expires_at timestamptz NOT NULL,consumed boolean NOT NULL DEFAULT false,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE contact_phones(owner_id text PRIMARY KEY REFERENCES users(id),phone_hash text NOT NULL UNIQUE,phone_enc text NOT NULL,verified_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE closure_requests (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),reason_enc text NOT NULL,
 status text NOT NULL CHECK(status IN('pending','completed','cancelled')),created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX open_closure ON closure_requests(owner_id) WHERE status='pending';
CREATE TABLE service_subscriptions (owner_id text NOT NULL REFERENCES users(id),product_id text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(owner_id,product_id));
CREATE TABLE service_availability(product_id text PRIMARY KEY,state text NOT NULL CHECK(state IN('available','unavailable','unknown')),observed_at timestamptz NOT NULL,version bigint NOT NULL DEFAULT 1);
CREATE TABLE funding_accounts (
 id text PRIMARY KEY,owner_id text NOT NULL UNIQUE REFERENCES users(id),provider text NOT NULL,
 provider_ref text NOT NULL DEFAULT '',state text NOT NULL CHECK(state IN('requested','pending','active','review','disabled')),
 details_enc text NOT NULL DEFAULT '',idempotency_key text NOT NULL UNIQUE,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE funding_observations (
 id text PRIMARY KEY,provider text NOT NULL,reference text NOT NULL,funding_account_id text NOT NULL REFERENCES funding_accounts(id),
 amount bigint NOT NULL CHECK(amount>0),currency text NOT NULL CHECK(currency='NGN'),fingerprint text NOT NULL,
 state text NOT NULL CHECK(state IN('pending','posted','review')),created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(provider,reference)
);
CREATE TABLE webauthn_users (owner_id text NOT NULL REFERENCES users(id),rp_id text NOT NULL,handle bytea NOT NULL,PRIMARY KEY(owner_id,rp_id),UNIQUE(rp_id,handle));
CREATE TABLE webauthn_credentials (
 id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),rp_id text NOT NULL,credential_id bytea NOT NULL,
 credential_enc text NOT NULL,label text NOT NULL,revoked boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),last_used_at timestamptz,UNIQUE(rp_id,credential_id)
);
CREATE TABLE webauthn_ceremonies (
 id text PRIMARY KEY,owner_id text REFERENCES users(id),session_id text REFERENCES sessions(id),kind text NOT NULL CHECK(kind IN('register','login')),
 binding_hash text NOT NULL,data_enc text NOT NULL,expires_at timestamptz NOT NULL,consumed boolean NOT NULL DEFAULT false
);
ALTER TABLE support_cases ADD COLUMN issue_type text NOT NULL DEFAULT 'general';
ALTER TABLE support_cases ADD COLUMN response_due_at timestamptz;
ALTER TABLE support_cases ADD COLUMN escalated_at timestamptz;
ALTER TABLE support_cases ADD COLUMN assigned_to text REFERENCES users(id);
CREATE TABLE case_events(id text PRIMARY KEY,case_id text NOT NULL REFERENCES support_cases(id),actor_id text NOT NULL REFERENCES users(id),event text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE TRIGGER immutable_case_events BEFORE UPDATE OR DELETE ON case_events FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
-- Reminder execution deliberately does not debit a wallet. Autopay mandates are separately qualified.

CREATE TABLE identity_case_snapshots(case_id text PRIMARY KEY REFERENCES kyc_cases(id),content_enc text NOT NULL);
CREATE TRIGGER immutable_identity_snapshot BEFORE UPDATE OR DELETE ON identity_case_snapshots FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE UNIQUE INDEX funding_provider_ref ON funding_accounts(provider,provider_ref) WHERE provider_ref<>'';
CREATE TABLE payment_mandates(
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), quote_id text NOT NULL REFERENCES quotes(id),
 title text NOT NULL, cadence text NOT NULL CHECK(cadence IN('once','weekly','monthly')), next_at timestamptz NOT NULL,
 anchor_day integer NOT NULL CHECK(anchor_day BETWEEN 1 AND 31), ends_at timestamptz NOT NULL,
 max_debit bigint NOT NULL CHECK(max_debit>0), max_total bigint NOT NULL CHECK(max_total>=max_debit),
 max_occurrences integer NOT NULL CHECK(max_occurrences BETWEEN 1 AND 120), occurrences integer NOT NULL DEFAULT 0,
 committed_total bigint NOT NULL DEFAULT 0, version bigint NOT NULL DEFAULT 1,
 status text NOT NULL DEFAULT 'active' CHECK(status IN('active','paused','cancelled','completed')),
 terms_hash text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(occurrences<=max_occurrences AND committed_total<=max_total)
);
CREATE TABLE mandate_occurrences(id text PRIMARY KEY,mandate_id text NOT NULL REFERENCES payment_mandates(id),due_at timestamptz NOT NULL,payment_id text UNIQUE REFERENCES payments(id),status text NOT NULL CHECK(status IN('submitted','action_required')),reason text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(mandate_id,due_at));
CREATE INDEX mandates_due ON payment_mandates(status,next_at);
CREATE TRIGGER immutable_mandate_occurrence BEFORE UPDATE OR DELETE ON mandate_occurrences FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

-- Once a mandate is authorised, only occurrence bookkeeping or stop controls may change.
CREATE FUNCTION immutable_mandate_terms() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-ARRAY['next_at','occurrences','committed_total','status','version']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['next_at','occurrences','committed_total','status','version']) THEN RAISE EXCEPTION 'immutable mandate terms' USING ERRCODE='23514'; END IF;
 IF OLD.status IN('completed','cancelled') AND NEW.status<>OLD.status THEN RAISE EXCEPTION 'terminal mandate cannot restart' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER mandate_terms BEFORE UPDATE ON payment_mandates FOR EACH ROW EXECUTE FUNCTION immutable_mandate_terms();
CREATE TRIGGER mandate_delete BEFORE DELETE ON payment_mandates FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
