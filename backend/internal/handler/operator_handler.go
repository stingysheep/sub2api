package handler

import (
	"bytes"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type OperatorHandler struct {
	svc     *service.OperatorService
	balance *service.OperatorBalanceService
}

func NewOperatorHandler(s *service.OperatorService, b *service.OperatorBalanceService) *OperatorHandler {
	return &OperatorHandler{svc: s, balance: b}
}
func operatorID(c *gin.Context) (int64, bool) {
	s, ok := middleware.GetAuthSubjectFromContext(c)
	return s.UserID, ok && s.UserID > 0
}
func operatorQuery(c *gin.Context, allowed ...string) bool {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.BadRequest(c, "Invalid query")
		return false
	}
	for k, v := range values {
		ok := false
		for _, a := range allowed {
			if k == a {
				ok = true
				break
			}
		}
		if !ok || len(v) != 1 {
			response.BadRequest(c, "Unsupported or duplicate query parameter")
			return false
		}
	}
	return true
}
func operatorPage(c *gin.Context) (int, int, bool) {
	p, n := 1, 20
	for k, d := range map[string]*int{"page": &p, "page_size": &n} {
		if s, ok := c.Request.URL.Query()[k]; ok {
			v, e := strconv.Atoi(s[0])
			if e != nil {
				response.BadRequest(c, "Invalid pagination")
				return 0, 0, false
			}
			*d = v
		}
	}
	if !service.OperatorPaginationValid(p, n) {
		response.BadRequest(c, "Invalid pagination")
		return 0, 0, false
	}
	return p, n, true
}
func operatorTarget(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.NotFound(c, "User not found")
		return 0, false
	}
	return id, true
}
func (h *OperatorHandler) ListUsers(c *gin.Context) {
	if !operatorQuery(c, "page", "page_size", "search") {
		return
	}
	p, n, ok := operatorPage(c)
	if !ok {
		return
	}
	v, t, e := h.svc.ListUsers(c.Request.Context(), p, n, strings.TrimSpace(c.Query("search")))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Paginated(c, v, t, p, n)
}
func (h *OperatorHandler) GetUser(c *gin.Context) {
	if !operatorQuery(c) {
		return
	}
	id, ok := operatorTarget(c)
	if !ok {
		return
	}
	v, e := h.svc.GetUser(c.Request.Context(), id)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *OperatorHandler) ListGroups(c *gin.Context) {
	if !operatorQuery(c) {
		return
	}
	v, e := h.svc.ListGroups(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"items": v})
}
func (h *OperatorHandler) ListOrders(c *gin.Context) {
	if !operatorQuery(c, "page", "page_size") {
		return
	}
	id, ok := operatorTarget(c)
	if !ok {
		return
	}
	p, n, ok := operatorPage(c)
	if !ok {
		return
	}
	v, t, e := h.svc.ListOrders(c.Request.Context(), id, p, n)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Paginated(c, v, t, p, n)
}
func (h *OperatorHandler) BalanceHistory(c *gin.Context) {
	if !operatorQuery(c, "page", "page_size") {
		return
	}
	id, ok := operatorTarget(c)
	if !ok {
		return
	}
	p, n, ok := operatorPage(c)
	if !ok {
		return
	}
	v, t, e := h.svc.ListBalanceHistory(c.Request.Context(), id, p, n)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Paginated(c, v, t, p, n)
}
func (h *OperatorHandler) Usage(c *gin.Context) {
	if !operatorQuery(c, "period") {
		return
	}
	id, ok := operatorTarget(c)
	if !ok {
		return
	}
	v, e := h.svc.Usage(c.Request.Context(), id, c.Query("period"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func (h *OperatorHandler) ChannelStatus(c *gin.Context) {
	if !operatorQuery(c) {
		return
	}
	actor, ok := operatorID(c)
	if !ok {
		response.Forbidden(c, "Operator access required")
		return
	}
	v, e := h.svc.ChannelStatus(c.Request.Context(), actor)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"items": v})
}

// Decode the object field-by-field so encoding/json's last-value-wins behavior
// cannot silently accept duplicate financial instructions.
func decodeOperatorAdjustment(body io.Reader) (service.OperatorBalanceAdjustmentRequest, error) {
	var req service.OperatorBalanceAdjustmentRequest
	data, e := io.ReadAll(io.LimitReader(body, 4097))
	if e != nil || len(data) > 4096 {
		return req, service.ErrOperatorBalanceInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return req, service.ErrOperatorBalanceInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		tok, e = d.Token()
		if e != nil {
			return req, e
		}
		k, ok := tok.(string)
		if !ok || seen[k] {
			return req, service.ErrOperatorBalanceInvalid
		}
		seen[k] = true
		var dst *string
		switch k {
		case "operation":
			dst = &req.Operation
		case "amount":
			dst = &req.Amount
		case "source":
			dst = &req.Source
		case "reason":
			dst = &req.Reason
		default:
			return req, service.ErrOperatorBalanceInvalid
		}
		var raw json.RawMessage
		if e = d.Decode(&raw); e != nil {
			return req, e
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return req, service.ErrOperatorBalanceInvalid
		}
		if e = json.Unmarshal(raw, dst); e != nil {
			return req, e
		}
	}
	if _, e = d.Token(); e != nil {
		return req, e
	}
	if _, e = d.Token(); e != io.EOF {
		return req, service.ErrOperatorBalanceInvalid
	}
	return req, nil
}
func (h *OperatorHandler) AdjustBalance(c *gin.Context) {
	if !operatorQuery(c) {
		return
	}
	id, ok := operatorTarget(c)
	actor, authorized := operatorID(c)
	if !ok || !authorized {
		return
	}
	if len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		response.BadRequest(c, "A single Idempotency-Key is required")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	req, e := decodeOperatorAdjustment(c.Request.Body)
	if e != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	out, e := h.balance.Adjust(c.Request.Context(), actor, id, req)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
