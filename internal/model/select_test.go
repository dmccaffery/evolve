// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package model

import (
	"slices"
	"testing"
)

func TestModelIdentity(t *testing.T) {
	m := Model{
		ID: "anthropic/claude-sonnet-4-6", ProviderID: "anthropic",
		Supported: map[string]string{"claude": "claude-sonnet-4-6", "copilot": "claude-sonnet-4.6"},
		Preferred: "claude",
	}
	if m.Key() != "anthropic/claude-sonnet-4-6" {
		t.Errorf("Key() = %q", m.Key())
	}
	if m.BareID() != "claude-sonnet-4-6" {
		t.Errorf("BareID() = %q", m.BareID())
	}
	if !m.Supports("copilot") || m.Supports("gemini") {
		t.Error("Supports wrong for copilot/gemini")
	}
	if got := m.SupportedHarnessIDs(); !slices.Equal(got, []string{"claude", "copilot"}) {
		t.Errorf("SupportedHarnessIDs() = %v, want sorted [claude copilot]", got)
	}
}

// TestBareIDUnqualified: an id without a provider prefix returns unchanged.
func TestBareIDUnqualified(t *testing.T) {
	if got := (Model{ID: "solo"}).BareID(); got != "solo" {
		t.Errorf("BareID() = %q, want solo", got)
	}
}

func TestPricingHelpers(t *testing.T) {
	m := Model{InputUSD: usd(2.0), OutputUSD: usd(10.0)}
	if got := InputCostUSD(m, new(1_000_000)); got == nil || *got != 2.0 {
		t.Errorf("InputCostUSD = %v, want 2.0", got)
	}
	if got := InputCostUSD(m, nil); got != nil {
		t.Errorf("InputCostUSD(nil) = %v, want nil", got)
	}
	if got := InputCostUSD(Model{}, new(1)); got != nil {
		t.Errorf("InputCostUSD without pricing = %v, want nil", got)
	}
	u := Usage{InputTokens: new(1_000_000), OutputTokens: new(1_000_000)}
	if got := UsageCostUSD(m, u); got == nil || *got != 12.0 {
		t.Errorf("UsageCostUSD = %v, want 12.0 (2 input + 10 output)", got)
	}
}
