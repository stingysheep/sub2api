package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func postgresAdminGuardUser(t *testing.T, repo *userRepository, role string) *service.User {
	t.Helper()
	u := createAdminGuardUser(t, repo, fmt.Sprintf("synthetic-%d@example.test", time.Now().UnixNano()), role, service.StatusActive)
	t.Cleanup(func() {
		// Retire exactly this synthetic fixture; leave the cluster for root cleanup.
		_, err := repo.sql.ExecContext(context.Background(), "UPDATE users SET deleted_at=NOW() WHERE id=$1", u.ID)
		require.NoError(t, err)
	})
	return u
}

func concurrentAdminGuardOperations(a, b func(context.Context) error) (error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var first, second error
	wg.Add(2)
	go func() { defer wg.Done(); <-start; first = a(ctx) }()
	go func() { defer wg.Done(); <-start; second = b(ctx) }()
	close(start)
	wg.Wait()
	return first, second
}

func TestUserRepositoryAdminGuardPostgresConcurrentDemotions(t *testing.T) {
	repo, client := adminGuardPostgresRepo(t)
	a := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	b := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	a.Role, b.Role = service.RoleUser, service.RoleOperator
	first, second := concurrentAdminGuardOperations(
		func(ctx context.Context) error { return repo.Update(ctx, a, service.UserUpdateFields{Role: true}) },
		func(ctx context.Context) error {
			// Separate repository and database transactions, as with two application instances.
			return newUserRepositoryWithSQL(client, repo.sql).Update(ctx, b, service.UserUpdateFields{Role: true})
		},
	)
	if first == nil {
		require.ErrorContains(t, second, "last admin")
	} else {
		require.ErrorContains(t, first, "last admin")
		require.NoError(t, second)
	}
	count, err := client.User.Query().Where(dbuser.RoleEQ(service.RoleAdmin), dbuser.StatusEQ(service.StatusActive)).Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestUserRepositoryAdminGuardPostgresPromotionVsStatusOrDelete(t *testing.T) {
	for _, operation := range []string{"disable", "delete"} {
		t.Run(operation, func(t *testing.T) {
			repo, client := adminGuardPostgresRepo(t)
			postgresAdminGuardUser(t, repo, service.RoleAdmin)
			u := postgresAdminGuardUser(t, repo, service.RoleUser)
			promotion, stale := *u, *u
			promotion.Role, stale.Status = service.RoleAdmin, service.StatusDisabled
			first, second := concurrentAdminGuardOperations(
				func(ctx context.Context) error {
					return repo.Update(ctx, &promotion, service.UserUpdateFields{Role: true})
				},
				func(ctx context.Context) error {
					if operation == "delete" {
						return repo.Delete(ctx, stale.ID)
					}
					return repo.Update(ctx, &stale, service.UserUpdateFields{Status: true})
				},
			)
			require.True(t, (first == nil) != (second == nil), "exactly one conflicting mutation may succeed: %v / %v", first, second)
			if first == nil {
				require.ErrorContains(t, second, "admin user")
				got, err := repo.GetByID(context.Background(), u.ID)
				require.NoError(t, err)
				require.Equal(t, service.RoleAdmin, got.Role)
				require.Equal(t, service.StatusActive, got.Status)
			}
			count, err := client.User.Query().Where(dbuser.RoleEQ(service.RoleAdmin), dbuser.StatusEQ(service.StatusActive)).Count(context.Background())
			require.NoError(t, err)
			require.GreaterOrEqual(t, count, 1)
		})
	}
}

func TestUserRepositoryAdminGuardPostgresOuterTransactionRetainsLock(t *testing.T) {
	repo, client := adminGuardPostgresRepo(t)
	a := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	b := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	a.Role, b.Role = service.RoleUser, service.RoleOperator
	txCtx := dbent.NewTxContext(ctx, tx)
	require.NoError(t, repo.Update(txCtx, a, service.UserUpdateFields{Role: true}))
	done := make(chan error, 1)
	go func() { done <- repo.Update(ctx, b, service.UserUpdateFields{Role: true}) }()
	select {
	case err := <-done:
		t.Fatalf("another transaction passed the uncommitted membership lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	// Reentering the outer transaction must not deadlock against the waiter.
	require.NoError(t, repo.Update(txCtx, a, service.UserUpdateFields{Role: true}))
	require.NoError(t, tx.Commit())
	require.ErrorContains(t, <-done, "last admin")
}

func TestUserRepositoryAdminGuardPostgresFailedTransactionDoesNotWrite(t *testing.T) {
	repo, client := adminGuardPostgresRepo(t)
	a := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	postgresAdminGuardUser(t, repo, service.RoleAdmin)
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	a.Role = service.RoleOperator
	err = repo.Update(dbent.NewTxContext(context.Background(), tx), a, service.UserUpdateFields{Role: true})
	require.Error(t, err)
	got, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleAdmin, got.Role)
}

func TestUserRepositoryAdminGuardPostgresRepeatableReadCannotUseStaleMembership(t *testing.T) {
	repo, client := adminGuardPostgresRepo(t)
	a := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	b := postgresAdminGuardUser(t, repo, service.RoleAdmin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	// Establish a snapshot with two admins before another transaction demotes A.
	_, err = tx.User.Get(ctx, b.ID)
	require.NoError(t, err)
	a.Role = service.RoleUser
	require.NoError(t, repo.Update(ctx, a, service.UserUpdateFields{Role: true}))
	b.Role = service.RoleOperator
	err = repo.Update(dbent.NewTxContext(ctx, tx), b, service.UserUpdateFields{Role: true})
	require.ErrorContains(t, err, "could not serialize", "obsolete snapshots must fail rather than remove the remaining admin")
	require.NoError(t, tx.Rollback())
	got, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, service.RoleAdmin, got.Role)
}
