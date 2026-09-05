package service

import "testing"

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
