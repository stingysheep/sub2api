package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "modernc.org/sqlite"
)

const recoveryMaxPayloadBytes = 32 * 1024

type usageRecoveryPayload struct {
	Command service.UsageBillingCommand
	Usage   json.RawMessage
}

type usageRecoveryEntry struct {
	id       int64
	payload  []byte
	checksum []byte
	attempts int
}

// usageBillingRecoveryRepository journals only the RecordUsage→billing/usage
// boundary. It does not claim to cover upstream success before RecordUsage,
// rebates/platform effects, or replay time-dependent quota commands.
type usageBillingRecoveryRepository struct {
	service.UsageBillingRepository
	state      usageBillingRecoveryState
	usage      service.UsageLogRepository
	journal    *sql.DB
	maxEntries int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

func ProvideUsageBillingRepository(client *dbent.Client, db *sql.DB, usage service.UsageLogRepository, cfg *config.Config) (service.UsageBillingRepository, error) {
	path, maxEntries, maxBytes := "./data/usage-billing-recovery.sqlite", 10000, int64(64*1024*1024)
	if cfg != nil {
		if cfg.Billing.RecoveryJournalPath != "" {
			path = cfg.Billing.RecoveryJournalPath
		}
		if cfg.Billing.RecoveryMaxEntries > 0 {
			maxEntries = cfg.Billing.RecoveryMaxEntries
		}
		if cfg.Billing.RecoveryMaxBytes > 0 {
			maxBytes = cfg.Billing.RecoveryMaxBytes
		}
	}
	r, err := openUsageBillingRecovery(NewUsageBillingRepository(client, db), usage, path, maxEntries, maxBytes)
	if err != nil {
		return nil, err
	}
	r.start()
	return r, nil
}

func openUsageBillingRecovery(base service.UsageBillingRepository, usage service.UsageLogRepository, path string, maxEntries int, maxBytes int64) (*usageBillingRecoveryRepository, error) {
	state, ok := base.(usageBillingRecoveryState)
	if !ok || usage == nil || maxEntries < 1 || maxBytes < 32768 || strings.ContainsAny(path, "?\x00") {
		return nil, errors.New("invalid usage recovery configuration")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	journal, err := sql.Open("sqlite", abs)
	if err != nil {
		return nil, err
	}
	journal.SetMaxOpenConns(1)
	journal.SetMaxIdleConns(1)
	ok = false
	defer func() {
		if !ok {
			_ = journal.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, statement := range []string{"PRAGMA busy_timeout=2000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA wal_autocheckpoint=32", "PRAGMA journal_size_limit=131072", fmt.Sprintf("PRAGMA max_page_count=%d", maxBytes/4096)} {
		if _, err := journal.ExecContext(ctx, statement); err != nil {
			return nil, err
		}
	}
	var pages, pageSize int64
	if err := journal.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return nil, err
	}
	if err := journal.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	if pageSize <= 0 || pages > maxBytes/pageSize {
		return nil, errors.New("existing usage recovery journal exceeds capacity")
	}
	if _, err := journal.ExecContext(ctx, fmt.Sprintf("PRAGMA max_page_count=%d", maxBytes/pageSize)); err != nil {
		return nil, err
	}
	var integrity string
	if err := journal.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		return nil, errors.New("usage recovery journal integrity check failed")
	}
	_, err = journal.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS billing_recovery(id INTEGER PRIMARY KEY,request_id TEXT NOT NULL,api_key_id INTEGER NOT NULL,fingerprint TEXT NOT NULL,payload BLOB NOT NULL,checksum BLOB NOT NULL,state TEXT NOT NULL DEFAULT 'pending',attempts INTEGER NOT NULL DEFAULT 0,next_at INTEGER NOT NULL,reason TEXT NOT NULL DEFAULT '',UNIQUE(request_id,api_key_id)); CREATE INDEX IF NOT EXISTS billing_recovery_due ON billing_recovery(state,next_at,id)`)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(abs, 0600); err != nil {
		return nil, err
	}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	ok = true
	return &usageBillingRecoveryRepository{UsageBillingRepository: base, state: state, usage: usage, journal: journal, maxEntries: maxEntries, ctx: workerCtx, cancel: workerCancel}, nil
}

// The explicit allowlist cannot acquire sensitive fields added to UsageLog in
// future. Recovery deliberately omits IP, UserAgent, session and raw credentials.
func recoveryUsageJSON(log *service.UsageLog) (json.RawMessage, error) {
	if log == nil {
		return nil, errors.New("usage recovery requires usage")
	}
	copy := *log
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = time.Now().UTC()
	}
	// Select before serialization: even the temporary JSON excludes attached
	// User/APIKey/Account objects and every credential-bearing nested value.
	value := reflect.ValueOf(copy)
	safe := make(map[string]any)
	for _, field := range []string{"UserID", "APIKeyID", "AccountID", "RequestID", "Model", "RequestedModel", "GroupID", "SubscriptionID", "InputTokens", "OutputTokens", "CacheCreationTokens", "CacheReadTokens", "CacheCreation5mTokens", "CacheCreation1hTokens", "ImageInputTokens", "ImageOutputTokens", "ImageInputCost", "ImageOutputCost", "InputCost", "OutputCost", "CacheCreationCost", "CacheReadCost", "TotalCost", "ActualCost", "RateMultiplier", "AccountRateMultiplier", "AccountStatsCost", "BillingType", "RequestType", "Stream", "OpenAIWSMode", "ImageCount", "MediaType", "CreatedAt", "UpstreamModel", "UpstreamResponseModel", "UpstreamModelMismatch", "ChannelID", "ModelMappingChain", "BillingTier", "BillingMode", "ServiceTier", "ReasoningEffort", "RequestedReasoningEffort", "ImageSize", "ImageInputSize", "ImageOutputSize", "ImageSizeSource", "ImageSizeBreakdown", "VideoCount", "VideoResolution", "VideoDurationSeconds", "CacheTTLOverridden", "LongContextBillingApplied", "NativeCompactionV2"} {
		selected := value.FieldByName(field)
		if !selected.IsValid() {
			return nil, errors.New("invalid recovery usage allowlist")
		}
		safe[field] = selected.Interface()
	}
	return json.Marshal(safe)
}

func (r *usageBillingRecoveryRepository) enqueue(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) error {
	_, err := r.enqueueStatus(ctx, cmd, log)
	return err
}

// enqueueStatus reports whether a matching pending intent already existed.
func (r *usageBillingRecoveryRepository) enqueueStatus(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) (bool, error) {
	if cmd == nil || len(cmd.RequestID) == 0 || len(cmd.RequestID) > 512 {
		return false, errors.New("invalid recovery command")
	}
	cmd.Normalize()
	copy := *cmd
	copy.RequestPayloadHash = "" // normalized fingerprint is sufficient; no request data persists
	usage, err := recoveryUsageJSON(log)
	if err != nil {
		return false, err
	}
	payload, err := json.Marshal(usageRecoveryPayload{Command: copy, Usage: usage})
	if err != nil || len(payload) > recoveryMaxPayloadBytes {
		return false, errors.New("recovery payload exceeds bound")
	}
	tx, err := r.journal.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var fingerprint, state string
	err = tx.QueryRowContext(ctx, "SELECT fingerprint,state FROM billing_recovery WHERE request_id=? AND api_key_id=?", cmd.RequestID, cmd.APIKeyID).Scan(&fingerprint, &state)
	if err == nil {
		if fingerprint != cmd.RequestFingerprint {
			return false, service.ErrUsageBillingRequestConflict
		}
		switch state {
		case "pending":
			return true, tx.Commit()
		case "quarantine":
			return false, errors.New("usage recovery intent is quarantined")
		default:
			return false, errors.New("invalid usage recovery state")
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM billing_recovery").Scan(&count); err != nil {
		return false, err
	}
	if count >= r.maxEntries {
		return false, errors.New("usage recovery capacity exhausted")
	}
	checksum := sha256.Sum256(payload)
	_, err = tx.ExecContext(ctx, "INSERT INTO billing_recovery(request_id,api_key_id,fingerprint,payload,checksum,next_at) VALUES(?,?,?,?,?,?)", cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint, payload, checksum[:], time.Now().Add(30*time.Second).Unix())
	if err != nil {
		return false, err
	}
	return false, tx.Commit()
}

func recoveryRequired(err error) error {
	return fmt.Errorf("%w: %w", service.ErrUsageBillingRecoveryRequired, err)
}

func (r *usageBillingRecoveryRepository) ApplyWithUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) (*service.UsageBillingApplyResult, error) {
	existingPending, err := r.enqueueStatus(ctx, cmd, log)
	if err != nil {
		return nil, recoveryRequired(err)
	}
	var result *service.UsageBillingApplyResult
	alreadyCommitted := false
	if existingPending {
		result, err = r.state.CommittedUsageBilling(ctx, cmd)
		if err != nil {
			return nil, recoveryRequired(err)
		}
		alreadyCommitted = result != nil
		if !alreadyCommitted && (cmd.SubscriptionCost != 0 || cmd.APIKeyRateLimitCost != 0 || cmd.AccountQuotaCost != 0) {
			_, err = r.journal.ExecContext(ctx, "UPDATE billing_recovery SET state='quarantine',reason='time_dependent_command' WHERE request_id=? AND api_key_id=? AND state='pending'", cmd.RequestID, cmd.APIKeyID)
			if err == nil {
				err = errors.New("time-dependent usage recovery requires review")
			}
			return nil, recoveryRequired(err)
		}
	}
	if !alreadyCommitted {
		result, err = r.UsageBillingRepository.Apply(ctx, cmd)
		if err == nil && (result == nil || !result.Applied) {
			result, err = r.state.CommittedUsageBilling(ctx, cmd)
			if err == nil && result == nil {
				err = errors.New("billing commit was not confirmed")
			}
			alreadyCommitted = err == nil
		}
	}
	if err == nil {
		if alreadyCommitted && cmd.BalanceCost > 0 {
			err = r.state.FenceRecoveredBalance(ctx, cmd.UserID)
		}
		if err == nil {
			err = r.persistUsage(ctx, cmd, log, result)
		}
	}
	if err != nil {
		// SQLite errors leave the intent intact; a finite worker retry/backoff
		// policy handles recovery without any request/credential error logging.
		ackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = r.journal.ExecContext(ackCtx, "UPDATE billing_recovery SET next_at=? WHERE request_id=? AND api_key_id=?", time.Now().Add(5*time.Second).Unix(), cmd.RequestID, cmd.APIKeyID)
		cancel()
		return nil, recoveryRequired(err)
	}
	return result, nil
}

func (r *usageBillingRecoveryRepository) persistUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog, result *service.UsageBillingApplyResult) error {
	if result == nil || log == nil {
		return errors.New("invalid recovery commit")
	}
	log.FreeBalanceCost, log.PaidBalanceCost, log.UnfundedBalanceCost = result.FreeBalanceCost, result.PaidBalanceCost, result.UnfundedBalanceCost
	// Create returns only after PostgreSQL persistence (or a verified duplicate),
	// rather than an asynchronous enqueue. Intent survives every failed ACK.
	inserted, err := r.usage.Create(ctx, log)
	if err != nil {
		return err
	}
	if !inserted {
		if err := r.state.VerifyCommittedUsage(ctx, cmd, log); err != nil {
			if errors.Is(err, errUsageRecoveryConflict) {
				// A historical zero-cost row can conflict after a confirmed debit.
				// Revoke shared caches before quarantining that accounting conflict.
				if cmd.BalanceCost > 0 {
					if fenceErr := r.state.FenceRecoveredBalance(ctx, cmd.UserID); fenceErr != nil {
						return fenceErr
					}
				}
				_, _ = r.journal.ExecContext(ctx, "UPDATE billing_recovery SET state='quarantine',reason='usage_conflict' WHERE request_id=? AND api_key_id=?", cmd.RequestID, cmd.APIKeyID)
			}
			return err
		}
	}
	deleted, err := r.journal.ExecContext(ctx, "DELETE FROM billing_recovery WHERE request_id=? AND api_key_id=? AND fingerprint=? AND state='pending'", cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return err
	}
	count, err := deleted.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	if count != 0 {
		return errors.New("unexpected usage recovery acknowledgement count")
	}
	var state string
	err = r.journal.QueryRowContext(ctx, "SELECT state FROM billing_recovery WHERE request_id=? AND api_key_id=? AND fingerprint=?", cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return r.state.VerifyCommittedUsage(ctx, cmd, log)
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("usage recovery intent is no longer pending: %s", state)
}

func (r *usageBillingRecoveryRepository) start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
				if err := r.RecoverOnce(ctx); err != nil && r.ctx.Err() == nil {
					logger.LegacyPrintf("repository.usage_recovery", "usage recovery journal unavailable; pending intent retained")
				}
				cancel()
			}
		}
	}()
}

func (r *usageBillingRecoveryRepository) Close() {
	r.closeOnce.Do(func() { r.cancel(); r.wg.Wait(); _ = r.journal.Close() })
}

// RecoverOnce is bounded to sixteen intents. Quarantine is durable and never
// silently discarded; a full journal backpressures billing before PG mutation.
func (r *usageBillingRecoveryRepository) RecoverOnce(ctx context.Context) error {
	rows, err := r.journal.QueryContext(ctx, "SELECT id,payload,checksum,attempts FROM billing_recovery WHERE state='pending' AND next_at<=? ORDER BY id LIMIT 16", time.Now().Unix())
	if err != nil {
		return err
	}
	var entries []usageRecoveryEntry
	for rows.Next() {
		var entry usageRecoveryEntry
		if err := rows.Scan(&entry.id, &entry.payload, &entry.checksum, &entry.attempts); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		checksum := sha256.Sum256(entry.payload)
		if len(entry.checksum) != len(checksum) || string(entry.checksum) != string(checksum[:]) {
			if err := r.quarantine(ctx, entry.id, "payload_checksum"); err != nil {
				return err
			}
			continue
		}
		if entry.attempts >= 10 {
			if err := r.quarantine(ctx, entry.id, "retry_limit"); err != nil {
				return err
			}
			continue
		}
		backoff := min(5*time.Minute, max(30*time.Second, 5*time.Second*time.Duration(1<<min(entry.attempts, 6))))
		claim, err := r.journal.ExecContext(ctx, "UPDATE billing_recovery SET attempts=attempts+1,next_at=? WHERE id=? AND state='pending' AND attempts=? AND next_at<=?", time.Now().Add(backoff).Unix(), entry.id, entry.attempts, time.Now().Unix())
		if err != nil {
			return err
		}
		claimed, err := claim.RowsAffected()
		if err != nil {
			return err
		}
		if claimed != 1 {
			continue
		}
		var payload usageRecoveryPayload
		if err := json.Unmarshal(entry.payload, &payload); err != nil {
			if err := r.quarantine(ctx, entry.id, "invalid_payload"); err != nil {
				return err
			}
			continue
		}
		cmd := &payload.Command
		result, err := r.state.CommittedUsageBilling(ctx, cmd)
		if errors.Is(err, service.ErrUsageBillingRequestConflict) {
			if err := r.quarantine(ctx, entry.id, "fingerprint_conflict"); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			continue
		}
		alreadyCommitted := result != nil
		if !alreadyCommitted {
			if cmd.SubscriptionCost != 0 || cmd.APIKeyRateLimitCost != 0 || cmd.AccountQuotaCost != 0 {
				if err := r.quarantine(ctx, entry.id, "time_dependent_command"); err != nil {
					return err
				}
				continue
			}
			cmd.Recovery = true
			result, err = r.UsageBillingRepository.Apply(ctx, cmd)
			if err != nil {
				continue
			}
			if result == nil || !result.Applied {
				result, err = r.state.CommittedUsageBilling(ctx, cmd)
				alreadyCommitted = true
			}
		}
		if err != nil || result == nil {
			continue
		}
		if alreadyCommitted && cmd.BalanceCost > 0 {
			if err := r.state.FenceRecoveredBalance(ctx, cmd.UserID); err != nil {
				continue
			}
		}
		var usage service.UsageLog
		if err := json.Unmarshal(payload.Usage, &usage); err != nil {
			if err := r.quarantine(ctx, entry.id, "invalid_usage"); err != nil {
				return err
			}
			continue
		}
		_ = r.persistUsage(ctx, cmd, &usage, result)
	}
	return nil
}

func (r *usageBillingRecoveryRepository) quarantine(ctx context.Context, id int64, reason string) error {
	_, err := r.journal.ExecContext(ctx, "UPDATE billing_recovery SET state='quarantine',reason=? WHERE id=?", reason, id)
	return err
}
