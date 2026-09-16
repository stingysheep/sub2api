package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// GroupConcurrencyKey is a credential-free projection for dashboard sampling.
type GroupConcurrencyKey struct {
	APIKeyID  int64
	UserID    int64
	UserLabel string
	GroupID   int64
	GroupName string
	Platform  string
}

type DashboardGroupConcurrencyUser struct {
	UserID       int64  `json:"user_id"`
	UserLabel    string `json:"user_label"`
	CurrentInUse int    `json:"current_in_use"`
}

type groupConcurrencyKeyRepository interface {
	ListGroupConcurrencyKeys(ctx context.Context, afterID int64, limit int) ([]GroupConcurrencyKey, error)
}

type DashboardGroupConcurrency struct {
	GroupID      int64                           `json:"group_id"`
	GroupName    string                          `json:"group_name"`
	Platform     string                          `json:"platform"`
	CurrentInUse int                             `json:"current_in_use"`
	ActiveUsers  int                             `json:"active_users"`
	Users        []DashboardGroupConcurrencyUser `json:"users"`
}

type DashboardGroupConcurrencySnapshot struct {
	Groups                  []DashboardGroupConcurrency     `json:"groups"`
	Users                   []DashboardGroupConcurrencyUser `json:"users"`
	Timestamp               time.Time                       `json:"timestamp"`
	CurrentConcurrency      int                             `json:"current_concurrency"`
	ActiveUsers             int                             `json:"active_users"`
	GroupAttributedSlots    int                             `json:"group_attributed_slots"`
	UnattributedConcurrency int                             `json:"unattributed_concurrency"`
}

const dashboardGroupConcurrencySampleTTL = time.Second

type dashboardGroupConcurrencySampler struct {
	mu        sync.Mutex
	snapshot  *DashboardGroupConcurrencySnapshot
	expiresAt time.Time
	inflight  *dashboardGroupConcurrencyCall
}

type dashboardGroupConcurrencyCall struct {
	done     chan struct{}
	snapshot *DashboardGroupConcurrencySnapshot
	err      error
}

func cloneDashboardGroupConcurrencySnapshot(snapshot *DashboardGroupConcurrencySnapshot) *DashboardGroupConcurrencySnapshot {
	if snapshot == nil {
		return nil
	}
	result := *snapshot
	result.Groups = make([]DashboardGroupConcurrency, len(snapshot.Groups))
	copy(result.Groups, snapshot.Groups)
	result.Users = make([]DashboardGroupConcurrencyUser, len(snapshot.Users))
	copy(result.Users, snapshot.Users)
	for i := range result.Groups {
		result.Groups[i].Users = make([]DashboardGroupConcurrencyUser, len(snapshot.Groups[i].Users))
		copy(result.Groups[i].Users, snapshot.Groups[i].Users)
	}
	return &result
}

// GetDashboardGroupConcurrency reports authoritative ordinary-user concurrency.
// API key slots are sampled separately to provide best-effort current group and
// user attribution; the response exposes any gap instead of presenting it as an
// exact partition. Each service instance shares one immutable short-lived sample.
func (s *APIKeyService) GetDashboardGroupConcurrency(ctx context.Context) (*DashboardGroupConcurrencySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.concurrencyService == nil {
		return nil, errors.New("group concurrency service unavailable")
	}
	repo, ok := s.apiKeyRepo.(groupConcurrencyKeyRepository)
	if !ok {
		return nil, errors.New("group concurrency projection unavailable")
	}
	baseCache := s.concurrencyService.cache
	cache, ok := baseCache.(APIKeyConcurrencyCache)
	if !ok {
		return nil, errors.New("group concurrency cache unavailable")
	}
	sampler := &s.dashboardGroupConcurrency
	sampler.mu.Lock()
	if sampler.snapshot != nil && time.Now().Before(sampler.expiresAt) {
		snapshot := sampler.snapshot
		sampler.mu.Unlock()
		return cloneDashboardGroupConcurrencySnapshot(snapshot), nil
	}
	call := sampler.inflight
	if call == nil {
		call = &dashboardGroupConcurrencyCall{done: make(chan struct{})}
		sampler.inflight = call
		go func() {
			// Neither cancellation nor user/transaction context values may leak
			// from an individual HTTP request into this global shared sample.
			sampleCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			snapshot, err := sampleDashboardGroupConcurrency(sampleCtx, repo, baseCache, cache)
			sampler.mu.Lock()
			if err == nil {
				sampler.snapshot = snapshot
				sampler.expiresAt = time.Now().Add(dashboardGroupConcurrencySampleTTL)
			}
			call.snapshot, call.err = snapshot, err
			sampler.inflight = nil
			close(call.done)
			sampler.mu.Unlock()
		}()
	}
	sampler.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-call.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return cloneDashboardGroupConcurrencySnapshot(call.snapshot), call.err
	}
}

