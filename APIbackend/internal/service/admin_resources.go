package service

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Every query is a reviewed allowlist projection. Never select secret-bearing rows wholesale.
// Pagination is deterministic descending text ID; chronology is displayed separately.
type adminResource struct {
	Roles []string
	SQL   string
}

var adminResources = map[string]adminResource{
	"customers":       {[]string{"admin", "finance", "compliance", "support", "auditor"}, `SELECT id,name,left(email,1)||'***@'||split_part(email,'@',2) AS email,role,status,verified AS email_verified,tier,mfa_enabled,version,created_at FROM users WHERE role='customer'`},
	"staff":           {[]string{"admin", "auditor"}, `SELECT id,name,email,role,status,mfa_enabled,version,created_at FROM users WHERE role<>'customer'`},
	"invitations":     {[]string{"admin", "auditor"}, `SELECT id,email,name,role,maker_id,checker_id,CASE WHEN expires_at<now() AND state IN('proposed','approved') THEN 'expired' ELSE state END AS status,expires_at,reason,created_at FROM staff_invitations`},
	"payments":        {[]string{"admin", "finance", "compliance", "support", "auditor"}, `SELECT id,owner_id,recipient_id,kind,status,amount::text AS amount_minor,fee::text AS fee_minor,total::text AS total_minor,currency,narration,provider_reference,fulfilment_status,created_at,updated_at FROM payments`},
	"kyc":             {[]string{"admin", "compliance", "auditor"}, `SELECT k.id,k.owner_id,u.name,k.status,u.tier,u.version,k.created_at,(SELECT count(*) FROM kyc_documents d WHERE d.case_id=k.id) AS document_count FROM kyc_cases k JOIN users u ON u.id=k.owner_id`},
	"proposals":       {[]string{"admin", "finance", "compliance", "auditor"}, `SELECT id,maker_id,checker_id,action,target,target_version,payload,reason,CASE WHEN expires_at<now() AND status='pending' THEN 'expired' ELSE status END AS status,expires_at,created_at FROM proposals`},
	"controls":        {[]string{"admin", "platform", "auditor"}, `SELECT id,kind,target,value,target_version,maker_id,checker_id,status,reason,expires_at,created_at FROM admin_controls`},
	"support":         {[]string{"admin", "support"}, `SELECT id,owner_id,payment_id,kind,subject,issue_type,status,assigned_to,response_due_at,escalated_at,version,created_at,updated_at FROM support_cases`},
	"jobs":            {[]string{"admin", "platform", "finance", "auditor"}, `SELECT id,object_id,status,attempts,available_at,lease_until,last_error,created_at FROM jobs`},
	"notifications":   {[]string{"admin", "platform", "auditor"}, `SELECT id,owner_id,workflow,reference,state AS status,secret AS contains_secret,attempts,read_at,expires_at,created_at FROM notification_intents`},
	"audit":           {[]string{"admin", "compliance", "auditor"}, `SELECT id,actor,action,target,details,created_at FROM audit_events`},
	"reconciliations": {[]string{"admin", "finance", "auditor"}, `SELECT r.id,r.source,r.actor_id,r.created_at,(SELECT count(*) FROM reconciliation_items i WHERE i.run_id=r.id) AS row_count,(SELECT count(*) FROM reconciliation_items i WHERE i.run_id=r.id AND NOT matched) AS exceptions FROM reconciliation_runs r`},
	"exceptions":      {[]string{"admin", "finance", "auditor"}, `SELECT i.id,i.run_id,i.reference,i.amount::text AS amount_minor,i.currency,i.status,i.reason,r.created_at FROM reconciliation_items i JOIN reconciliation_runs r ON r.id=i.run_id WHERE NOT i.matched`},
	"ledger":          {[]string{"admin", "finance", "auditor"}, `SELECT id,reference,kind,currency,created_at FROM journals`},
	"treasury":        {[]string{"admin", "finance", "auditor"}, `SELECT id,owner_id,kind,currency,balance::text AS balance_minor,reserved::text AS held_minor,(balance-reserved)::text AS available_minor FROM accounts`},
	"funding":         {[]string{"admin", "finance", "support", "auditor"}, `SELECT id,owner_id,provider,provider_ref,state AS status,created_at,updated_at FROM funding_accounts`},
	"schedules":       {[]string{"admin", "finance", "support", "auditor"}, `SELECT id,owner_id,title,cadence,status,next_at,ends_at,occurrences,max_occurrences,max_debit::text AS max_debit_minor,max_total::text AS max_total_minor,committed_total::text AS committed_minor,version,created_at FROM payment_mandates`},
	"requests":        {[]string{"admin", "support", "compliance", "auditor"}, `SELECT id,owner_id,amount::text AS amount_minor,currency,memo,status,expires_at,created_at FROM money_requests`},
	"reminders":       {[]string{"admin", "support", "auditor"}, `SELECT id,owner_id,title,cadence,status,due_at,version,created_at FROM reminders`},
	"risk":            {[]string{"admin", "compliance"}, `SELECT id,kind,subject,target,severity,status,assigned_to,created_by,version,due_at,created_at,updated_at FROM admin_work_items WHERE kind='risk'`},
	"incidents":       {[]string{"admin", "platform", "auditor"}, `SELECT id,kind,subject,target,severity,status,assigned_to,created_by,version,due_at,created_at,updated_at FROM admin_work_items WHERE kind='incident'`},
	"investigations":  {[]string{"admin", "finance", "auditor"}, `SELECT id,kind,subject,target,severity,status,assigned_to,created_by,version,due_at,created_at,updated_at FROM admin_work_items WHERE kind IN('exception','refund','return')`},
	"privacy":         {[]string{"admin", "compliance", "auditor"}, `SELECT id,owner_id,status,created_at FROM closure_requests`},
	"products":        {[]string{"admin", "platform", "finance", "auditor"}, `SELECT id,enabled,version,updated_at FROM product_controls`},
	"availability":    {[]string{"admin", "platform", "support", "auditor"}, `SELECT product_id AS id,CASE WHEN observed_at<now()-interval '5 minutes' THEN 'unknown' ELSE state END AS status,observed_at,version FROM service_availability`},
	"exports":         {[]string{"admin", "finance", "compliance", "auditor"}, `SELECT id,actor_id,resource,search,reason,row_count,created_at FROM admin_export_events`},
}

