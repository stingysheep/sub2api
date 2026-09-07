package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SettingKeyUpstreamProviderProfiles stores administrator-maintained upstream
// provider templates. Keeping these in settings avoids a schema migration for
// metadata that does not participate in request scheduling.
const SettingKeyUpstreamProviderProfiles = "admin_upstream_provider_profiles"

type UpstreamProviderProfile struct {
	ID         int64  `json:"id"`
	SortOrder  int64  `json:"sort_order"`
	Name       string `json:"name"`
	NamePrefix string `json:"name_prefix"`
	BaseURL    string `json:"base_url"`
	Platform   string `json:"platform,omitempty"`
	Enabled    bool   `json:"enabled"`
}

func (s *SettingService) GetUpstreamProviderProfiles(ctx context.Context) ([]UpstreamProviderProfile, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyUpstreamProviderProfiles)
	if err != nil {
		if err == ErrSettingNotFound {
			return []UpstreamProviderProfile{}, nil
		}
		return nil, fmt.Errorf("get upstream provider profiles: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return []UpstreamProviderProfile{}, nil
	}

	var profiles []UpstreamProviderProfile
	if err := json.Unmarshal([]byte(value), &profiles); err != nil {
		return nil, fmt.Errorf("decode upstream provider profiles: %w", err)
	}
	return normalizeUpstreamProviderProfiles(profiles)
}

func (s *SettingService) SetUpstreamProviderProfiles(ctx context.Context, profiles, expected []UpstreamProviderProfile) ([]UpstreamProviderProfile, error) {
	if expected == nil {
		return nil, infraerrors.Conflict("UPSTREAM_PROFILES_STALE", "Reload upstream profiles before saving")
	}
	normalized, err := normalizeUpstreamProviderProfiles(profiles)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode upstream provider profiles: %w", err)
	}
	// Compare the editor snapshot against storage, not a potentially empty UI prop.
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyUpstreamProviderProfiles)
	var previous *string
	if err == nil {
		previous = &raw
	} else if !errors.Is(err, ErrSettingNotFound) {
		return nil, fmt.Errorf("read upstream provider profiles before save: %w", err)
	}
	current := []UpstreamProviderProfile{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &current); err != nil {
			return nil, fmt.Errorf("decode upstream provider profiles before save: %w", err)
		}
	}
	current, err = normalizeUpstreamProviderProfiles(current)
	if err != nil {
		return nil, err
	}
	expected, err = normalizeUpstreamProviderProfiles(expected)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(current, expected) {
		return nil, infraerrors.Conflict("UPSTREAM_PROFILES_STALE", "Upstream profiles changed; reload before saving")
	}
	store, ok := s.settingRepo.(interface {
		CompareAndSwap(context.Context, string, *string, string) (bool, error)
	})
	if !ok {
		return nil, fmt.Errorf("upstream profile store does not support safe updates")
	}
	saved, err := store.CompareAndSwap(ctx, SettingKeyUpstreamProviderProfiles, previous, string(payload))
	if err != nil {
		return nil, fmt.Errorf("save upstream provider profiles: %w", err)
	}
	if !saved {
		return nil, infraerrors.Conflict("UPSTREAM_PROFILES_STALE", "Upstream profiles changed; reload before saving")
	}
	return normalized, nil
}

func normalizeUpstreamProviderProfiles(profiles []UpstreamProviderProfile) ([]UpstreamProviderProfile, error) {
	result := make([]UpstreamProviderProfile, 0, len(profiles))
	seenIDs := make(map[int64]struct{}, len(profiles))
	maxID := int64(0)
	// Reserve all explicit IDs before assigning new ones; list order must not
	// cause a new profile to collide with an existing account's category.
	for _, profile := range profiles {
		if profile.ID > maxID {
			maxID = profile.ID
		}
	}
	for _, profile := range profiles {
		profile.Name = strings.TrimSpace(profile.Name)
		profile.NamePrefix = strings.TrimSpace(profile.NamePrefix)
		profile.BaseURL = strings.TrimSpace(profile.BaseURL)
		profile.Platform = strings.TrimSpace(profile.Platform)
		if profile.Name == "" {
			return nil, infraerrors.BadRequest("UPSTREAM_PROFILE_NAME_REQUIRED", "Upstream provider profile name is required")
		}
		if len(profile.Name) > 100 || len(profile.NamePrefix) > 100 || len(profile.BaseURL) > 500 {
			return nil, infraerrors.BadRequest("UPSTREAM_PROFILE_FIELD_TOO_LONG", "Upstream provider profile field is too long")
		}
		if profile.ID > maxID {
			maxID = profile.ID
		}
		if profile.ID <= 0 {
			maxID++
			profile.ID = maxID
		}
		if _, exists := seenIDs[profile.ID]; exists {
			return nil, infraerrors.BadRequest("UPSTREAM_PROFILE_DUPLICATE_ID", "Upstream provider profile ID is duplicated")
		}
		seenIDs[profile.ID] = struct{}{}
		result = append(result, profile)
	}
	// Older settings did not persist an order. Keep their historical ID order
	// once, then use explicit order values for subsequent drag-and-drop changes.
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	legacyOrder := len(result) > 1
	for i := 1; i < len(result); i++ {
		if result[i].SortOrder != result[0].SortOrder {
			legacyOrder = false
			break
		}
	}
	if legacyOrder && result[0].SortOrder == 0 {
		for i := range result {
			result[i].SortOrder = int64(i * 10)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
