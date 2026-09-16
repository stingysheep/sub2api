package repository

import (
	"context"
	"encoding/json"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"math/big"
	"regexp"
	"strings"
)

var operatorReceiptCodePattern = regexp.MustCompile(`^op_[a-z2-7]{28}$`)
var operatorFingerprintPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type operatorBalanceRepository struct{ client *dbent.Client }

func NewOperatorBalanceRepository(client *dbent.Client) service.OperatorBalanceRepository {
	return &operatorBalanceRepository{client}
}

type operatorBalanceReceipt struct {
	Version     int                                     `json:"version"`
	ActorID     int64                                   `json:"actor_id"`
	TargetID    int64                                   `json:"target_id"`
	Fingerprint string                                  `json:"fingerprint"`
	Reason      string                                  `json:"reason"`
	Source      string                                  `json:"source"`
	Delta       string                                  `json:"delta"`
	Result      service.OperatorBalanceAdjustmentResult `json:"result"`
}

func operatorDecimal(value string) (*big.Int, error) {
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
	}
	if strings.Trim(value, "0.") == "" {
		return new(big.Int), nil
	}
	v, err := service.ParseOperatorBalanceAmount(value)
	if err != nil {
		return nil, err
	}
	if negative {
		v.Neg(v)
	}
	return v, nil
}
func operatorInRange(v *big.Int) bool {
	max, _ := new(big.Int).SetString("99999999999999999999", 10)
	return new(big.Int).Abs(v).Cmp(max) <= 0
}
func (r *operatorBalanceRepository) AdjustOperatorBalance(ctx context.Context, c service.OperatorBalanceCommand) (*service.OperatorBalanceAdjustmentResult, error) {
	if c.ActorID <= 0 || c.TargetID <= 0 || !operatorReceiptCodePattern.MatchString(c.OperationID) || !operatorFingerprintPattern.MatchString(c.Fingerprint) || (c.Request.Operation != "add" && c.Request.Operation != "subtract") || (c.Request.Operation == "add" && c.Request.Source != "free" && c.Request.Source != "paid") || (c.Request.Operation == "subtract" && c.Request.Source != "") {
		return nil, service.ErrOperatorBalanceInvalid
	}
	if _, err := service.ParseOperatorBalanceAmount(c.Request.Amount); err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.Request.Reason) == "" || len(c.Request.Reason) > 1000 {
		return nil, service.ErrOperatorBalanceInvalid
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	ctx = dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	rows, err := client.QueryContext(ctx, `SELECT id, role, status, balance::text, free_balance::text, paid_balance::text, free_balance_issued::text, total_recharged::text FROM users WHERE id IN ($1,$2) AND deleted_at IS NULL ORDER BY id FOR UPDATE`, c.ActorID, c.TargetID)
	if err != nil {
		return nil, err
	}
	authorized, targetFound := false, false
	var values [5]string
	for rows.Next() {
		var id int64
		var role, status string
		var v [5]string
		if err = rows.Scan(&id, &role, &status, &v[0], &v[1], &v[2], &v[3], &v[4]); err != nil {
			rows.Close()
			return nil, err
		}
		if id == c.ActorID {
			authorized = status == service.StatusActive && (role == service.RoleAdmin || role == service.RoleOperator)
		}
		if id == c.TargetID {
			targetFound = role == service.RoleUser
			values = v
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if !authorized || !targetFound {
		return nil, service.ErrOperatorBalanceForbidden
	}
	rows, err = client.QueryContext(ctx, `SELECT notes FROM redeem_codes WHERE code=$1 AND type=$2`, c.OperationID, service.AdjustmentTypeOperatorBalance)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		var notes string
		err = rows.Scan(&notes)
		rows.Close()
		if err != nil {
			return nil, err
		}
		var receipt operatorBalanceReceipt
		if json.Unmarshal([]byte(notes), &receipt) != nil || receipt.Version != 1 {
			return nil, service.ErrOperatorBalanceUnavailable
		}
		if receipt.Fingerprint != c.Fingerprint || receipt.ActorID != c.ActorID || receipt.TargetID != c.TargetID {
			return nil, service.ErrOperatorBalanceConflict
		}
		result := receipt.Result
		result.Replayed = true
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return &result, nil
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var before [5]*big.Int
	var after [5]*big.Int
	for i, value := range values {
		before[i], err = operatorDecimal(value)
		if err != nil {
			return nil, service.ErrOperatorBalanceUnavailable
		}
		after[i] = new(big.Int).Set(before[i])
		if !operatorInRange(before[i]) {
			return nil, service.ErrOperatorBalanceUnavailable
		}
	}
	if before[3].Sign() < 0 || before[4].Sign() < 0 {
		return nil, service.ErrOperatorBalanceUnavailable
	}
	amount, err := service.ParseOperatorBalanceAmount(c.Request.Amount)
	if err != nil {
		return nil, err
	}
	free, paid := new(big.Int), new(big.Int)
	if c.Request.Operation == "add" {
		if c.Request.Source == "free" {
			free.Set(amount)
			after[3].Add(after[3], amount)
		} else {
			paid.Set(amount)
		}
		after[4].Add(after[4], amount)
	} else {
		if before[0].Cmp(amount) < 0 || before[2].Sign() < 0 {
			return nil, service.ErrBalanceNegative
		}
		free.Set(amount)
		if free.Cmp(before[1]) > 0 {
			free.Set(before[1])
			if free.Sign() < 0 {
				free.SetInt64(0)
			}
		}
		paid.Sub(amount, free)
		if paid.Cmp(before[2]) > 0 {
			return nil, service.ErrBalanceNegative
		}
		free.Neg(free)
		paid.Neg(paid)
	}
	after[1].Add(after[1], free)
	after[2].Add(after[2], paid)
	after[0].Add(after[0], free)
	after[0].Add(after[0], paid)
	for _, v := range after {
		if !operatorInRange(v) {
			return nil, service.ErrOperatorBalanceInvalid
		}
	}
	f := service.FormatOperatorBalanceUnits
	result := service.OperatorBalanceAdjustmentResult{OperationID: c.OperationID, Operation: c.Request.Operation, Amount: c.Request.Amount, Source: c.Request.Source, BeforeBalance: f(before[0]), AfterBalance: f(after[0]), BeforeFreeBalance: f(before[1]), AfterFreeBalance: f(after[1]), BeforePaidBalance: f(before[2]), AfterPaidBalance: f(after[2]), FreeAmount: f(new(big.Int).Abs(free)), PaidAmount: f(new(big.Int).Abs(paid))}
	updated, err := client.ExecContext(ctx, `UPDATE users SET balance=$1::numeric,free_balance=$2::numeric,paid_balance=$3::numeric,free_balance_issued=$4::numeric,total_recharged=$5::numeric,operator_balance_cache_version=operator_balance_cache_version+1,updated_at=NOW() WHERE id=$6 AND role=$7 AND deleted_at IS NULL`, f(after[0]), f(after[1]), f(after[2]), f(after[3]), f(after[4]), c.TargetID, service.RoleUser)
	if err != nil {
		return nil, err
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrOperatorBalanceUnavailable
	}
	deltas := []*big.Int{free, paid}
	sources := []string{"free", "paid"}
	inserted := 0
	for i, delta := range deltas {
		if delta.Sign() == 0 {
			continue
		}
		code := c.OperationID
		if inserted > 0 {
			code = c.OperationID + "p"
		}
		receipt := operatorBalanceReceipt{Version: 1, ActorID: c.ActorID, TargetID: c.TargetID, Fingerprint: c.Fingerprint, Reason: c.Request.Reason, Source: sources[i], Delta: f(delta), Result: result}
		notes, marshalErr := json.Marshal(receipt)
		if marshalErr != nil {
			return nil, marshalErr
		}
		_, err = client.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,balance_source,status,used_by,used_at,notes,created_at,validity_days,expires_at) VALUES($1,$2,$3::numeric,$4,$5,$6,NOW(),$7,NOW(),0,NULL)`, code, service.AdjustmentTypeOperatorBalance, f(delta), sources[i], service.StatusUsed, c.TargetID, string(notes))
		if err != nil {
			return nil, err
		}
		inserted++
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}
