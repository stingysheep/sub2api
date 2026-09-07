package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupCategoriesPreservesOrderAndUniqueAssignments(t *testing.T) {
	got, err := normalizeGroupCategories([]GroupCategory{
		{ID: 2, Name: "二", GroupIDs: []int64{20, 10}},
		{ID: 1, Name: "一", GroupIDs: []int64{10, 30}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), got[0].ID)
	require.Equal(t, []int64{20, 10}, got[0].GroupIDs)
	require.Equal(t, int64(1), got[1].ID)
	require.Equal(t, []int64{30}, got[1].GroupIDs)
}

func TestNormalizeGroupCategoriesValidatesNamesAndIDs(t *testing.T) {
	for _, input := range [][]GroupCategory{
		{{ID: 0, Name: "有效"}},
		{{ID: 1, Name: " "}},
		{{ID: 1, Name: strings.Repeat("分", 101)}},
		{{ID: 1, Name: "一"}, {ID: 1, Name: "二"}},
	} {
		_, err := normalizeGroupCategories(input)
		status, _ := infraerrors.ToHTTP(err)
		require.Equal(t, 400, status)
	}
}

type groupCategoriesStoreStub struct {
	SettingRepository
	raw      *string
	loseRace bool
}

func (r *groupCategoriesStoreStub) GetValue(context.Context, string) (string, error) {
	if r.raw == nil {
		return "", ErrSettingNotFound
	}
	return *r.raw, nil
}

func (r *groupCategoriesStoreStub) CompareAndSwap(_ context.Context, _ string, previous *string, next string) (bool, error) {
	if r.loseRace || (previous == nil) != (r.raw == nil) || (previous != nil && *previous != *r.raw) {
		return false, nil
	}
	r.raw = &next
	return true, nil
}

func TestSetGroupCategoriesRejectsStaleEditor(t *testing.T) {
	original := []GroupCategory{{ID: 1, SortOrder: 0, Name: "默认", GroupIDs: []int64{10}}}
	payload, err := json.Marshal(original)
	require.NoError(t, err)
	repo := &groupCategoriesStoreStub{raw: func() *string { value := string(payload); return &value }()}
	svc := &SettingService{settingRepo: repo}

	_, err = svc.SetGroupCategories(context.Background(), original, []GroupCategory{{ID: 1, Name: "旧名称", GroupIDs: []int64{10}}})
	require.Error(t, err)
	status, body := infraerrors.ToHTTP(err)
	require.Equal(t, 409, status)
	require.Equal(t, "GROUP_CATEGORIES_STALE", body.Reason)
}

func TestSetGroupCategoriesUsesCompareAndSwap(t *testing.T) {
	repo := &groupCategoriesStoreStub{}
	svc := &SettingService{settingRepo: repo}
	next := []GroupCategory{{ID: 1, Name: "运营", GroupIDs: []int64{11, 12}}}

	saved, err := svc.SetGroupCategories(context.Background(), next, []GroupCategory{})
	require.NoError(t, err)
	require.Equal(t, next, saved)
	require.NotNil(t, repo.raw)
}
