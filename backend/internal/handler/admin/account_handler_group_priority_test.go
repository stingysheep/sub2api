package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountGroupPriorityAdminService struct {
	*stubAdminService
	addedAccountID   int64
	addedGroupID     int64
	updatedAccountID int64
	updatedGroupID   int64
	updatedPriority  int
	removedAccountID int64
	removedGroupID   int64
}

func (s *accountGroupPriorityAdminService) AddAccountToGroup(_ context.Context, accountID, groupID int64) error {
	s.addedAccountID = accountID
	s.addedGroupID = groupID
	return nil
}

func (s *accountGroupPriorityAdminService) UpdateAccountGroupPriority(_ context.Context, accountID, groupID int64, priority int) error {
	s.updatedAccountID = accountID
	s.updatedGroupID = groupID
	s.updatedPriority = priority
	return nil
}

func (s *accountGroupPriorityAdminService) RemoveAccountFromGroup(_ context.Context, accountID, groupID int64) error {
	s.removedAccountID = accountID
	s.removedGroupID = groupID
	return nil
}

func setupAccountGroupPriorityRouter(t *testing.T, svc service.AdminService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/accounts/:id/groups/:group_id", handler.AddToGroup)
	router.DELETE("/accounts/:id/groups/:group_id", handler.RemoveFromGroup)
	router.PUT("/accounts/:id/groups/:group_id/priority", handler.UpdateGroupPriority)
	return router
}

func TestAccountHandlerGroupPriorityActionsUseAccountAndGroupScope(t *testing.T) {
	svc := &accountGroupPriorityAdminService{stubAdminService: newStubAdminService()}
	router := setupAccountGroupPriorityRouter(t, svc)

	addRecorder := httptest.NewRecorder()
	addRequest := httptest.NewRequest(http.MethodPost, "/accounts/12/groups/34", nil)
	router.ServeHTTP(addRecorder, addRequest)
	require.Equal(t, http.StatusOK, addRecorder.Code)
	require.Equal(t, int64(12), svc.addedAccountID)
	require.Equal(t, int64(34), svc.addedGroupID)

	priorityRecorder := httptest.NewRecorder()
	priorityRequest := httptest.NewRequest(http.MethodPut, "/accounts/12/groups/34/priority", bytes.NewBufferString(`{"priority":7}`))
	priorityRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(priorityRecorder, priorityRequest)
	require.Equal(t, http.StatusOK, priorityRecorder.Code)
	require.Equal(t, int64(12), svc.updatedAccountID)
	require.Equal(t, int64(34), svc.updatedGroupID)
	require.Equal(t, 7, svc.updatedPriority)

	var body struct {
		Data struct {
			AccountID int64 `json:"account_id"`
			GroupID   int64 `json:"group_id"`
			Priority  int   `json:"priority"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(priorityRecorder.Body.Bytes(), &body))
	require.Equal(t, int64(12), body.Data.AccountID)
	require.Equal(t, int64(34), body.Data.GroupID)
	require.Equal(t, 7, body.Data.Priority)

	removeRecorder := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodDelete, "/accounts/12/groups/34", nil)
	router.ServeHTTP(removeRecorder, removeRequest)
	require.Equal(t, http.StatusOK, removeRecorder.Code)
	require.Equal(t, int64(12), svc.removedAccountID)
	require.Equal(t, int64(34), svc.removedGroupID)
}

func TestAccountHandlerGroupPriorityRejectsInvalidPriority(t *testing.T) {
	svc := &accountGroupPriorityAdminService{stubAdminService: newStubAdminService()}
	router := setupAccountGroupPriorityRouter(t, svc)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/accounts/12/groups/34/priority", bytes.NewBufferString(`{"priority":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, svc.updatedPriority)
}
