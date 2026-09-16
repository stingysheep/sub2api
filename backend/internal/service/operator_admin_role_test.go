//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOperatorDemotionProtectsLastAdmin(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "a@example.com", Role: RoleAdmin}}
	repo := &roleGuardUserRepoStub{rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base}, adminTotal: 1}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}
	_, e := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleOperator})
	require.Error(t, e)
	require.Contains(t, e.Error(), "last admin")
	require.Nil(t, repo.lastUpdated)
}
func TestOperatorRoleChangeInvalidatesAuth(t *testing.T) {
	for _, role := range []string{RoleOperator, RoleUser} {
		old := RoleUser
		if role == RoleUser {
			old = RoleOperator
		}
		base := &userRepoStub{user: &User{ID: 42, Email: "a@example.com", Role: old}}
		repo := &rpmUserRepoStub{userRepoStub: base}
		invalidator := &authCacheInvalidatorStub{}
		svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}, authCacheInvalidator: invalidator}
		u, e := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: role})
		require.NoError(t, e)
		require.Equal(t, role, u.Role)
		require.Equal(t, []int64{42}, invalidator.userIDs)
	}
}
