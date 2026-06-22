// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package model

import "testing"

func TestCounterFor(t *testing.T) {
	for _, pid := range []string{ProviderAnthropic, ProviderOpenAI, ProviderGoogle} {
		if _, ok := CounterFor(pid); !ok {
			t.Errorf("CounterFor(%q) = not found, want a counter", pid)
		}
		if len(CounterEnvKeys(pid)) == 0 {
			t.Errorf("CounterEnvKeys(%q) empty", pid)
		}
	}
	// Cursor has no counting API; harness-only ids are not vendors.
	for _, pid := range []string{ProviderCursor, "copilot", "unknown"} {
		if _, ok := CounterFor(pid); ok {
			t.Errorf("CounterFor(%q) = found, want none", pid)
		}
		if CounterEnvKeys(pid) != nil {
			t.Errorf("CounterEnvKeys(%q) non-nil", pid)
		}
	}
}
