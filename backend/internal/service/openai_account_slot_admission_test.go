package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type slotAdmissionGroupRepo struct {
	GroupRepository
	group *Group
}

func (r slotAdmissionGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func (r slotAdmissionGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func newSlotAdmissionFixture(t *testing.T, group *Group) (*OpenAIGatewayService, *schedulerTestOpenAIAccountRepo) {
	t.Helper()
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
	account.Extra = map[string]any{"privacy_mode": PrivacyModeTrainingOff}
	if group != nil {
		account.GroupIDs = []int64{group.ID}
	}
	repo := &schedulerTestOpenAIAccountRepo{accounts: []Account{account}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: cfg, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
	if group != nil {
		cache := &openAISnapshotCacheStub{snapshotAccounts: []*Account{&repo.accounts[0]}, accountsByID: map[int64]*Account{1: &repo.accounts[0]}}
		svc.schedulerSnapshot = NewSchedulerSnapshotService(cache, nil, repo, slotAdmissionGroupRepo{group: group}, cfg)
	}
	return svc, repo
}

func TestSlotAdmissionQuarantineSelectionIsolation(t *testing.T) {
	for _, mode := range []string{"legacy", "advanced"} {
		t.Run(mode, func(t *testing.T) { testSlotAdmissionQuarantineSelectionIsolation(t, mode == "advanced") })
	}
}

func TestSlotAdmissionContextPreservesNoopAndClearsPrevious(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	admission := &openAIAccountSlotAdmission{accountID: 1, quarantineBypass: true}
	selection := &AccountSelectionResult{slotAdmission: admission}
	withAdmission := ContextWithSelectionProfitGate(base, selection)
	require.Same(t, admission, withAdmission.Value(openAIAccountSlotAdmissionKey{}))
	require.Same(t, withAdmission, ContextWithSelectionProfitGate(withAdmission, selection))
	for _, plain := range []*AccountSelectionResult{nil, {}} {
		require.Same(t, base, ContextWithSelectionProfitGate(base, plain))
		cleared := ContextWithSelectionProfitGate(withAdmission, plain)
		require.Nil(t, cleared.Value(openAIAccountSlotAdmissionKey{}))
		require.Equal(t, base.Done(), cleared.Done())
	}
}

func testSlotAdmissionQuarantineSelectionIsolation(t *testing.T, advanced bool) {
	t.Helper()
	svc, repo := newSlotAdmissionFixture(t, nil)
	if advanced {
		svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
	}
	proxyID := int64(7)
	repo.accounts[0].ProxyID = &proxyID
	svc.openaiProxyStreamCircuit = newOpenAIProxyStreamCircuit(openAIProxyStreamCircuitSettings{failureThreshold: 1, failureWindow: time.Minute, quarantineTTL: time.Minute, maxEntries: 8})
	svc.openaiProxyStreamCircuit.recordFailure(proxyID, time.Now())
	ctx := context.Background()
	selection, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	defer selection.ReleaseFunc()
	admissionCtx := ContextWithSelectionProfitGate(ctx, selection)
	_, vetoed, reason := svc.AccountSlotVetoLatest(admissionCtx, selection.Account, OpenAIAccountSlotRequirements{}, false)
	require.False(t, vetoed, reason)
	require.True(t, svc.isOpenAIProxyStreamQuarantined(ctx, selection.Account), "fallback must not clear the circuit or mutate the request context")
	otherProxy := int64(8)
	svc.openaiProxyStreamCircuit.recordFailure(otherProxy, time.Now())
	repo.accounts[0].ProxyID = &otherProxy
	_, vetoed, reason = svc.AccountSlotVetoLatest(admissionCtx, selection.Account, OpenAIAccountSlotRequirements{}, true)
	require.True(t, vetoed, "fallback for the selected proxy must not authorize a different proxy")
	require.Equal(t, "proxy_stream_quarantined", reason)
	repo.accounts[0].ProxyID = &proxyID

	svc.openaiProxyStreamCircuit.recordSuccess(proxyID)
	next, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	defer next.ReleaseFunc()
	svc.openaiProxyStreamCircuit.recordFailure(proxyID, time.Now())
	// Even accidental reuse of the prior admission context must not confer its bypass.
	nextCtx := ContextWithSelectionProfitGate(admissionCtx, next)
	_, vetoed, reason = svc.AccountSlotVetoLatest(nextCtx, next.Account, OpenAIAccountSlotRequirements{}, false)
	require.True(t, vetoed)
	require.Equal(t, "proxy_stream_quarantined", reason)
}

func TestSlotAdmissionDoesNotMutateReusedSelection(t *testing.T) {
	groupID := int64(10)
	account := &Account{ID: 1}
	original := &AccountSelectionResult{Account: account}
	first := attachOpenAIAccountSlotAdmission(withOpenAIProxyStreamQuarantineBypass(context.Background()), original, OpenAIAccountSlotRequirements{GroupID: &groupID}, true)
	groupID = 20
	next := attachOpenAIAccountSlotAdmission(context.Background(), original, OpenAIAccountSlotRequirements{GroupID: &groupID}, false)
	require.Nil(t, original.slotAdmission)
	require.True(t, first.slotAdmission.quarantineBypass)
	require.EqualValues(t, 10, *first.slotAdmission.requirements.GroupID)
	require.False(t, next.slotAdmission.quarantineBypass)
	require.EqualValues(t, 20, *next.slotAdmission.requirements.GroupID)
	ctx := ContextWithSelectionProfitGate(ContextWithSelectionProfitGate(context.Background(), first), original)
	admission, _ := ctx.Value(openAIAccountSlotAdmissionKey{}).(*openAIAccountSlotAdmission)
	require.Nil(t, admission, "a legacy result must clear the previous selection metadata too")
}

func TestSlotAdmissionRefreshGroupAndPrivacy(t *testing.T) {
	for _, change := range []string{"group", "privacy", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			group := &Group{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, RequirePrivacySet: true}
			svc, repo := newSlotAdmissionFixture(t, group)
			groupID := group.ID
			selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-5", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			defer selection.ReleaseFunc()
			switch change {
			case "group":
				repo.accounts[0].GroupIDs = []int64{20}
			case "privacy":
				repo.accounts[0].Extra = map[string]any{"privacy_mode": "training_on"}
			}
			// The returned selection must own the effective group value, not this pointer.
			groupID = 99
			ctx := ContextWithSelectionProfitGate(context.Background(), selection)
			_, vetoed, reason := svc.AccountSlotVetoLatest(ctx, selection.Account, OpenAIAccountSlotRequirements{GroupID: &groupID}, true)
			if change == "unchanged" {
				require.False(t, vetoed, reason)
			} else {
				require.True(t, vetoed, "queued account lost "+change+" eligibility")
				require.Equal(t, map[string]string{"group": "group_mismatch", "privacy": "privacy_not_set"}[change], reason)
			}
		})
	}
}

func TestSlotAdmissionPreservesEndpointSelectionRequirements(t *testing.T) {
	for _, capability := range []OpenAIEndpointCapability{OpenAIEndpointCapabilityChatCompletions, OpenAIEndpointCapabilityResponses, OpenAIEndpointCapabilityEmbeddings, OpenAIEndpointCapabilityAlphaSearch, OpenAIEndpointCapabilityGrokMediaGeneration} {
		t.Run(string(capability), func(t *testing.T) {
			svc, repo := newSlotAdmissionFixture(t, nil)
			platform, model := PlatformOpenAI, "gpt-5"
			if capability == OpenAIEndpointCapabilityGrokMediaGeneration {
				platform, model = PlatformGrok, "grok-imagine-image"
				repo.accounts[0].Platform = platform
			}
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), nil, "", "", model, nil, OpenAIUpstreamTransportHTTPSSE, capability, false, false, true, platform)
			require.NoError(t, err)
			defer selection.ReleaseFunc()
			admission := selection.slotAdmission
			require.NotNil(t, admission)
			require.Equal(t, capability, admission.requirements.RequiredCapability)
			require.Equal(t, model, admission.requirements.RequestedModel)
			ctx := ContextWithSelectionProfitGate(context.Background(), selection)
			_, vetoed, reason := svc.AccountSlotVetoLatest(ctx, selection.Account, OpenAIAccountSlotRequirements{}, false)
			require.False(t, vetoed, reason)
		})
	}
}

