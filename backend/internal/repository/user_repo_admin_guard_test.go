package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func createAdminGuardUser(t *testing.T, repo *userRepository, email, role, status string) *service.User {
	t.Helper()
	u := &service.User{Email: email, PasswordHash: "synthetic-hash", Role: role, Status: status}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func TestUserRepositoryAdminGuardRejectsLastAdminAndInactiveReplacement(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	admin := createAdminGuardUser(t, repo, "admin@example.test", service.RoleAdmin, service.StatusActive)
	createAdminGuardUser(t, repo, "inactive@example.test", service.RoleAdmin, service.StatusDisabled)
	admin.Role = service.RoleUser
	require.ErrorContains(t, repo.Update(context.Background(), admin, service.UserUpdateFields{Role: true}), "last admin")
	got, err := repo.GetByID(context.Background(), admin.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleAdmin, got.Role)
}

func TestUserRepositoryAdminGuardRechecksStaleStatusAndDeletion(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	ctx := context.Background()
	u := createAdminGuardUser(t, repo, "promote@example.test", service.RoleUser, service.StatusActive)
	stale := *u
	u.Role = service.RoleAdmin
	require.NoError(t, repo.Update(ctx, u, service.UserUpdateFields{Role: true}))
	stale.Status = service.StatusDisabled
	require.ErrorContains(t, repo.Update(ctx, &stale, service.UserUpdateFields{Status: true}), "cannot disable admin")
	require.ErrorContains(t, repo.Delete(ctx, u.ID), "cannot delete admin")
	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleAdmin, got.Role)
	require.Equal(t, service.StatusActive, got.Status)
}

func TestUserRepositoryAdminGuardRejectsPromotionOfDisabledUser(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	u := createAdminGuardUser(t, repo, "disabled@example.test", service.RoleUser, service.StatusDisabled)
	u.Role = service.RoleAdmin
	require.ErrorContains(t, repo.Update(context.Background(), u, service.UserUpdateFields{Role: true}), "cannot disable admin")
	u.Status = service.StatusActive
	require.NoError(t, repo.Update(context.Background(), u, service.UserUpdateFields{Role: true, Status: true}))
}

func TestUserRepositoryAdminGuardUpdateRespectsOuterRollback(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	u := createAdminGuardUser(t, repo, "rollback@example.test", service.RoleAdmin, service.StatusActive)
	createAdminGuardUser(t, repo, "remaining@example.test", service.RoleAdmin, service.StatusActive)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	u.Role = service.RoleOperator
	require.NoError(t, repo.Update(dbent.NewTxContext(ctx, tx), u, service.UserUpdateFields{Role: true}))
	require.NoError(t, tx.Rollback())
	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleAdmin, got.Role)
}
