package repository

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToServicePreservesReasoningEffortOverLimit(t *testing.T) {
	for _, policy := range []string{service.ReasoningEffortOverLimitDeny, service.ReasoningEffortOverLimitDowngrade} {
		t.Run(policy, func(t *testing.T) {
			got := groupEntityToService(&dbent.Group{
				MaxReasoningEffort:          "medium",
				MaxReasoningEffortOverLimit: policy,
			})
			require.Equal(t, "medium", got.MaxReasoningEffort)
			require.Equal(t, policy, got.MaxReasoningEffortOverLimit)
		})
	}
}

func TestAPIKeyAuthProjectionPreservesReasoningEffortOverLimit(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "reasoning-projection@example.invalid")
	for _, policy := range []string{service.ReasoningEffortOverLimitDeny, service.ReasoningEffortOverLimitDowngrade} {
		t.Run(policy, func(t *testing.T) {
			group, err := client.Group.Create().SetName("projection-" + policy).
				SetMaxReasoningEffort("medium").SetMaxReasoningEffortOverLimit(policy).Save(ctx)
			require.NoError(t, err)
			key := &service.APIKey{UserID: user.ID, Key: "test-projection-" + policy,
				Name: "projection", GroupID: &group.ID, Status: service.StatusActive}
			require.NoError(t, repo.Create(ctx, key))
			got, err := repo.GetByKeyForAuth(ctx, key.Key)
			require.NoError(t, err)
			require.NotNil(t, got.Group)
			require.Equal(t, policy, got.Group.MaxReasoningEffortOverLimit)
		})
	}
}

func TestGroupRepositoryWritesReasoningEffortOverLimit(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, policy := range []string{service.ReasoningEffortOverLimitDeny, service.ReasoningEffortOverLimitDowngrade} {
			t.Run(operation+"/"+policy, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
				t.Cleanup(func() { _ = client.Close() })

				mutationSeen := false
				client.Group.Use(func(next dbent.Mutator) dbent.Mutator {
					return dbent.MutateFunc(func(ctx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
						m, ok := mutation.(*dbent.GroupMutation)
						require.True(t, ok)
						value, set := m.MaxReasoningEffortOverLimit()
						mutationSeen = true
						assert.True(t, set, "write must explicitly carry the policy")
						assert.Equal(t, policy, value)
						return next.Mutate(ctx, mutation)
					})
				})

				const id int64 = 31
				if operation == "create" {
					mock.ExpectQuery(`INSERT INTO "groups" .*"max_reasoning_effort_over_limit".* RETURNING "id"`).
						WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
				} else {
					mock.ExpectBegin()
					mock.ExpectExec(`UPDATE "groups" SET .*"max_reasoning_effort_over_limit" = \$[0-9]+`).
						WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectQuery(`SELECT .* FROM "groups" WHERE .*"id" = \$1`).
						WithArgs(id).
						WillReturnRows(sqlmock.NewRows([]string{"id", "updated_at", "max_reasoning_effort_over_limit"}).
							AddRow(id, time.Now(), policy))
					mock.ExpectCommit()
				}
				mock.ExpectExec(`INSERT INTO scheduler_outbox`).
					WillReturnResult(sqlmock.NewResult(1, 1))

				group := &service.Group{
					ID:                          id,
					Name:                        "reasoning-policy",
					Platform:                    service.PlatformOpenAI,
					Status:                      service.StatusActive,
					SubscriptionType:            service.SubscriptionTypeStandard,
					MaxReasoningEffort:          "medium",
					MaxReasoningEffortOverLimit: policy,
				}
				repo := newGroupRepositoryWithSQL(client, db)
				if operation == "create" {
					err = repo.Create(context.Background(), group)
				} else {
					err = repo.Update(context.Background(), group)
				}
				require.NoError(t, err)
				require.True(t, mutationSeen)
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
