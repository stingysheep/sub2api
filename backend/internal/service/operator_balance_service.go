package service

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrOperatorBalanceInvalid     = infraerrors.BadRequest("OPERATOR_BALANCE_INVALID", "Invalid balance adjustment")
	ErrOperatorBalanceForbidden   = infraerrors.Forbidden("OPERATOR_BALANCE_FORBIDDEN", "Balance adjustment is not permitted")
	ErrOperatorBalanceConflict    = infraerrors.Conflict("OPERATOR_BALANCE_CONFLICT", "Idempotency key belongs to a different adjustment")
	ErrOperatorBalanceUnavailable = infraerrors.ServiceUnavailable("OPERATOR_BALANCE_UNAVAILABLE", "Balance adjustment is unavailable")
)

type OperatorBalanceAdjustmentRequest struct {
	Operation      string `json:"operation"`
	Amount         string `json:"amount"`
	Source         string `json:"source"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"-"`
}

type OperatorBalanceAdjustmentResult struct {
	OperationID       string `json:"operation_id"`
	Operation         string `json:"operation"`
	Amount            string `json:"amount"`
	Source            string `json:"source"`
	BeforeBalance     string `json:"before_balance"`
	AfterBalance      string `json:"after_balance"`
	BeforeFreeBalance string `json:"before_free_balance"`
	AfterFreeBalance  string `json:"after_free_balance"`
	BeforePaidBalance string `json:"before_paid_balance"`
	AfterPaidBalance  string `json:"after_paid_balance"`
	FreeAmount        string `json:"free_amount"`
	PaidAmount        string `json:"paid_amount"`
	Replayed          bool   `json:"replayed"`
	CacheSynced       bool   `json:"cache_synced"`
}

type OperatorBalanceCommand struct {
	ActorID, TargetID        int64
	Request                  OperatorBalanceAdjustmentRequest
	OperationID, Fingerprint string
}
type OperatorBalanceRepository interface {
	AdjustOperatorBalance(context.Context, OperatorBalanceCommand) (*OperatorBalanceAdjustmentResult, error)
}
type OperatorBalanceService struct {
	repo         OperatorBalanceRepository
	billingCache *BillingCacheService
	authCache    APIKeyAuthCacheInvalidator
}

func NewOperatorBalanceService(repo OperatorBalanceRepository, billingCache *BillingCacheService, authCache APIKeyAuthCacheInvalidator) *OperatorBalanceService {
	return &OperatorBalanceService{repo: repo, billingCache: billingCache, authCache: authCache}
}

// ParseOperatorBalanceAmount returns exact decimal(20,8) units. No float conversion occurs.
var operatorAmountPattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,8})?$`)

func ParseOperatorBalanceAmount(amount string) (*big.Int, error) {
	if !operatorAmountPattern.MatchString(amount) {
		return nil, ErrOperatorBalanceInvalid
	}
	parts := strings.SplitN(amount, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	value, ok := new(big.Int).SetString(parts[0]+fraction+strings.Repeat("0", 8-len(fraction)), 10)
	if !ok || value.Sign() <= 0 {
		return nil, ErrOperatorBalanceInvalid
	}
	return value, nil
}
func FormatOperatorBalanceUnits(units *big.Int) string {
	negative := units.Sign() < 0
	digits := new(big.Int).Abs(units).String()
	if len(digits) < 9 {
		digits = strings.Repeat("0", 9-len(digits)) + digits
	}
	result := digits[:len(digits)-8] + "." + digits[len(digits)-8:]
	if negative {
		result = "-" + result
	}
	return result
}

func (s *OperatorBalanceService) Adjust(ctx context.Context, actorID, targetID int64, req OperatorBalanceAdjustmentRequest) (*OperatorBalanceAdjustmentResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOperatorBalanceUnavailable
	}
	if actorID <= 0 || targetID <= 0 || (req.Operation != "add" && req.Operation != "subtract") {
		return nil, ErrOperatorBalanceInvalid
	}
	units, err := ParseOperatorBalanceAmount(req.Amount)
	if err != nil {
		return nil, err
	}
	req.Amount = FormatOperatorBalanceUnits(units)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || len(req.Reason) > 1000 || len(req.IdempotencyKey) < 8 || len(req.IdempotencyKey) > 128 || strings.TrimSpace(req.IdempotencyKey) != req.IdempotencyKey {
		return nil, ErrOperatorBalanceInvalid
	}
	if req.Operation == "add" {
		if req.Source == "" {
			req.Source = "free"
		}
		if req.Source != "free" && req.Source != "paid" {
			return nil, ErrOperatorBalanceInvalid
		}
	} else if req.Source != "" {
		return nil, ErrOperatorBalanceInvalid
	}
	namespace, _ := json.Marshal([]any{"operator_balance/v1", actorID, req.IdempotencyKey})
	keyHash := sha256.Sum256(namespace)
	operationID := "op_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(keyHash[:])[:28])
	payload, _ := json.Marshal([]any{1, actorID, targetID, req.Operation, req.Amount, req.Source, req.Reason})
	fingerprint := sha256.Sum256(payload)
	result, err := s.repo.AdjustOperatorBalance(ctx, OperatorBalanceCommand{ActorID: actorID, TargetID: targetID, Request: req, OperationID: operationID, Fingerprint: hex.EncodeToString(fingerprint[:])})
	if err != nil {
		return nil, err
	}
	// Commit has already happened. Cache errors must never suggest the operation rolled back.
	cacheCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result.CacheSynced = true
	if s.billingCache != nil {
		s.billingCache.MarkOperatorBalanceDirty(targetID)
		if err := s.billingCache.InvalidateUserBalance(cacheCtx, targetID); err != nil {
			result.CacheSynced = false
		}
	}
	if s.authCache != nil {
		s.authCache.InvalidateAuthCacheByUserID(cacheCtx, targetID)
		// CacheSynced acknowledges balance invalidation only; auth invalidation has no acknowledgement.
	}
	return result, nil
}
