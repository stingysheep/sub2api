//go:build integration

package repository

import (
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"testing"
)

func adminGuardPostgresRepo(t *testing.T) (*userRepository, *dbent.Client) {
	return adminGuardPostgresSchemaRepo(t, integrationDB, integrationPostgresDSN)
}
