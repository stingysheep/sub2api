package service

import (
	"context"
	"encoding/json"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestNormalizeUpstreamProviderProfilesPreservesAndBackfillsOrder(t *testing.T) {
	legacy, err := normalizeUpstreamProviderProfiles([]UpstreamProviderProfile{
		{ID: 20, Name: "second"},
		{ID: 10, Name: "first"},
	})
	if err != nil {
		t.Fatalf("normalize legacy profiles: %v", err)
	}
	if legacy[0].ID != 10 || legacy[0].SortOrder != 0 || legacy[1].ID != 20 || legacy[1].SortOrder != 10 {
		t.Fatalf("legacy profile order = %#v, want IDs 10/20 with sort orders 0/10", legacy)
	}

	explicit, err := normalizeUpstreamProviderProfiles([]UpstreamProviderProfile{
		{ID: 10, Name: "first", SortOrder: 20},
		{ID: 20, Name: "second", SortOrder: 0},
	})
	if err != nil {
		t.Fatalf("normalize explicit profiles: %v", err)
	}
	if explicit[0].ID != 20 || explicit[1].ID != 10 {
		t.Fatalf("explicit profile order = %#v, want IDs 20/10", explicit)
	}
}

func TestUpstreamProfilesNormalizeAllocatesAfterExistingIDs(t *testing.T) {
	got, err := normalizeUpstreamProviderProfiles([]UpstreamProviderProfile{
		{Name: "new"}, {ID: 1, Name: "original"}, {ID: 9, Name: "latest"},
	})
	require.NoError(t, err)
	byName := map[string]int64{}
	for _, p := range got {
		byName[p.Name] = p.ID
	}
	require.Equal(t, int64(10), byName["new"])
	require.Equal(t, int64(1), byName["original"])
}

func TestUpstreamProfilesValidationReturnsBadRequest(t *testing.T) {
	for _, input := range [][]UpstreamProviderProfile{
		{{ID: 1, Name: " "}},
		{{ID: 1, Name: strings.Repeat("a", 101)}},
		{{ID: 1, Name: "one"}, {ID: 1, Name: "two"}},
	} {
		_, err := normalizeUpstreamProviderProfiles(input)
		status, _ := infraerrors.ToHTTP(err)
		require.Equal(t, 400, status)
	}
}

type upstreamProfilesStoreStub struct {
	SettingRepository
	raw      *string
	loseRace bool
	writes   int
}

func (r *upstreamProfilesStoreStub) GetValue(context.Context, string) (string, error) {
	if r.raw == nil {
		return "", ErrSettingNotFound
	}
	return *r.raw, nil
}
func (r *upstreamProfilesStoreStub) CompareAndSwap(_ context.Context, _ string, previous *string, next string) (bool, error) {
	if r.loseRace {
		return false, nil
	}
	if (previous == nil) != (r.raw == nil) {
		return false, nil
	}
	if previous != nil && *previous != *r.raw {
		return false, nil
	}
	r.raw = &next
	r.writes++
	return true, nil
}
func TestUpstreamProfilesSafeSave(t *testing.T) {
	original := []UpstreamProviderProfile{{ID: 1, Name: "relay-one"}, {ID: 2, Name: "relay-two", SortOrder: 10}}
	payload, _ := json.Marshal(original)
	raw := string(payload)
	next := append(append([]UpstreamProviderProfile{}, original...), UpstreamProviderProfile{ID: 3, Name: "new-relay", SortOrder: 20})
	for _, tc := range []struct {
		name       string
		expected   []UpstreamProviderProfile
		race       bool
		wantStatus int
	}{
		{name: "missing snapshot", expected: nil, wantStatus: 409},
		{name: "empty editor cannot replace existing profiles", expected: []UpstreamProviderProfile{}, wantStatus: 409},
		{name: "stale snapshot", expected: original[:1], wantStatus: 409},
		{name: "concurrent writer", expected: original, race: true, wantStatus: 409},
		{name: "add preserves IDs and order", expected: original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &upstreamProfilesStoreStub{raw: &raw, loseRace: tc.race}
			svc := &SettingService{settingRepo: store}
			got, err := svc.SetUpstreamProviderProfiles(context.Background(), next, tc.expected)
			if tc.wantStatus != 0 {
				status, _ := infraerrors.ToHTTP(err)
				require.Equal(t, tc.wantStatus, status)
				require.Equal(t, 0, store.writes)
				require.Equal(t, raw, *store.raw)
			} else {
				require.NoError(t, err)
				require.Equal(t, next, got)
				require.Equal(t, 1, store.writes)
			}
		})
	}
	store := &upstreamProfilesStoreStub{}
	svc := &SettingService{settingRepo: store}
	got, err := svc.SetUpstreamProviderProfiles(context.Background(), original, []UpstreamProviderProfile{})
	require.NoError(t, err)
	require.Equal(t, original, got)
}
