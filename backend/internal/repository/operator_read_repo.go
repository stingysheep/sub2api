package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

type operatorReadRepository struct{ client *dbent.Client }

func NewOperatorReadRepository(client *dbent.Client) service.OperatorReadRepository {
	return &operatorReadRepository{client}
}
func (r *operatorReadRepository) base() *dbent.UserQuery {
	return r.client.User.Query().Where(user.RoleEQ(service.RoleUser), user.DeletedAtIsNil())
}
func toOperatorUser(u *dbent.User) service.OperatorUser {
	return service.OperatorUser{ID: u.ID, Email: u.Email, Username: u.Username, Status: u.Status, Balance: u.Balance, FreeBalance: u.FreeBalance, PaidBalance: u.PaidBalance, CreatedAt: u.CreatedAt, LastActiveAt: u.LastActiveAt}
}
func (r *operatorReadRepository) ListUsers(ctx context.Context, p, n int, q string) ([]service.OperatorUser, int64, error) {
	b := r.base()
	if q != "" {
		b = b.Where(user.Or(user.EmailContainsFold(q), user.UsernameContainsFold(q)))
	}
	total, e := b.Count(ctx)
	if e != nil {
		return nil, 0, e
	}
	rows, e := b.Order(dbent.Desc(user.FieldCreatedAt)).Offset((p-1)*n).Limit(n).
		Select(user.FieldID, user.FieldEmail, user.FieldUsername, user.FieldStatus, user.FieldBalance, user.FieldFreeBalance, user.FieldPaidBalance, user.FieldCreatedAt, user.FieldLastActiveAt).All(ctx)
	if e != nil {
		return nil, 0, e
	}
	out := make([]service.OperatorUser, len(rows))
	for i := range rows {
		out[i] = toOperatorUser(rows[i])
	}
	return out, int64(total), nil
}
func (r *operatorReadRepository) GetUser(ctx context.Context, id int64) (*service.OperatorUser, error) {
	u, e := r.base().Where(user.IDEQ(id)).
		Select(user.FieldID, user.FieldEmail, user.FieldUsername, user.FieldStatus, user.FieldBalance, user.FieldFreeBalance, user.FieldPaidBalance, user.FieldCreatedAt, user.FieldLastActiveAt).Only(ctx)
	if e != nil {
		if dbent.IsNotFound(e) {
			return nil, service.ErrOperatorUserNotFound
		}
		return nil, e
	}
	out := toOperatorUser(u)
	return &out, nil
}
func (r *operatorReadRepository) ListGroups(ctx context.Context) ([]service.OperatorGroup, error) {
	rows, e := r.client.Group.Query().Where(group.DeletedAtIsNil()).Select(group.FieldID, group.FieldName, group.FieldPlatform, group.FieldStatus, group.FieldModelPricing, group.FieldUpdatedAt).Order(dbent.Asc(group.FieldName)).All(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]service.OperatorGroup, len(rows))
	for i, g := range rows {
		var pricing []service.ChannelModelPricing
		if len(g.ModelPricing) > 0 {
			if e = json.Unmarshal(g.ModelPricing, &pricing); e != nil {
				return nil, e
			}
		}
		out[i] = service.OperatorGroup{ID: g.ID, Name: g.Name, Platform: g.Platform, Status: g.Status, ModelCount: len(pricing), UpdatedAt: g.UpdatedAt}
	}
	return out, nil
}

// A target, count and page share one SQL snapshot and one fixed authorization
// predicate. A concurrent role change cannot authorize an unscoped related read.
const operatorTargetSQL = `WITH target AS (SELECT id FROM users WHERE id=$1 AND role='user' AND deleted_at IS NULL)`