func TestSlotAdmissionGrokVoiceDoesNotRequireTextModel(t *testing.T) {
	svc, repo := newSlotAdmissionFixture(t, nil)
	repo.accounts[0].Platform = PlatformGrok
	repo.accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"grok-old": "grok-old"}}
	selection, _, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), nil, "", "", "", nil, OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityChatCompletions, false, false, false, PlatformGrok)
	require.NoError(t, err)
	defer selection.ReleaseFunc()
	ctx := ContextWithSelectionProfitGate(context.Background(), selection)
	_, vetoed, reason := svc.AccountSlotVetoLatest(ctx, selection.Account, OpenAIAccountSlotRequirements{RequestedModel: "grok-voice-latest"}, false)
	require.False(t, vetoed, reason)
}

func TestSlotAdmissionSelectionModelAndCapability(t *testing.T) {
	for _, change := range []string{"model", "model_limit", "capability", "images_limit", "images_type"} {
		t.Run(change, func(t *testing.T) {
			svc, repo := newSlotAdmissionFixture(t, nil)
			ctx := context.Background()
			var selection *AccountSelectionResult
			var err error
			if change == "images_limit" || change == "images_type" {
				ctx = WithOpenAIImagesEndpoint(WithOpenAIImageGenerationIntent(ctx))
				selection, _, err = svc.SelectAccountWithSchedulerForImages(ctx, nil, "", "gpt-5", nil, OpenAIImagesCapabilityBasic)
			} else {
				selection, _, err = svc.SelectAccountWithSchedulerForCapability(ctx, nil, "", "", "text-embedding-3-small", nil, OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityEmbeddings, false, false, true)
			}
			require.NoError(t, err)
			defer selection.ReleaseFunc()
			switch change {
			case "model":
				repo.accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
			case "capability":
				repo.accounts[0].Type = AccountTypeOAuth
			case "images_type":
				repo.accounts[0].Type = AccountTypeBedrock
			case "model_limit", "images_limit":
				key := "text-embedding-3-small"
				if change == "images_limit" {
					key = openAIImageGenerationRateLimitKey
				}
				setAccountModelRateLimitSnapshot(&repo.accounts[0], key, time.Now().Add(time.Hour), "test", time.Now())
			}
			// Handler's original context has neither the image intent nor scheduler-local data.
			admissionCtx := ContextWithSelectionProfitGate(context.Background(), selection)
			_, vetoed, _ := svc.AccountSlotVetoLatest(admissionCtx, selection.Account, OpenAIAccountSlotRequirements{}, true)
			require.True(t, vetoed, change)
		})
	}
}

