package service

import (
	"context"
	"testing"
	"time"
)

func TestShouldPreemptRecoveredSticky(t *testing.T) {
	groupID := int64(42)
	now := time.Now()
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 1},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 5, Concurrency: 1},
	}}
	svc := &OpenAIGatewayService{
		accountRepo:         repo,
		recoveryCoordinator: &accountRecoveryCoordinator{repo: repo, recovered: map[int64]time.Time{1: now}, active: map[int64]struct{}{}},
	}
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-5.1"}
	if !svc.shouldPreemptRecoveredSticky(context.Background(), req, &repo.accounts[1]) {
		t.Fatal("expected recovered higher-priority account to preempt sticky fallback")
	}
}

func TestShouldPreemptRecoveredStickyIgnoresUnmarkedAccount(t *testing.T) {
	groupID := int64(42)
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 1},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 5, Concurrency: 1},
	}}
	svc := &OpenAIGatewayService{
		accountRepo:         repo,
		recoveryCoordinator: &accountRecoveryCoordinator{repo: repo, recovered: map[int64]time.Time{}, active: map[int64]struct{}{}},
	}
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-5.1"}
	if svc.shouldPreemptRecoveredSticky(context.Background(), req, &repo.accounts[1]) {
		t.Fatal("unmarked account must not preempt sticky fallback")
	}
}

func TestGatewayShouldPreemptRecoveredStickyFromPool(t *testing.T) {
	repo := stubOpenAIAccountRepo{}
	svc := &GatewayService{
		recoveryCoordinator: &accountRecoveryCoordinator{
			repo:      repo,
			recovered: map[int64]time.Time{1: time.Now()},
			active:    map[int64]struct{}{},
		},
	}
	sticky := &Account{ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Priority: 5}
	accounts := []Account{
		{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Priority: 1},
		*sticky,
	}
	if !svc.shouldPreemptRecoveredStickyFromAccounts(context.Background(), PlatformAnthropic, false, "", sticky, accounts) {
		t.Fatal("expected generic gateway to preempt recovered higher-priority sticky account")
	}
}
