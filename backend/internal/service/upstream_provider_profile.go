package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

func (s *SettingService) SetUpstreamProviderProfiles(ctx context.Context, profiles []UpstreamProviderProfile) ([]UpstreamProviderProfile, error) {
	normalized, err := normalizeUpstreamProviderProfiles(profiles)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode upstream provider profiles: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyUpstreamProviderProfiles, string(payload)); err != nil {
		return nil, fmt.Errorf("save upstream provider profiles: %w", err)
	}
	return normalized, nil
}

func normalizeUpstreamProviderProfiles(profiles []UpstreamProviderProfile) ([]UpstreamProviderProfile, error) {
	result := make([]UpstreamProviderProfile, 0, len(profiles))
	seenIDs := make(map[int64]struct{}, len(profiles))
	maxID := int64(0)
	for _, profile := range profiles {
		profile.Name = strings.TrimSpace(profile.Name)
		profile.NamePrefix = strings.TrimSpace(profile.NamePrefix)
		profile.BaseURL = strings.TrimSpace(profile.BaseURL)
		profile.Platform = strings.TrimSpace(profile.Platform)
		if profile.Name == "" {
			return nil, fmt.Errorf("upstream provider profile name is required")
		}
		if len(profile.Name) > 100 || len(profile.NamePrefix) > 100 || len(profile.BaseURL) > 500 {
			return nil, fmt.Errorf("upstream provider profile field is too long")
		}
		if profile.ID > maxID {
			maxID = profile.ID
		}
		if profile.ID <= 0 {
			maxID++
			profile.ID = maxID
		}
		if _, exists := seenIDs[profile.ID]; exists {
			return nil, fmt.Errorf("upstream provider profile id %d is duplicated", profile.ID)
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
