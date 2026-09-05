package domain

const (
	ReasoningEffortMatchExact  = "exact"
	ReasoningEffortMatchPrefix = "prefix"
	ReasoningEffortMatchSuffix = "suffix"
)

// ReasoningEffortMapping rewrites one explicit OpenAI/Codex reasoning effort
// value to another before the group ceiling is applied.
type ReasoningEffortMapping struct {
	From      string `json:"from"`
	To        string `json:"to"`
	MatchType string `json:"match_type,omitempty"`
	Model     string `json:"model,omitempty"`
}
