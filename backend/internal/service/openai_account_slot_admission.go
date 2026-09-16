package service

import "context"

// OpenAIAccountSlotRequirements carries the request constraints that must still
// hold after waiting for a concurrency slot. It does not change selection policy.
type OpenAIAccountSlotRequirements struct {
	GroupID                 *int64
	Platform                string
	RequestedModel          string
	RequiredCapability      OpenAIEndpointCapability
	RequiredImageCapability OpenAIImagesCapability
	RequireCompact          bool
	RequireHTTPContinuation bool
}

type openAIAccountSlotAdmissionKey struct{}

type openAIAccountSlotAdmission struct {
	requirements     OpenAIAccountSlotRequirements
	requirePrivacy   bool
	quarantineBypass bool
	accountID        int64
	proxyID          int64
	imageIntent      bool
	imagesEndpoint   bool
	quotaAutoPause   OpsOpenAIAccountQuotaAutoPauseSettings
}

func attachOpenAIAccountSlotAdmission(ctx context.Context, selection *AccountSelectionResult, requirements OpenAIAccountSlotRequirements, requirePrivacy bool) *AccountSelectionResult {
	if selection == nil || selection.Account == nil {
		return selection
	}
	if requirements.GroupID != nil {
		groupID := *requirements.GroupID
		requirements.GroupID = &groupID
	}
	proxyID, _ := openAIProxyStreamCircuitProxyID(selection.Account)
	// Do not mutate a result that a custom scheduler may reuse for another attempt.
	result := *selection
	result.slotAdmission = &openAIAccountSlotAdmission{
		requirements: requirements, requirePrivacy: requirePrivacy,
		quarantineBypass: openAIProxyStreamQuarantineBypassed(ctx),
		accountID:        selection.Account.ID, proxyID: proxyID,
		imageIntent:    OpenAIImageGenerationIntentFromContext(ctx),
		imagesEndpoint: OpenAIImagesEndpointFromContext(ctx),
		quotaAutoPause: openAIQuotaAutoPauseSettingsFromContext(ctx),
	}
	return &result
}

// AccountSlotVetoLatest rechecks runtime state even when profit control is off.
// Only queued admission requires a DB refresh; immediate admission reuses the
// scheduler's fresh selection or a newer snapshot without an extra DB round trip.
func (s *OpenAIGatewayService) AccountSlotVetoLatest(ctx context.Context, selected *Account, requirements OpenAIAccountSlotRequirements, waited bool) (*Account, bool, string) {
	if selected == nil {
		return nil, true, "account_nil"
	}
	admission, _ := ctx.Value(openAIAccountSlotAdmissionKey{}).(*openAIAccountSlotAdmission)
	requirePrivacy := false
	if admission != nil {
		if selected.ID != admission.accountID {
			return selected, true, "selection_mismatch"
		}
		continuation := requirements.RequireHTTPContinuation
		requirements = admission.requirements
		requirements.RequireHTTPContinuation = continuation
		requirePrivacy = admission.requirePrivacy
		ctx = withOpenAIQuotaAutoPauseSettings(ctx, admission.quotaAutoPause)
		if admission.imageIntent {
			ctx = WithOpenAIImageGenerationIntent(ctx)
		}
		if admission.imagesEndpoint {
			ctx = WithOpenAIImagesEndpoint(ctx)
		}
	} else if s != nil {
		requirePrivacy = s.openAIGroupRequiresPrivacySet(ctx, requirements.GroupID)
	}
	latest := selected
	var err error
	if waited && s != nil && s.accountRepo != nil {
		latest, err = s.accountRepo.GetByID(ctx, selected.ID)
	} else if s != nil && s.schedulerSnapshot != nil {
		latest, err = s.schedulerSnapshot.GetAccount(ctx, selected.ID)
		if err == nil && latest != nil && latest.UpdatedAt.Before(selected.UpdatedAt) {
			latest = selected
		}
	}
	if err != nil || latest == nil || latest.ID != selected.ID {
		return selected, true, "account_refresh_failed"
	}
	if s != nil && !s.openAIAccountMatchesSchedulingGroup(latest, requirements.GroupID) {
		return latest, true, "group_mismatch"
	}
	if requirePrivacy && !latest.IsPrivacySet() {
		return latest, true, "privacy_not_set"
	}
	platform := requirements.Platform
	if platform == "" {
		platform = selected.Platform
	}
	if reason := openAICompatibleAccountEligibilityFailureReasonBeforeProfit(ctx, latest, platform, requirements.RequestedModel, requirements.RequireCompact, requirements.RequiredCapability); reason != "" {
		return latest, true, reason
	}
	if requirements.RequireHTTPContinuation && latest.IsOpenAI() && !latest.IsOpenAIApiKey() {
		return latest, true, string(OpenAIHTTPContinuationUnsupportedReason)
	}
	if !latest.SupportsOpenAIImageCapability(requirements.RequiredImageCapability) {
		return latest, true, "image_capability_mismatch"
	}
	if s != nil {
		if s.isOpenAIAccountRequestRuntimeBlocked(latest, requirements.RequestedModel) || s.isOpenAIAccountBlockedBySchedulingThreshold(ctx, latest) {
			return latest, true, "runtime_blocked"
		}
		if !parentHealthyForShadow(latest, s.parentAccountLookup(ctx)) {
			return latest, true, "parent_unavailable"
		}
		proxyID, _ := openAIProxyStreamCircuitProxyID(latest)
		bypass := admission != nil && admission.quarantineBypass && proxyID == admission.proxyID
		// Never trust a bypass inherited from an earlier attempt's context.
		quarantineCtx := context.WithValue(ctx, openAIProxyStreamQuarantineBypassKey{}, bypass)
		if s.isOpenAIProxyStreamQuarantined(quarantineCtx, latest) {
			return latest, true, "proxy_stream_quarantined"
		}
	}
	vetoed, reason := OpenAIProfitControlVeto(ctx, latest)
	return latest, vetoed, reason
}
