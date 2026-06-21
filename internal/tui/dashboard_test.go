// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bitwise-media-group/evolve/internal/run"
)

// inflightCount counts the live execution timers tracked for one case.
func inflightCount(d dashboardModel, ref run.UnitRef, label string) int {
	n := 0
	for _, ifl := range d.inflight {
		if ifl.ref == ref && ifl.label == label {
			n++
		}
	}
	return n
}

// TestBaselineRunningLifecycle pins the baseline-phase row state: a baseline start
// flags the eval's row running-with-baseline (one execution-log entry, one live
// timer), the run-under-test start clears the flag without doubling the timer, and
// completion settles the row. The yellow/blue tint is a render-only style choice.
func TestBaselineRunningLifecycle(t *testing.T) {
	cat := soloCatalog(t)
	_, m1 := soloModels()
	ev := run.UnitRef{Skill: "solo-skill", Key: m1.Key(), Kind: run.KindEvals}
	filter := &run.Filter{
		Skills: map[string]bool{"solo-skill": true},
		Evals:  map[string]map[string]bool{"solo-skill": {"e1": true}},
	}
	d := newDashboard(cat, []run.UnitRef{ev}, filter, run.PriorMetrics{})
	d.w, d.h = 120, 40
	d.apply(unitStartedMsg{ref: ev, total: 1, mode: run.ModeRun})

	// Baseline starts first.
	d.apply(baselineStartedMsg{ref: ev, item: run.ItemStart{Label: "e1"}})
	cr := d.unit(ev).byLabel["e1"]
	if cr == nil || cr.status != stRunning || !cr.baselineRunning {
		t.Fatalf("after baselineStarted: %+v, want running + baselineRunning", cr)
	}
	if got := inflightCount(d, ev, "e1"); got != 1 {
		t.Errorf("inflight for e1 during baseline = %d, want 1", got)
	}
	if got := d.renderRuns(120, 10); !strings.Contains(got, "e1") {
		t.Errorf("runs pane did not render the baseline row:\n%s", got)
	}

	// The run under test starts: the baseline flag clears, no duplicate timer.
	d.apply(itemStartedMsg{ref: ev, item: run.ItemStart{Label: "e1"}})
	if cr.baselineRunning || cr.status != stRunning {
		t.Errorf("after itemStarted: baselineRunning=%v status=%v, want false + running", cr.baselineRunning, cr.status)
	}
	if got := inflightCount(d, ev, "e1"); got != 1 {
		t.Errorf("inflight for e1 after run start = %d, want 1 (no duplicate)", got)
	}

	// Completion settles the row.
	d.apply(itemDoneMsg{ref: ev, item: run.ItemResult{Label: "e1", Status: run.StatusPass,
		Metrics: run.ItemMetrics{AvgRunSeconds: new(2.0)}}})
	if cr.baselineRunning || cr.status != stPass {
		t.Errorf("after itemDone: baselineRunning=%v status=%v, want false + pass", cr.baselineRunning, cr.status)
	}
	if got := inflightCount(d, ev, "e1"); got != 0 {
		t.Errorf("inflight for e1 after done = %d, want 0", got)
	}
}

// TestQuitDialog covers the quit-confirmation flow.
func TestQuitDialog(t *testing.T) {
	m := testModel(t)
	m = step(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if m.screen != screenDashboard {
		t.Fatal("did not reach the dashboard")
	}

	// q opens the dialog without quitting.
	m, cmd := stepCmd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		t.Fatal("q should not quit immediately")
	}
	if !m.dash.confirmQuit || !strings.Contains(m.View(), "Are you sure") {
		t.Errorf("q should open the quit dialog:\n%s", m.View())
	}
	// n dismisses it.
	m, _ = stepCmd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.dash.confirmQuit {
		t.Error("n should dismiss the quit dialog")
	}
	// q then y quits.
	m, _ = stepCmd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if _, cmd = stepCmd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}); cmd == nil {
		t.Error("y in the dialog should quit")
	}
	// Two ctrl+c in a row quit immediately.
	m, _ = stepCmd(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.dash.confirmQuit {
		t.Error("first ctrl+c should open the dialog")
	}
	if _, cmd = stepCmd(m, tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("second ctrl+c should quit")
	}
}