func sampleDashboardGroupConcurrency(ctx context.Context, repo groupConcurrencyKeyRepository, baseCache ConcurrencyCache, cache APIKeyConcurrencyCache) (*DashboardGroupConcurrencySnapshot, error) {
	const batchSize = 500
	groups := make(map[int64]*DashboardGroupConcurrency)
	groupUsers := make(map[int64]map[int64]*DashboardGroupConcurrencyUser)
	users := make(map[int64]GroupConcurrencyKey)
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
			if key.UserID > 0 {
				users[key.UserID] = key
			}
			group := groups[key.GroupID]
			if group == nil {
				group = &DashboardGroupConcurrency{GroupID: key.GroupID, GroupName: key.GroupName, Platform: key.Platform, Users: []DashboardGroupConcurrencyUser{}}
				groups[key.GroupID] = group
				groupUsers[key.GroupID] = make(map[int64]*DashboardGroupConcurrencyUser)
			}
			if count := counts[key.APIKeyID]; count > 0 {
				group.CurrentInUse += count
				if key.UserID > 0 {
					user := groupUsers[key.GroupID][key.UserID]
					if user == nil {
						user = &DashboardGroupConcurrencyUser{UserID: key.UserID, UserLabel: key.UserLabel}
						groupUsers[key.GroupID][key.UserID] = user
					}
					user.CurrentInUse += count
				}
			}
		}
		if len(keys) < batchSize {
			break
		}
	}
	userBatch := make([]UserWithConcurrency, 0, len(users))
	for userID := range users {
		userBatch = append(userBatch, UserWithConcurrency{ID: userID})
	}
	sort.Slice(userBatch, func(i, j int) bool { return userBatch[i].ID < userBatch[j].ID })
	loads, err := baseCache.GetUsersLoadBatch(ctx, userBatch)
	if err != nil {
		return nil, err
	}
	result := &DashboardGroupConcurrencySnapshot{
		Groups:    make([]DashboardGroupConcurrency, 0, len(groups)),
		Users:     make([]DashboardGroupConcurrencyUser, 0, len(loads)),
		Timestamp: time.Now().UTC(),
	}
	for userID, load := range loads {
		if load != nil && load.CurrentConcurrency > 0 {
			result.ActiveUsers++
			result.CurrentConcurrency += load.CurrentConcurrency
			result.Users = append(result.Users, DashboardGroupConcurrencyUser{
				UserID:       userID,
				UserLabel:    users[userID].UserLabel,
				CurrentInUse: load.CurrentConcurrency,
			})
		}
	}
	sort.Slice(result.Users, func(i, j int) bool {
		if result.Users[i].CurrentInUse != result.Users[j].CurrentInUse {
			return result.Users[i].CurrentInUse > result.Users[j].CurrentInUse
		}
		return result.Users[i].UserID < result.Users[j].UserID
	})
	for _, group := range groups {
		for _, user := range groupUsers[group.GroupID] {
			group.Users = append(group.Users, *user)
		}
		sort.Slice(group.Users, func(i, j int) bool {
			if group.Users[i].CurrentInUse != group.Users[j].CurrentInUse {
				return group.Users[i].CurrentInUse > group.Users[j].CurrentInUse
			}
			return group.Users[i].UserID < group.Users[j].UserID
		})
		group.ActiveUsers = len(group.Users)
		result.GroupAttributedSlots += group.CurrentInUse
		result.Groups = append(result.Groups, *group)
	}
	if result.CurrentConcurrency > result.GroupAttributedSlots {
		result.UnattributedConcurrency = result.CurrentConcurrency - result.GroupAttributedSlots
	}
	sort.Slice(result.Groups, func(i, j int) bool {
		if result.Groups[i].CurrentInUse != result.Groups[j].CurrentInUse {
			return result.Groups[i].CurrentInUse > result.Groups[j].CurrentInUse
		}
		return result.Groups[i].GroupID < result.Groups[j].GroupID
	})
	return result, nil
}
