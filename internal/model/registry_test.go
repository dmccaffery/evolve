// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package model

import (
	"strings"
	"testing"
)

// TestBuiltinInvariants guards the registry's structural rules: ids are
// provider-qualified, the ProviderID prefix matches the id, every model is
// supported by at least one harness, and Preferred is always one of those
// harnesses. A bad Supported/Preferred entry would silently send the wrong
// --model string to a CLI, so these are load-bearing.
func TestBuiltinInvariants(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range builtins() {
		if seen[m.ID] {
			t.Errorf("duplicate model id %q", m.ID)
		}
		seen[m.ID] = true

		prov, rest, ok := strings.Cut(m.ID, "/")
		if !ok || rest == "" {
			t.Errorf("model id %q is not provider-qualified", m.ID)
		}
		if prov != m.ProviderID {
			t.Errorf("model %q: id prefix %q != ProviderID %q", m.ID, prov, m.ProviderID)
		}
		if !IsProviderID(m.ProviderID) {
			t.Errorf("model %q: unknown ProviderID %q", m.ID, m.ProviderID)
		}
		if len(m.Supported) == 0 {
			t.Errorf("model %q: no supported harness", m.ID)
		}
		if _, ok := m.Supported[m.Preferred]; !ok {
			t.Errorf("model %q: Preferred %q not in Supported %v", m.ID, m.Preferred, m.Supported)
		}
	}
}

// TestKeyStability pins the results-key bytes for a vendor-native model so the
// harness split cannot orphan committed results.
func TestKeyStability(t *testing.T) {
	m, ok := ModelByID(builtins(), "anthropic/claude-sonnet-4-6")
	if !ok {
		t.Fatal("anthropic/claude-sonnet-4-6 missing from registry")
	}
	if m.Key() != "anthropic/claude-sonnet-4-6" {
		t.Errorf("Key() = %q, want anthropic/claude-sonnet-4-6", m.Key())
	}
	if m.BareID() != "claude-sonnet-4-6" {
		t.Errorf("BareID() = %q, want claude-sonnet-4-6", m.BareID())
	}
	// Copilot drives the same model under a different CLI id; the divergence
	// lives only in the Supported map, never in the key.
	if id, ok := m.CLIModelID("copilot"); !ok || id != "claude-sonnet-4.6" {
		t.Errorf("copilot CLI id = %q (%v), want claude-sonnet-4.6", id, ok)
	}
	if id, ok := m.CLIModelID("claude"); !ok || id != "claude-sonnet-4-6" {
		t.Errorf("claude CLI id = %q (%v), want claude-sonnet-4-6", id, ok)
	}
}

// TestAllModelsOverride replaces one provider's matrix and leaves the others.
func TestAllModelsOverride(t *testing.T) {
	override := map[string][]Model{
		ProviderCursor: {{
			ID: "cursor/composer-3", ProviderID: ProviderCursor, Name: "Composer 3",
			Supported: map[string]string{HarnessCursor: "composer-3"}, Preferred: HarnessCursor,
		}},
	}
	got := AllModels(override)
	if _, ok := ModelByID(got, "cursor/composer-2.5"); ok {
		t.Error("builtin cursor/composer-2.5 should be replaced by the override")
	}
	if _, ok := ModelByID(got, "cursor/composer-3"); !ok {
		t.Error("override cursor/composer-3 missing")
	}
	if _, ok := ModelByID(got, "anthropic/claude-sonnet-4-6"); !ok {
		t.Error("non-overridden anthropic models should remain")
	}
}
