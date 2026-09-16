//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotRPM_KnownAbsenceSurvivesRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &rpmOverrideRepoStub{}
	auth := &APIKeyService{userGroupRateRepo: repo}
	groupID := int64(10)
	snapshot := auth.snapshotFromAPIKey(ctx, &APIKey{
		ID: 1, UserID: 2, GroupID: &groupID,
		User: &User{ID: 2, RPMLimit: 100}, Group: &Group{ID: groupID, RPMLimit: 5},
	})
	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var restored APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(payload, &restored))
	key := auth.snapshotToAPIKey("", &restored)
	cache := &userRPMCacheStub{userGroupCounts: []int{5, 6}}
	svc := newBillingServiceForRPM(t, cache, repo)
	require.NoError(t, svc.checkRPM(ctx, key.User, key.Group))
	require.ErrorIs(t, svc.checkRPM(ctx, key.User, key.Group), ErrGroupRPMExceeded)
	require.EqualValues(t, 1, repo.calls, "known absence should only query during snapshot construction")
}

func TestAPIKeyAuthSnapshotRPM_LegacyAndLookupFailureStillQuery(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "lookup_failure"}[failure], func(t *testing.T) {
			ctx := context.Background()
			repo := &rpmOverrideRepoStub{}
			auth := &APIKeyService{userGroupRateRepo: repo}
			var snapshot APIKeyAuthSnapshot
			if failure {
				repo.err = errors.New("lookup failed")
				groupID := int64(10)
				snapshot = *auth.snapshotFromAPIKey(ctx, &APIKey{
					ID: 1, UserID: 2, GroupID: &groupID,
					User: &User{ID: 2}, Group: &Group{ID: groupID, RPMLimit: 5},
				})
				repo.err = nil
				repo.calls = 0
			} else {
				require.NoError(t, json.Unmarshal([]byte(`{"user":{"id":2},"group":{"id":10,"rpm_limit":5}}`), &snapshot))
			}
			limit := 1
			repo.override = &limit
			key := auth.snapshotToAPIKey("", &snapshot)
			svc := newBillingServiceForRPM(t, &userRPMCacheStub{userGroupCounts: []int{2}}, repo)
			require.ErrorIs(t, svc.checkRPM(ctx, key.User, key.Group), ErrGroupRPMExceeded)
			require.EqualValues(t, 1, repo.calls)
		})
	}
}

func TestAPIKeyAuthSnapshotRPM_OverridesPreserveZeroAndUserCap(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, override := range []int{0, 2} {
			name := "current"
			if legacy {
				name = "legacy"
			}
			t.Run(name+map[int]string{0: "_zero", 2: "_positive"}[override], func(t *testing.T) {
				ctx := context.Background()
				repo := &rpmOverrideRepoStub{override: &override}
				auth := &APIKeyService{userGroupRateRepo: repo}
				groupID := int64(10)
				snapshot := auth.snapshotFromAPIKey(ctx, &APIKey{
					ID: 1, UserID: 2, GroupID: &groupID,
					User: &User{ID: 2, RPMLimit: 1}, Group: &Group{ID: groupID, RPMLimit: 1},
				})
				payload, err := json.Marshal(snapshot)
				require.NoError(t, err)
				if legacy {
					var fields map[string]any
					require.NoError(t, json.Unmarshal(payload, &fields))
					delete(fields["user"].(map[string]any), "user_group_rpm_override_loaded")
					payload, err = json.Marshal(fields)
					require.NoError(t, err)
				}
				var restored APIKeyAuthSnapshot
				require.NoError(t, json.Unmarshal(payload, &restored))
				key, used, err := auth.applyAuthCacheEntry("", &APIKeyAuthCacheEntry{Snapshot: &restored})
				require.NoError(t, err)
				require.True(t, used)
				repo.calls = 0
				cache := &userRPMCacheStub{userGroupCounts: []int{2}, userCounts: []int{2}}
				svc := newBillingServiceForRPM(t, cache, repo)
				require.ErrorIs(t, svc.checkRPM(ctx, key.User, key.Group), ErrUserRPMExceeded)
				require.Zero(t, repo.calls)
				if override == 0 {
					require.Zero(t, cache.userGroupCalls)
				} else {
					require.EqualValues(t, 1, cache.userGroupCalls)
				}
			})
		}
	}
}

func TestAPIKeyAuthSnapshotRPM_ChangedGroupRequiresLookup(t *testing.T) {
	ctx := context.Background()
	repo := &rpmOverrideRepoStub{}
	auth := &APIKeyService{userGroupRateRepo: repo}
	groupID := int64(10)
	snapshot := auth.snapshotFromAPIKey(ctx, &APIKey{
		ID: 1, UserID: 2, GroupID: &groupID, User: &User{ID: 2},
	})
	key := auth.snapshotToAPIKey("", snapshot)
	limit := 1
	repo.override = &limit
	repo.calls = 0
	svc := newBillingServiceForRPM(t, &userRPMCacheStub{userGroupCounts: []int{2}}, repo)
	require.ErrorIs(t, svc.checkRPM(ctx, key.User, &Group{ID: 20, RPMLimit: 5}), ErrGroupRPMExceeded)
	require.EqualValues(t, 1, repo.calls)
}
