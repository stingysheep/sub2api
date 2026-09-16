package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMapperPreservesMaxReasoningEffortOverLimit(t *testing.T) {
	for _, tt := range []struct {
		name   string
		policy string
	}{
		{name: "deny", policy: service.ReasoningEffortOverLimitDeny},
		{name: "downgrade", policy: service.ReasoningEffortOverLimitDowngrade},
		{name: "empty", policy: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			group := &service.Group{
				Platform:                    service.PlatformOpenAI,
				MaxReasoningEffort:          "medium",
				MaxReasoningEffortOverLimit: tt.policy,
			}
			for name, mapped := range map[string]any{
				"GroupFromServiceAdmin":   GroupFromServiceAdmin(group),
				"GroupFromService":        GroupFromService(group),
				"GroupFromServiceShallow": GroupFromServiceShallow(group),
			} {
				t.Run(name, func(t *testing.T) {
					fields := marshalToMap(t, mapped)
					require.Equal(t, "medium", fields["max_reasoning_effort"])
					require.Contains(t, fields, "max_reasoning_effort_over_limit")
					require.Equal(t, tt.policy, fields["max_reasoning_effort_over_limit"])
				})
			}
		})
	}
}
