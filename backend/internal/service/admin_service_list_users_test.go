//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type userRepoStubForListUsers struct {
	userRepoStub
	users                 []User
	err                   error
	listWithFiltersParams pagination.PaginationParams
	lastUsedByUserID      map[int64]*time.Time
	lastUsedErr           error
}

func (s *userRepoStubForListUsers) ListWithFilters(_ context.Context, params pagination.PaginationParams, _ UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listWithFiltersParams = params
	if s.err != nil {
		return nil, nil, s.err
	}
	out := make([]User, len(s.users))
	copy(out, s.users)
	return out, &pagination.PaginationResult{
		Total:    int64(len(out)),
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserIDs(_ context.Context, userIDs []int64) (map[int64]*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	result := make(map[int64]*time.Time, len(userIDs))
	for _, userID := range userIDs {
		if ts, ok := s.lastUsedByUserID[userID]; ok {
			result[userID] = ts
		}
	}
	return result, nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserID(_ context.Context, userID int64) (*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	return s.lastUsedByUserID[userID], nil
}

type userGroupRateRepoStubForListUsers struct {
	batchCalls int
	singleCall []int64

	batchErr  error
	batchData map[int64]map[int64]float64

	singleErr  map[int64]error
	singleData map[int64]map[int64]float64
}

func (s *userGroupRateRepoStubForListUsers) GetByUserIDs(_ context.Context, _ []int64) (map[int64]map[int64]float64, error) {
	s.batchCalls++
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	return s.batchData, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserID(_ context.Context, userID int64) (map[int64]float64, error) {
	s.singleCall = append(s.singleCall, userID)
	if err, ok := s.singleErr[userID]; ok {
		return nil, err
	}
	if rates, ok := s.singleData[userID]; ok {
		return rates, nil
	}
	return map[int64]float64{}, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserAndGroup(_ context.Context, userID, groupID int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) GetRPMOverrideByUserAndGroup(_ context.Context, _, _ int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	panic("unexpected SyncUserGroupRates call")
}

func (s *userGroupRateRepoStubForListUsers) GetByGroupID(_ context.Context, _ int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRateMultipliers(_ context.Context, _ int64, _ []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRPMOverrides(_ context.Context, _ int64, _ []GroupRPMOverrideInput) error {
	panic("unexpected SyncGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) ClearGroupRPMOverrides(_ context.Context, _ int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByGroupID(_ context.Context, _ int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByUserID(_ context.Context, userID int64) error {
	panic("unexpected DeleteByUserID call")
}

func TestAdminService_ListUsers_BatchRateFallbackToSingle(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 101, Username: "u1"},
			{ID: 202, Username: "u2"},
		},
	}
	rateRepo := &userGroupRateRepoStubForListUsers{
		batchErr: errors.New("batch unavailable"),
		singleData: map[int64]map[int64]float64{
			101: {11: 1.1},
			202: {22: 2.2},
		},
	}
	svc := &adminServiceImpl{
		userRepo:          userRepo,
		userGroupRateRepo: rateRepo,
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, users, 2)
	require.Equal(t, 1, rateRepo.batchCalls)
	require.ElementsMatch(t, []int64{101, 202}, rateRepo.singleCall)
	require.Equal(t, 1.1, users[0].GroupRates[11])
	require.Equal(t, 2.2, users[1].GroupRates[22])
}

func TestAdminService_ListUsers_BatchRateFailureStopsOnSingleFailure(t *testing.T) {
	users := make([]User, 1000)
	for i := range users {
		users[i].ID = int64(i + 1)
	}
	repo := &userGroupRateRepoStubForListUsers{
		batchErr:  errors.New("database unavailable"),
		singleErr: map[int64]error{1: errors.New("database unavailable")},
	}
	svc := &adminServiceImpl{userRepo: &userRepoStubForListUsers{users: users}, userGroupRateRepo: repo}
	got, _, err := svc.ListUsers(context.Background(), 1, 1000, UserListFilters{}, "", "")
	require.ErrorContains(t, err, "user group rates unavailable")
	require.Nil(t, got, "incomplete rates must not look like absent overrides")
	require.Equal(t, 1, repo.batchCalls)
	require.Equal(t, []int64{1}, repo.singleCall, "outage must cost only one fallback query, not N")
}

func TestAdminService_ListUsers_BatchRateFailureHasQueryBudget(t *testing.T) {
	users := make([]User, 1000)
	for i := range users {
		users[i].ID = int64(i + 1)
	}
	repo := &userGroupRateRepoStubForListUsers{batchErr: errors.New("batch unavailable")}
	svc := &adminServiceImpl{userRepo: &userRepoStubForListUsers{users: users}, userGroupRateRepo: repo}
	got, _, err := svc.ListUsers(context.Background(), 1, 1000, UserListFilters{}, "", "")
	require.ErrorContains(t, err, "budget exceeded")
	require.Nil(t, got)
	require.Len(t, repo.singleCall, 20, "successful fallback reads must also be bounded")
}

func TestAdminService_ListUsers_BatchRateFallbackHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &userGroupRateRepoStubForListUsers{batchErr: context.Canceled}
	svc := &adminServiceImpl{userRepo: &userRepoStubForListUsers{users: []User{{ID: 1}}}, userGroupRateRepo: repo}
	_, _, err := svc.ListUsers(ctx, 1, 20, UserListFilters{}, "", "")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, repo.singleCall)
}

// Wrapper intentionally omits the optional batch interface to cover legacy repositories.
type userGroupRateSingleReaderForListUsers struct {
	UserGroupRateRepository
	stub *userGroupRateRepoStubForListUsers
}

func (r *userGroupRateSingleReaderForListUsers) GetByUserID(ctx context.Context, id int64) (map[int64]float64, error) {
	return r.stub.GetByUserID(ctx, id)
}

func TestAdminService_ListUsers_LegacySingleReaderKeepsCompatibility(t *testing.T) {
	users := make([]User, 30)
	for i := range users {
		users[i].ID = int64(i + 1)
	}
	repo := &userGroupRateRepoStubForListUsers{
		singleErr:  map[int64]error{1: errors.New("one user unavailable")},
		singleData: map[int64]map[int64]float64{30: {11: 1.5}},
	}
	svc := &adminServiceImpl{
		userRepo:          &userRepoStubForListUsers{users: users},
		userGroupRateRepo: &userGroupRateSingleReaderForListUsers{UserGroupRateRepository: repo, stub: repo},
	}
	got, total, err := svc.ListUsers(context.Background(), 1, 30, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(30), total)
	require.Len(t, repo.singleCall, 30)
	require.Equal(t, 1.5, got[29].GroupRates[11])
	require.Zero(t, repo.batchCalls)
}

func TestAdminService_ListUsers_PassesSortParams(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 1, Email: "a@example.com"}},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	_, _, err := svc.ListUsers(context.Background(), 2, 50, UserListFilters{}, "email", "ASC")
	require.NoError(t, err)
	require.Equal(t, pagination.PaginationParams{
		Page:      2,
		PageSize:  50,
		SortBy:    "email",
		SortOrder: "ASC",
	}, userRepo.listWithFiltersParams)
}

func TestAdminService_ListUsers_PopulatesLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 101, Email: "u@example.com"}},
		lastUsedByUserID: map[int64]*time.Time{
			101: &lastUsed,
		},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].LastUsedAt)
	require.WithinDuration(t, lastUsed, *users[0].LastUsedAt, time.Second)
}
