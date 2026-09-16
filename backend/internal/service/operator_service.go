package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"time"
)

var ErrOperatorUserNotFound = infraerrors.NotFound("OPERATOR_USER_NOT_FOUND", "User not found")
var ErrOperatorQueryInvalid = infraerrors.BadRequest("OPERATOR_QUERY_INVALID", "Invalid operator query")

type OperatorUser struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	Status       string     `json:"status"`
	Balance      float64    `json:"balance"`
	FreeBalance  float64    `json:"free_balance"`
	PaidBalance  float64    `json:"paid_balance"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt *time.Time `json:"last_active_at"`
}
type OperatorGroup struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform"`
	Status     string    `json:"status"`
	ModelCount int       `json:"model_count"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type OperatorOrder struct {
	ID        int64      `json:"id"`
	Amount    string     `json:"amount"`
	Currency  string     `json:"currency"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at"`
}
type OperatorUsage struct {
	Period           string    `json:"period"`
	RequestCount     int64     `json:"request_count"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	UsageAmount      string    `json:"usage_amount"`
	StartAt          time.Time `json:"start_at"`
	EndAt            time.Time `json:"end_at"`
}
type OperatorBalanceHistory struct {
	ID        int64     `json:"id"`
	Operation string    `json:"operation"`
	Amount    string    `json:"amount"`
	Source    string    `json:"source"`
	Time      time.Time `json:"time"`
	Reason    string    `json:"reason"`
}
type OperatorChannelStatus struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	LatencyMs *int64    `json:"latency_ms"`
	UpdatedAt time.Time `json:"updated_at"`
}
type OperatorReadRepository interface {
	ListUsers(context.Context, int, int, string) ([]OperatorUser, int64, error)
	GetUser(context.Context, int64) (*OperatorUser, error)
	ListGroups(context.Context) ([]OperatorGroup, error)
	ListOrders(context.Context, int64, int, int) ([]OperatorOrder, int64, error)
	ListBalanceHistory(context.Context, int64, int, int) ([]OperatorBalanceHistory, int64, error)
	Usage(context.Context, int64, time.Time, time.Time) (*OperatorUsage, error)
	ListChannelStatus(context.Context) ([]OperatorChannelStatus, error)
}
type operatorGroupAuthorizer interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
}
type operatorV2Reader interface {
	ParseFilter(string, []string, []string, []int64) (ChannelMonitorV2Filter, error)
	Matrix(context.Context, ChannelMonitorV2Filter, ChannelMonitorV2GroupBy, bool) (*ChannelMonitorV2Matrix, error)
}
type OperatorService struct {
	read     OperatorReadRepository
	v2       operatorV2Reader
	groups   operatorGroupAuthorizer
	settings *SettingService
}

func NewOperatorService(read OperatorReadRepository, v2 *ChannelMonitorV2Service, groups *APIKeyService, settings *SettingService) *OperatorService {
	return &OperatorService{read: read, v2: v2, groups: groups, settings: settings}
}
func OperatorPaginationValid(p, n int) bool { return p >= 1 && p <= 1000000 && n >= 1 && n <= 100 }
func (s *OperatorService) ListUsers(c context.Context, p, n int, q string) ([]OperatorUser, int64, error) {
	if !OperatorPaginationValid(p, n) || len(q) > 200 {
		return nil, 0, ErrOperatorQueryInvalid
	}
	return s.read.ListUsers(c, p, n, q)
}
func (s *OperatorService) GetUser(c context.Context, id int64) (*OperatorUser, error) {
	if id <= 0 {
		return nil, ErrOperatorUserNotFound
	}
	return s.read.GetUser(c, id)
}
func (s *OperatorService) ListGroups(c context.Context) ([]OperatorGroup, error) {
	return s.read.ListGroups(c)
}
func (s *OperatorService) ListOrders(c context.Context, id int64, p, n int) ([]OperatorOrder, int64, error) {
	if id <= 0 {
		return nil, 0, ErrOperatorUserNotFound
	}
	if !OperatorPaginationValid(p, n) {
		return nil, 0, ErrOperatorQueryInvalid
	}
	return s.read.ListOrders(c, id, p, n)
}
func (s *OperatorService) ListBalanceHistory(c context.Context, id int64, p, n int) ([]OperatorBalanceHistory, int64, error) {
	if id <= 0 {
		return nil, 0, ErrOperatorUserNotFound
	}
	if !OperatorPaginationValid(p, n) {
		return nil, 0, ErrOperatorQueryInvalid
	}
	return s.read.ListBalanceHistory(c, id, p, n)
}
func OperatorUsageWindow(period string, now time.Time) (time.Time, time.Time, error) {
	now = now.UTC()
	switch period {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), now, nil
	case "7d":
		return now.AddDate(0, 0, -7), now, nil
	case "30d":
		return now.AddDate(0, 0, -30), now, nil
	case "90d":
		return now.AddDate(0, 0, -90), now, nil
	default:
		return time.Time{}, time.Time{}, ErrOperatorQueryInvalid
	}
}
func (s *OperatorService) Usage(c context.Context, id int64, period string) (*OperatorUsage, error) {
	if id <= 0 {
		return nil, ErrOperatorUserNotFound
	}
	if period == "" {
		period = "30d"
	}
	start, end, e := OperatorUsageWindow(period, time.Now())
	if e != nil {
		return nil, e
	}
	v, e := s.read.Usage(c, id, start, end)
	if e == nil {
		v.Period = period
		v.StartAt = start
		v.EndAt = end
	}
	return v, e
}
func (s *OperatorService) ChannelStatus(c context.Context, actor int64) ([]OperatorChannelStatus, error) {
	if actor <= 0 {
		return nil, ErrOperatorBalanceForbidden
	}
	if s.settings == nil {
		return nil, ErrOperatorBalanceUnavailable
	}
	runtime := s.settings.GetChannelMonitorRuntime(c)
	if !runtime.Enabled {
		return []OperatorChannelStatus{}, nil
	}
	if runtime.Mode == ChannelMonitorModeV1 {
		return s.read.ListChannelStatus(c)
	}
	if s.v2 == nil || s.groups == nil {
		return nil, ErrOperatorBalanceUnavailable
	}
	groups, e := s.groups.GetAvailableGroups(c, actor)
	if e != nil {
		return nil, e
	}
	filter, e := s.v2.ParseFilter("90m", nil, nil, nil)
	if e != nil {
		return nil, e
	}
	filter.RestrictGroups = true
	filter.AllowedGroupIDs = make([]int64, 0, len(groups))
	for _, g := range groups {
		filter.AllowedGroupIDs = append(filter.AllowedGroupIDs, g.ID)
	}
	matrix, e := s.v2.Matrix(c, filter, ChannelMonitorV2GroupByPlatformGroup, false)
	if e != nil {
		return nil, e
	}
	out := make([]OperatorChannelStatus, 0)
	if matrix == nil {
		return out, nil
	}
	for _, row := range matrix.Items {
		if row.GroupID == nil {
			continue
		}
		out = append(out, OperatorChannelStatus{ID: *row.GroupID, Name: row.GroupName, Status: row.Health.Overall, LatencyMs: row.Metrics.TTFT.P50Ms, UpdatedAt: matrix.Coverage.DataThrough})
	}
	return out, nil
}