type AdminPage struct {
	Items       []map[string]any `json:"items"`
	NextBefore  string           `json:"next_before"`
	HasMore     bool             `json:"has_more"`
	GeneratedAt time.Time        `json:"generated_at"`
}
type AdminQuery struct {
	Search, Before, ID string
	Limit              int
}

func (s *Service) consolePrincipal(ctx context.Context, tx *sql.Tx, p Principal, roles ...string) (Principal, error) {
	u, e := s.activePrincipal(ctx, tx, p)
	if e != nil {
		return p, e
	}
	p.User = u
	if !u.MFA {
		return p, denied()
	}
	return p, requireRole(p, roles...)
}
func readAdminRows(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]map[string]any, error) {
	rows, e := tx.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var item map[string]any
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.UseNumber()
		if e = d.Decode(&item); e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func resourceSQL(resource, role string) (adminResource, error) {
	r, ok := adminResources[resource]
	if !ok {
		return r, missing()
	}
	if resource == "proposals" && role == "finance" {
		r.SQL += ` WHERE action='policy'`
	}
	if resource == "proposals" && role == "compliance" {
		r.SQL += ` WHERE action<>'policy'`
	}
	if resource == "controls" && role == "platform" {
		r.SQL += ` WHERE kind IN('product','resume')`
	}
	return r, nil
}
func (s *Service) AdminList(ctx context.Context, p Principal, resource string, q AdminQuery) (AdminPage, error) {
	out := AdminPage{Items: []map[string]any{}, GeneratedAt: s.Now()}
	r, e := resourceSQL(resource, p.User.Role)
	if e != nil {
		return out, e
	}
	if q.Limit < 1 || q.Limit > 100 || len(q.Search) > 100 || len(q.Before) > 150 || len(q.ID) > 150 {
		return out, Invalid("invalid page or search bounds")
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, r.Roles...)
		if err != nil {
			return err
		}
		r, err = resourceSQL(resource, p.User.Role)
		if err != nil {
			return err
		}
		out.Items, err = readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (`+r.SQL+`) t WHERE ($1='' OR id<$1) AND ($2='' OR id=$2) AND ($3='' OR position(lower($3) in lower(to_jsonb(t)::text))>0) ORDER BY id DESC LIMIT $4`, q.Before, q.ID, q.Search, q.Limit+1)
		if err != nil {
			return err
		}
		out.HasMore = len(out.Items) > q.Limit
		out.NextBefore = ""
		if out.HasMore {
			out.Items = out.Items[:q.Limit]
			out.NextBefore = out.Items[len(out.Items)-1]["id"].(string)
		}
		return s.audit(ctx, tx, p.User.ID, "admin.read", resource, map[string]any{"count": len(out.Items), "filtered": q.Search != ""})
	})
	return out, e
}
func (s *Service) AdminBootstrap(ctx context.Context, p Principal) (map[string]any, error) {
	out := map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin", "finance", "compliance", "support", "platform", "auditor")
		if err != nil {
			return err
		}
		resources := []string{}
		for key, r := range adminResources {
			if requireRole(p, r.Roles...) == nil {
				resources = append(resources, key)
			}
		}
		sort.Strings(resources)
		actions := []string{}
		for key, roles := range adminActionRoles {
			if requireRole(p, roles...) == nil {
				actions = append(actions, key)
			}
		}
		sort.Strings(actions)
		out = map[string]any{"user": p.User, "resources": resources, "actions": actions, "environment": s.Config.Environment, "synthetic_execution": s.Config.Environment == "local", "server_time": s.Now(), "external_execution_configured": s.Config.ExternalEnabled, "notification_mode": s.Config.NotificationMode, "schedules_configured": s.Config.SchedulesEnabled, "private_storage_configured": s.Config.UploadStore != nil, "scanner_configured": s.Config.UploadScanner != nil, "live_acceptance": false, "gates": []string{"Live QPay partner qualification", "Refund/return execution adapter", "Three-way settlement completeness", "Live Novu delivery acceptance", "Specialised products are not enabled"}}
		return s.audit(ctx, tx, p.User.ID, "admin.session_opened", p.User.ID, map[string]any{})
	})
	return out, e
}
func (s *Service) AdminMetrics(ctx context.Context, p Principal) (map[string]any, error) {
	out := map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin", "finance", "compliance", "support", "platform", "auditor")
		if err != nil {
			return err
		}
		sqlText := `SELECT json_build_object('customers',(SELECT count(*) FROM users WHERE role='customer'),'pending_payments',(SELECT count(*) FROM payments WHERE status IN('accepted','submitted','pending','pending_review')),'open_support',(SELECT count(*) FROM support_cases WHERE status<>'resolved'),'kyc_waiting',(SELECT count(*) FROM kyc_cases WHERE status IN('submitted','information_required')),'approvals',(SELECT count(*) FROM proposals WHERE status='pending' AND expires_at>now())+(SELECT count(*) FROM admin_controls WHERE status='pending' AND expires_at>now()),'dead_jobs',(SELECT count(*) FROM jobs WHERE status='dead'),'exceptions',(SELECT count(*) FROM reconciliation_items WHERE NOT matched),'payments_enabled',(SELECT payments_enabled FROM policies WHERE id=1))`
		data, err := readAdminRows(ctx, tx, sqlText)
		if err != nil {
			return err
		}
		out = data[0]
		if requireRole(p, "admin", "finance", "auditor") == nil {
			data, err = readAdminRows(ctx, tx, `SELECT json_build_object('currency','NGN','wallet_liability_minor',coalesce(sum(balance::numeric) FILTER(WHERE kind='wallet'),0)::text,'held_minor',coalesce(sum(reserved::numeric) FILTER(WHERE kind='wallet'),0)::text,'fee_income_minor',coalesce(sum(balance::numeric) FILTER(WHERE kind='revenue'),0)::text,'clearing_minor',coalesce(sum(balance::numeric) FILTER(WHERE kind='clearing'),0)::text,'unbalanced_journals',(SELECT count(*) FROM (SELECT j.id FROM journals j LEFT JOIN postings p ON p.journal_id=j.id GROUP BY j.id HAVING count(p.id)<2 OR sum(p.amount::numeric)<>0) b)) FROM accounts`)
			if err != nil {
				return err
			}
			out["ledger"] = data[0]
		}
		out["generated_at"] = s.Now()
		out["scope"] = "Database operational snapshot. Clearing is not independently verified bank float; imported reconciliation rows do not establish settlement completeness."
		return s.audit(ctx, tx, p.User.ID, "admin.metrics_read", "overview", map[string]any{})
	})
	return out, e
}

// AdminDetail adds permissioned related records, never plaintext secrets or unrestricted destinations.
func (s *Service) AdminDetail(ctx context.Context, p Principal, resource, id string) (map[string]any, error) {
	page, e := s.AdminList(ctx, p, resource, AdminQuery{ID: id, Limit: 1})
	if e != nil {
		return nil, e
	}
	if len(page.Items) != 1 {
		return nil, missing()
	}
	out := map[string]any{"record": page.Items[0]}
	r, _ := resourceSQL(resource, p.User.Role)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, r.Roles...)
		if err != nil {
			return err
		}
		related := map[string]string{}
		switch resource {
		case "customers":
			related["wallet"] = `SELECT row_to_json(t) FROM (SELECT currency,balance::text AS balance_minor,reserved::text AS held_minor FROM accounts WHERE owner_id=$1) t`
			related["sessions"] = `SELECT row_to_json(t) FROM (SELECT id,device_name,audience,revoked,created_at,expires_at FROM sessions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100) t`
		case "payments":
			related["observations"] = `SELECT row_to_json(t) FROM (SELECT id,status,amount::text AS amount_minor,currency,created_at FROM observations WHERE payment_id=$1 ORDER BY created_at LIMIT 200) t`
			related["holds"] = `SELECT row_to_json(t) FROM (SELECT id,status,amount::text AS amount_minor,created_at FROM holds WHERE payment_id=$1) t`
			related["journals"] = `SELECT row_to_json(t) FROM (SELECT id,kind,currency,created_at FROM journals WHERE reference=$1) t`
			related["jobs"] = `SELECT row_to_json(t) FROM (SELECT id,status,attempts,last_error,available_at FROM jobs WHERE object_id=$1) t`
		case "ledger":
			related["postings"] = `SELECT row_to_json(t) FROM (SELECT id::text,account_id,amount::text AS delta_minor FROM postings WHERE journal_id=$1 ORDER BY id) t`
		case "kyc":
			if p.User.Role != "auditor" {
				related["documents"] = `SELECT row_to_json(t) FROM (SELECT u.id,u.mime,u.size,u.state AS status,u.created_at FROM kyc_documents d JOIN private_uploads u ON u.id=d.upload_id WHERE d.case_id=$1) t`
			}
		case "support":
			related["events"] = `SELECT row_to_json(t) FROM (SELECT id,actor_id,event,created_at FROM case_events WHERE case_id=$1 ORDER BY created_at LIMIT 500) t`
			related["documents"] = `SELECT row_to_json(t) FROM (SELECT id,mime,size,state AS status,created_at FROM private_uploads WHERE case_id=$1 AND purpose='support' AND state<>'deleted') t`
		case "schedules":
			related["occurrences"] = `SELECT row_to_json(t) FROM (SELECT id,payment_id,status,reason,due_at,created_at FROM mandate_occurrences WHERE mandate_id=$1 ORDER BY created_at DESC LIMIT 120) t`
		case "requests":
			related["shares"] = `SELECT row_to_json(t) FROM (SELECT id,payer_id,amount::text AS amount_minor,received::text AS received_minor,status FROM request_shares WHERE request_id=$1 ORDER BY id) t`
		case "notifications":
			related["events"] = `SELECT row_to_json(t) FROM (SELECT id,provider,event_type,created_at FROM delivery_events WHERE notification_id=$1 ORDER BY created_at LIMIT 200) t`
		case "reconciliations":
			related["items"] = `SELECT row_to_json(t) FROM (SELECT id,reference,amount::text AS amount_minor,currency,status,matched,reason FROM reconciliation_items WHERE run_id=$1 ORDER BY id LIMIT 2001) t`
		case "staff":
			related["sessions"] = `SELECT row_to_json(t) FROM (SELECT id,device_name,revoked,created_at,expires_at FROM sessions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100) t`
		}
		for name, query := range related {
			data, err := readAdminRows(ctx, tx, query, id)
			if err != nil {
				return err
			}
			out[name] = data
		}
		notes, err := readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (SELECT id,actor_id,body_enc,created_at FROM admin_notes WHERE resource=$1 AND target=$2 ORDER BY created_at DESC LIMIT 200) t`, resource, id)
		if err != nil {
			return err
		}
		for _, n := range notes {
			body, err := s.Config.Box.Open(n["body_enc"].(string), "admin-note:"+n["id"].(string))
			if err != nil {
				return err
			}
			delete(n, "body_enc")
			n["message"] = body
		}
		out["notes"] = notes
		return s.audit(ctx, tx, p.User.ID, "admin.detail_read", id, map[string]any{"resource": resource})
	})
	return out, e
}
func (s *Service) AdminExport(ctx context.Context, p Principal, resource, search, reason string) (string, error) {
	r, e := resourceSQL(resource, p.User.Role)
	if e != nil {
		return "", e
	}
	if !safeText(reason, 500) || len(search) > 100 {
		return "", Invalid("bounded export reason and search required")
	}
	var csvText string
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin", "finance", "compliance", "auditor")
		if err != nil {
			return err
		}
		if err = requireRole(p, r.Roles...); err != nil {
			return err
		}
		r, err = resourceSQL(resource, p.User.Role)
		if err != nil {
			return err
		}
		items, err := readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (`+r.SQL+`) t WHERE $1='' OR position(lower($1) in lower(to_jsonb(t)::text))>0 ORDER BY id DESC LIMIT 5001`, search)
		if err != nil {
			return err
		}
		if len(items) > 5000 {
			return &Fault{422, "export_too_large", "Narrow the filter; no truncated export was produced"}
		}
		keys := []string{}
		if len(items) > 0 {
			for k := range items[0] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		var b strings.Builder
		w := csv.NewWriter(&b)
		if err = w.Write(keys); err != nil {
			return err
		}
		for _, item := range items {
			cells := []string{}
			for _, k := range keys {
				v := ""
				if item[k] != nil {
					if text, ok := item[k].(string); ok {
						v = text
					} else {
						raw, _ := json.Marshal(item[k])
						v = string(raw)
					}
				}
				if strings.IndexAny(strings.TrimLeft(v, " \r\n\t"), "=+@-") == 0 {
					v = "'" + v
				}
				cells = append(cells, v)
			}
			if err = w.Write(cells); err != nil {
				return err
			}
		}
		w.Flush()
		if err = w.Error(); err != nil {
			return err
		}
		csvText = b.String()
		if err = exec(tx, ctx, `INSERT INTO admin_export_events(id,actor_id,resource,search,reason,row_count) VALUES($1,$2,$3,$4,$5,$6)`, stringID("export_"), p.User.ID, resource, search, reason, len(items)); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.User.ID, "admin.export", resource, map[string]any{"rows": strconv.Itoa(len(items)), "reason": reason})
	})
	return csvText, e
}

func (s *Service) RecordEvidenceReason(ctx context.Context, p Principal, id, reason string) error {
	if !safeText(reason, 500) {
		return Invalid("access reason required")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, "admin", "compliance", "support")
		if e != nil {
			return e
		}
		var purpose string
		if e = tx.QueryRowContext(ctx, `SELECT purpose FROM private_uploads WHERE id=$1`, id).Scan(&purpose); e != nil {
			return isMissing(e)
		}
		roles := []string{"admin", "compliance"}
		if purpose == "support" {
			roles = []string{"admin", "support"}
		}
		if e = requireRole(p, roles...); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "admin.evidence_access", id, map[string]any{"reason": reason})
	})
}