type slotAdmissionScheduler struct {
	OpenAIAccountScheduler
	result *AccountSelectionResult
	err    error
}

func (s slotAdmissionScheduler) Select(context.Context, OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.result, OpenAIAccountScheduleDecision{Layer: "synthetic"}, s.err
}

func TestSlotAdmissionCustomSchedulerResultAndErrors(t *testing.T) {
	for _, outcome := range []string{"success", "nil", "error"} {
		t.Run(outcome, func(t *testing.T) {
			group := &Group{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, RequirePrivacySet: true}
			svc, repo := newSlotAdmissionFixture(t, group)
			svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			original := &AccountSelectionResult{Account: &repo.accounts[0]}
			stub := slotAdmissionScheduler{result: original}
			if outcome == "nil" {
				stub.result = nil
			}
			if outcome == "error" {
				stub.err = errors.New("synthetic scheduler failure")
			}
			svc.openaiScheduler = stub
			selection, decision, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), &group.ID, "", "", "text-embedding-3-small", nil, OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityEmbeddings, false, false, true)
			require.Equal(t, "synthetic", decision.Layer)
			require.Nil(t, original.slotAdmission, "a reusable custom result must remain unchanged")
			if outcome == "error" {
				require.ErrorIs(t, err, stub.err)
				require.Same(t, original, selection)
			} else if outcome == "nil" {
				require.NoError(t, err)
				require.Nil(t, selection)
			} else {
				require.NoError(t, err)
				require.NotSame(t, original, selection)
				require.NotNil(t, selection.slotAdmission)
				require.True(t, selection.slotAdmission.requirePrivacy)
				require.Equal(t, OpenAIEndpointCapabilityEmbeddings, selection.slotAdmission.requirements.RequiredCapability)
			}
		})
	}
}