func (r *operatorReadRepository) ListOrders(ctx context.Context, id int64, p, n int) ([]service.OperatorOrder, int64, error) {
	rows, e := r.client.QueryContext(ctx, operatorTargetSQL+`, scoped AS (SELECT o.id,o.pay_amount::text amount,COALESCE(NULLIF(o.provider_snapshot->>'currency',''),'CNY') currency,o.status,o.created_at,o.paid_at FROM payment_orders o JOIN target t ON t.id=o.user_id) SELECT (SELECT COUNT(*) FROM scoped),v.id,v.amount,v.currency,v.status,v.created_at,v.paid_at FROM target LEFT JOIN LATERAL (SELECT * FROM scoped ORDER BY created_at DESC,id DESC OFFSET $2 LIMIT $3) v ON TRUE`, id, (p-1)*n, n)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := make([]service.OperatorOrder, 0, n)
	var total int64
	found := false
	for rows.Next() {
		found = true
		var rid sql.NullInt64
		var amount, currency, status sql.NullString
		var created, paid sql.NullTime
		if e = rows.Scan(&total, &rid, &amount, &currency, &status, &created, &paid); e != nil {
			return nil, 0, e
		}
		if rid.Valid {
			v := service.OperatorOrder{ID: rid.Int64, Amount: amount.String, Currency: currency.String, Status: status.String, CreatedAt: created.Time}
			if paid.Valid {
				v.PaidAt = &paid.Time
			}
			out = append(out, v)
		}
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	if !found {
		return nil, 0, service.ErrOperatorUserNotFound
	}
	return out, total, nil
}
func (r *operatorReadRepository) Usage(ctx context.Context, id int64, start, end time.Time) (*service.OperatorUsage, error) {
	rows, e := r.client.QueryContext(ctx, operatorTargetSQL+` SELECT COUNT(l.id),COALESCE(SUM(l.input_tokens),0),COALESCE(SUM(l.output_tokens),0),COALESCE(SUM(l.actual_cost),0)::text FROM target t LEFT JOIN usage_logs l ON l.user_id=t.id AND l.created_at >= $2 AND l.created_at < $3 GROUP BY t.id`, id, start, end)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	if !rows.Next() {
		if e = rows.Err(); e != nil {
			return nil, e
		}
		return nil, service.ErrOperatorUserNotFound
	}
	v := &service.OperatorUsage{}
	if e = rows.Scan(&v.RequestCount, &v.PromptTokens, &v.CompletionTokens, &v.UsageAmount); e != nil {
		return nil, e
	}
	v.TotalTokens = v.PromptTokens + v.CompletionTokens
	return v, rows.Err()
}
func (r *operatorReadRepository) ListBalanceHistory(ctx context.Context, id int64, p, n int) ([]service.OperatorBalanceHistory, int64, error) {
	rows, e := r.client.QueryContext(ctx, operatorTargetSQL+`, scoped AS (
 SELECT r.id,r.value amount,r.balance_source source,COALESCE(r.used_at,r.created_at) time,CASE WHEN r.type='operator_balance' THEN COALESCE(r.notes::jsonb->>'reason','') ELSE '' END reason FROM redeem_codes r JOIN target t ON t.id=r.used_by WHERE r.type IN ('balance','admin_balance','operator_balance') AND r.status='used'
 UNION ALL SELECT -a.id,a.amount,'free',a.created_at,'' FROM user_affiliate_ledger a JOIN target t ON t.id=a.user_id WHERE a.action='transfer'
 ) SELECT (SELECT COUNT(*) FROM scoped),v.id,ABS(v.amount)::text,CASE WHEN v.amount<0 THEN 'subtract' ELSE 'add' END,v.source,v.time,v.reason FROM target LEFT JOIN LATERAL (SELECT * FROM scoped ORDER BY time DESC,id DESC OFFSET $2 LIMIT $3) v ON TRUE`, id, (p-1)*n, n)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := make([]service.OperatorBalanceHistory, 0, n)
	var total int64
	found := false
	for rows.Next() {
		found = true
		var rid sql.NullInt64
		var amount, operation, source, reason sql.NullString
		var at sql.NullTime
		if e = rows.Scan(&total, &rid, &amount, &operation, &source, &at, &reason); e != nil {
			return nil, 0, e
		}
		if rid.Valid {
			out = append(out, service.OperatorBalanceHistory{ID: rid.Int64, Amount: amount.String, Operation: operation.String, Source: source.String, Time: at.Time, Reason: reason.String})
		}
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	if !found {
		return nil, 0, service.ErrOperatorUserNotFound
	}
	return out, total, nil
}
func (r *operatorReadRepository) ListChannelStatus(ctx context.Context) ([]service.OperatorChannelStatus, error) {
	rows, e := r.client.QueryContext(ctx, `SELECT m.id,m.name,COALESCE(h.status,'unknown'),h.latency_ms,COALESCE(h.checked_at,m.updated_at) FROM channel_monitors m LEFT JOIN LATERAL (SELECT status,latency_ms,checked_at FROM channel_monitor_histories WHERE monitor_id=m.id ORDER BY checked_at DESC,id DESC LIMIT 1) h ON TRUE WHERE m.enabled=TRUE ORDER BY m.monitor_sort_order,m.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]service.OperatorChannelStatus, 0)
	for rows.Next() {
		var v service.OperatorChannelStatus
		var latency sql.NullInt64
		if e = rows.Scan(&v.ID, &v.Name, &v.Status, &latency, &v.UpdatedAt); e != nil {
			return nil, e
		}
		if latency.Valid {
			v.LatencyMs = &latency.Int64
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
