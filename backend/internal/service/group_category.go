package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SettingKeyGroupCategories stores administrator-maintained group folders and
// the group IDs assigned to each folder. It is settings-backed so the feature
// does not require a database migration.
const SettingKeyGroupCategories = "admin_group_categories"

type GroupCategory struct {
	ID        int64   `json:"id"`
	SortOrder int64   `json:"sort_order"`
	Name      string  `json:"name"`
	GroupIDs  []int64 `json:"group_ids"`
}

func (s *SettingService) GetGroupCategories(ctx context.Context) ([]GroupCategory, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyGroupCategories)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return []GroupCategory{}, nil
		}
		return nil, fmt.Errorf("get group categories: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return []GroupCategory{}, nil
	}
	var categories []GroupCategory
	if err := json.Unmarshal([]byte(value), &categories); err != nil {
		return nil, fmt.Errorf("decode group categories: %w", err)
	}
	return normalizeGroupCategories(categories)
}

func (s *SettingService) SetGroupCategories(ctx context.Context, categories, expected []GroupCategory) ([]GroupCategory, error) {
	if expected == nil {
		return nil, infraerrors.Conflict("GROUP_CATEGORIES_STALE", "Reload group categories before saving")
	}
	normalized, err := normalizeGroupCategories(categories)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode group categories: %w", err)
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyGroupCategories)
	var previous *string
	if err == nil {
		previous = &raw
	} else if !errors.Is(err, ErrSettingNotFound) {
		return nil, fmt.Errorf("read group categories before save: %w", err)
	}
	current := []GroupCategory{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &current); err != nil {
			return nil, fmt.Errorf("decode group categories before save: %w", err)
		}
		current, err = normalizeGroupCategories(current)
		if err != nil {
			return nil, fmt.Errorf("normalize group categories before save: %w", err)
		}
	}
	expected, err = normalizeGroupCategories(expected)
	if err != nil {
		return nil, err
	}
	if !equalGroupCategories(current, expected) {
		return nil, infraerrors.Conflict("GROUP_CATEGORIES_STALE", "Group categories changed; reload before saving")
	}

	store, ok := s.settingRepo.(interface {
		CompareAndSwap(context.Context, string, *string, string) (bool, error)
	})
	if !ok {
		return nil, fmt.Errorf("group category store does not support safe updates")
	}
	saved, err := store.CompareAndSwap(ctx, SettingKeyGroupCategories, previous, string(payload))
	if err != nil {
		return nil, fmt.Errorf("save group categories: %w", err)
	}
	if !saved {
		return nil, infraerrors.Conflict("GROUP_CATEGORIES_STALE", "Group categories changed; reload before saving")
	}
	return normalized, nil
}

func normalizeGroupCategories(categories []GroupCategory) ([]GroupCategory, error) {
	result := make([]GroupCategory, 0, len(categories))
	seen := make(map[int64]struct{}, len(categories))
	assigned := make(map[int64]struct{})
	for _, category := range categories {
		if category.ID <= 0 {
			return nil, infraerrors.BadRequest("GROUP_CATEGORY_INVALID_ID", "group category id must be positive")
		}
		if _, ok := seen[category.ID]; ok {
			return nil, infraerrors.BadRequest("GROUP_CATEGORY_DUPLICATE_ID", "group category ids must be unique")
		}
		seen[category.ID] = struct{}{}
		category.Name = strings.TrimSpace(category.Name)
		if category.Name == "" {
			return nil, infraerrors.BadRequest("GROUP_CATEGORY_NAME_REQUIRED", "group category name is required")
		}
		if len([]rune(category.Name)) > 100 {
			return nil, infraerrors.BadRequest("GROUP_CATEGORY_NAME_TOO_LONG", "group category name is too long")
		}
		ids := make([]int64, 0, len(category.GroupIDs))
		for _, groupID := range category.GroupIDs {
			if groupID <= 0 {
				continue
			}
			if _, ok := assigned[groupID]; ok {
				continue
			}
			assigned[groupID] = struct{}{}
			ids = append(ids, groupID)
		}
		category.GroupIDs = ids
		result = append(result, category)
	}
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

func equalGroupCategories(a, b []GroupCategory) bool {
	left, err := normalizeGroupCategories(a)
	if err != nil {
		return false
	}
	right, err := normalizeGroupCategories(b)
	if err != nil || len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].ID != right[i].ID || left[i].SortOrder != right[i].SortOrder || left[i].Name != right[i].Name || len(left[i].GroupIDs) != len(right[i].GroupIDs) {
			return false
		}
		for j := range left[i].GroupIDs {
			if left[i].GroupIDs[j] != right[i].GroupIDs[j] {
				return false
			}
		}
	}
	return true
}
