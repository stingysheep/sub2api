package service

import (
	"context"
	"errors"
	"sort"
	"time"
)

// GroupConcurrencyKey is a credential-free projection for dashboard sampling.
type GroupConcurrencyKey struct {
	APIKeyID  int64
	GroupID   int64
	GroupName string
	Platform  string
}

type groupConcurrencyKeyRepository interface {
	ListGroupConcurrencyKeys(ctx context.Context, afterID int64, limit int) ([]GroupConcurrencyKey, error)
}

type DashboardGroupConcurrency struct {
	GroupID      int64  `json:"group_id"`
	GroupName    string `json:"group_name"`
	Platform     string `json:"platform"`
	CurrentInUse int    `json:"current_in_use"`
}

type DashboardGroupConcurrencySnapshot struct {
	Groups    []DashboardGroupConcurrency `json:"groups"`
	Timestamp time.Time                   `json:"timestamp"`
}

// GetDashboardGroupConcurrency samples API key request/session slots. A shared
// upstream account does not duplicate traffic into all of its pricing groups.
// The current key binding determines attribution (not historical usage logs).
func (s *APIKeyService) GetDashboardGroupConcurrency(ctx context.Context) (*DashboardGroupConcurrencySnapshot, error) {
	if s == nil || s.concurrencyService == nil {
		return nil, errors.New("group concurrency service unavailable")
	}
	repo, ok := s.apiKeyRepo.(groupConcurrencyKeyRepository)
	if !ok {
		return nil, errors.New("group concurrency projection unavailable")
	}
	cache, ok := s.concurrencyService.cache.(APIKeyConcurrencyCache)
	if !ok {
		return nil, errors.New("group concurrency cache unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	const batchSize = 500
	groups := make(map[int64]*DashboardGroupConcurrency)
	var afterID int64
	for {
		keys, err := repo.ListGroupConcurrencyKeys(ctx, afterID, batchSize)
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			break
		}
		ids := make([]int64, len(keys))
		for i, key := range keys {
			// Bound keyset progress even if an alternate repository violates the contract.
			if key.APIKeyID <= afterID {
				return nil, errors.New("invalid concurrency projection ordering")
			}
			ids[i] = key.APIKeyID
			afterID = key.APIKeyID
		}
		// Propagate Redis errors: unknown usage must not be presented as idle.
		counts, err := cache.GetAPIKeyConcurrencyBatch(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			group := groups[key.GroupID]
			if group == nil {
				group = &DashboardGroupConcurrency{GroupID: key.GroupID, GroupName: key.GroupName, Platform: key.Platform}
				groups[key.GroupID] = group
			}
			if count := counts[key.APIKeyID]; count > 0 {
				group.CurrentInUse += count
			}
		}
		if len(keys) < batchSize {
			break
		}
	}
	result := &DashboardGroupConcurrencySnapshot{Groups: make([]DashboardGroupConcurrency, 0, len(groups)), Timestamp: time.Now().UTC()}
	for _, group := range groups {
		result.Groups = append(result.Groups, *group)
	}
	sort.Slice(result.Groups, func(i, j int) bool {
		if result.Groups[i].CurrentInUse != result.Groups[j].CurrentInUse {
			return result.Groups[i].CurrentInUse > result.Groups[j].CurrentInUse
		}
		return result.Groups[i].GroupID < result.Groups[j].GroupID
	})
	return result, nil
}
