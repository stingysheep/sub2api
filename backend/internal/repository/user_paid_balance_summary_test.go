package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryGetPaidBalanceSummaryRanksCurrentPaidBalances(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()

	users := []*service.User{
		{Email: "largest@example.com", Username: "largest", PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive},
		{Email: "second@example.com", Username: "second", PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive},
		{Email: "free@example.com", Username: "free", PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive},
	}
	for _, user := range users {
		require.NoError(t, repo.Create(ctx, user))
	}
	require.NoError(t, client.User.UpdateOneID(users[0].ID).SetPaidBalance(12.5).Exec(ctx))
	require.NoError(t, client.User.UpdateOneID(users[1].ID).SetPaidBalance(4.25).Exec(ctx))

	summary, err := repo.GetPaidBalanceSummary(ctx, 10)
	require.NoError(t, err)
	require.InDelta(t, 16.75, summary.TotalPaidBalance, 0.000001)
	require.Equal(t, int64(2), summary.UsersWithPaidBalance)
	require.Len(t, summary.Ranking, 2)
	require.Equal(t, users[0].ID, summary.Ranking[0].UserID)
	require.Equal(t, users[1].ID, summary.Ranking[1].UserID)
}
