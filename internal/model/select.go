// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package model

import (
	"slices"
	"strings"
)

// Key is the results-file model key: the provider-qualified id
// ("anthropic/claude-sonnet-4-6"). Byte-identical to the pre-split
// provider/model key for vendor-native harnesses, so committed results keep
// loading. Harness never appears in it.
func (m Model) Key() string { return m.ID }

// BareID is the id without its provider prefix ("claude-sonnet-4-6"). This is
// the vendor's own model id — what the results Header records and what a
// vendor counting API expects, independent of the executing harness.
func (m Model) BareID() string {
	if _, id, ok := strings.Cut(m.ID, "/"); ok {
		return id
	}
	return m.ID
}

// CLIModelID returns the harness-specific model-id string this model's
// --model flag expects for harnessID, and whether the harness supports it.
func (m Model) CLIModelID(harnessID string) (string, bool) {
	id, ok := m.Supported[harnessID]
	return id, ok
}

// Supports reports whether harnessID can run this model.
func (m Model) Supports(harnessID string) bool {
	_, ok := m.Supported[harnessID]
	return ok
}

// MatchedBy reports whether any token names this model: "all", its provider id,
// its canonical id, or its bare id. Tokens are trimmed of surrounding space.
// This is the shared matcher behind the root `models` config and the eval-set
// models restriction.
func (m Model) MatchedBy(tokens []string) bool {
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "all" || t == m.ProviderID || t == m.ID || t == m.BareID() {
			return true
		}
	}
	return false
}

// SupportedHarnessIDs lists the harness ids that can run this model, sorted for
// stable display.
func (m Model) SupportedHarnessIDs() []string {
	ids := make([]string, 0, len(m.Supported))
	for id := range m.Supported {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