type slotAdmissionRefreshRepo struct {
	AccountRepository
	latest *Account
	err    error
	reads  int
}

func (r *slotAdmissionRefreshRepo) GetByID(context.Context, int64) (*Account, error) {
	r.reads++
	return r.latest, r.err
}

func TestSlotAdmissionRefreshFailuresAndSelectionIdentity(t *testing.T) {
	for _, outcome := range []string{"refresh", "error", "missing", "wrong_id", "selection_mismatch"} {
		t.Run(outcome, func(t *testing.T) {
			selected := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
			latest := *selected
			latest.Name = "fresh synthetic account"
			repo := &slotAdmissionRefreshRepo{latest: &latest}
			selection := attachOpenAIAccountSlotAdmission(context.Background(), &AccountSelectionResult{Account: selected}, OpenAIAccountSlotRequirements{RequestedModel: "gpt-5"}, false)
			ctx := ContextWithSelectionProfitGate(context.Background(), selection)
			switch outcome {
			case "error":
				repo.err = errors.New("synthetic refresh failure")
			case "missing":
				repo.latest = nil
			case "wrong_id":
				latest.ID = 2
			case "selection_mismatch":
				selected.ID = 2
			}
			svc := &OpenAIGatewayService{accountRepo: repo}
			account, vetoed, reason := svc.AccountSlotVetoLatest(ctx, selected, OpenAIAccountSlotRequirements{}, true)
			if outcome == "refresh" {
				require.False(t, vetoed, reason)
				require.Same(t, repo.latest, account)
			} else {
				require.True(t, vetoed)
				require.Same(t, selected, account)
				want := "account_refresh_failed"
				if outcome == "selection_mismatch" {
					want = "selection_mismatch"
				}
				require.Equal(t, want, reason)
			}
			if outcome == "selection_mismatch" {
				require.Zero(t, repo.reads)
			} else {
				require.Equal(t, 1, repo.reads)
			}
		})
	}
}

type slotAdmissionImagesFallbackScheduler struct {
	OpenAIAccountScheduler
	result       *AccountSelectionResult
	capabilities *[]OpenAIImagesCapability
}

func (s slotAdmissionImagesFallbackScheduler) Select(_ context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	*s.capabilities = append(*s.capabilities, req.RequiredImageCapability)
	if req.RequiredImageCapability == OpenAIImagesCapabilityNative {
		return nil, OpenAIAccountScheduleDecision{}, ErrNoAvailableAccounts
	}
	return s.result, OpenAIAccountScheduleDecision{}, nil
}

func TestSlotAdmissionImagesFallbackCapturesEffectiveCapability(t *testing.T) {
	svc, repo := newSlotAdmissionFixture(t, nil)
	svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
	var capabilities []OpenAIImagesCapability
	// Both OAuth and API-key accounts currently support native images. Force the
	// first selection to fail so this verifies the wrapper's actual fallback path.
	svc.openaiScheduler = slotAdmissionImagesFallbackScheduler{
		result:       &AccountSelectionResult{Account: &repo.accounts[0], ReleaseFunc: func() {}},
		capabilities: &capabilities,
	}
	ctx := WithOpenAIImagesEndpoint(WithOpenAIImageGenerationIntent(context.Background()))
	selection, _, err := svc.SelectAccountWithSchedulerForImages(ctx, nil, "", "gpt-5", nil, OpenAIImagesCapabilityNative)
	require.NoError(t, err)
	require.NotNil(t, selection)
	defer selection.ReleaseFunc()
	require.Equal(t, []OpenAIImagesCapability{OpenAIImagesCapabilityNative, OpenAIImagesCapabilityBasic}, capabilities)
	require.Equal(t, OpenAIImagesCapabilityBasic, selection.slotAdmission.requirements.RequiredImageCapability)
	require.True(t, selection.slotAdmission.imageIntent)
	require.True(t, selection.slotAdmission.imagesEndpoint)
	_, vetoed, reason := svc.AccountSlotVetoLatest(ContextWithSelectionProfitGate(context.Background(), selection), selection.Account, OpenAIAccountSlotRequirements{RequiredImageCapability: OpenAIImagesCapabilityNative}, false)
	require.False(t, vetoed, reason)
}
