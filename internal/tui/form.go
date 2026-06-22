// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bitwise-media-group/evolve/internal/evalspec"
	"github.com/bitwise-media-group/evolve/internal/plan"
)

// formAction is what a key press resolved to on the selection screen.
type formAction int

const (
	actionNone formAction = iota
	actionRun
	actionCancel
)

// The form's four focusable regions.
const (
	paneFilters = iota
	paneHarness
	paneModels
	paneTree
	paneCount
)

// Case-marker and selection glyphs. Markers identify the tier; the selection
// glyph shows the resolved state the Session computes (force on/off, or auto
// queued for all / some / none of a node's applicable enabled models).
const (
	markTrigger = "⌖"
	markEval    = "⚙"

	glyphForceOn  = "☑"
	glyphForceOff = "☐"
	glyphAutoAll  = "◉"
	glyphPartial  = "◷"
	glyphAutoNone = "○"
)

// formModel is the selection screen: a left column stacking the filter,
// harness, and model panes, beside a plugin → skill → case tree. All selection
// state lives in the plan.Session; the form navigates and routes key presses to
// the Session's receivers, then renders each pane's glyphs from it.
type formModel struct {
	session *plan.Session
	cat     []plan.SkillCatalog

	filters list
	harness list
	models  list
	tree    tree
	focus   int
	w, h    int
}

func newForm(session *plan.Session, cat []plan.SkillCatalog, evalFilter string) formModel {
	f := formModel{
		session: session,
		cat:     cat,
		filters: list{items: []listItem{
			{label: "new", id: "new"},
			{label: "modified", id: "modified"},
			{label: "failed", id: "failed"},
		}},
		tree: buildCaseTree(cat),
	}
	for _, h := range session.Harnesses() {
		f.harness.items = append(f.harness.items, listItem{label: h.Harness.Name(), id: h.Harness.ID()})
	}
	for _, m := range session.Models() {
		f.models.items = append(f.models.items, listItem{label: m.Name, id: m.Key()})
	}
	// --eval forces every non-matching eval off so the run is scoped to it.
	if evalFilter != "" {
		for _, n := range f.tree.nodes {
			if n.leaf && n.kind == plan.KindEvals && n.caseKey != evalFilter {
				session.SetCases([]plan.CaseRef{{Skill: n.skill, Kind: plan.KindEvals, Case: n.caseKey}}, plan.Off)
			}
		}
	}
	// Open branches that have a preselected case; keep the rest compact.
	f.tree.expandWhere(func(cr plan.CaseRef) bool {
		switch session.NodeSel([]plan.CaseRef{cr}) {
		case plan.SelForceOn, plan.SelAutoAll, plan.SelAutoPartial:
			return true
		default:
			return false
		}
	})
	return f
}

// buildCaseTree builds the plugin → skill → case tree: under each skill the
// trigger cases come first (target glyph), then the eval cases (gear glyph).
func buildCaseTree(cat []plan.SkillCatalog) tree {
	var t tree
	pluginNode := map[string]int{}
	for _, sc := range cat {
		if len(sc.Triggers) == 0 && len(sc.Evals) == 0 {
			continue
		}
		pidx, ok := pluginNode[sc.Plugin]
		if !ok {
			pidx = t.add(treeNode{label: sc.Plugin, parent: -1, expanded: true})
			pluginNode[sc.Plugin] = pidx
		}
		sidx := t.add(treeNode{label: sc.Skill, depth: 1, parent: pidx, expanded: true, skill: sc.Skill})
		for _, tr := range sc.Triggers {
			t.add(treeNode{
				label: triggerLabel(tr), depth: 2, parent: sidx, leaf: true,
				skill: sc.Skill, kind: plan.KindTriggers, caseKey: tr.Query, hasKind: true,
			})
		}
		for _, ev := range sc.Evals {
			t.add(treeNode{
				label: evalLabel(ev), depth: 2, parent: sidx, leaf: true,
				skill: sc.Skill, kind: plan.KindEvals, caseKey: ev.ID, hasKind: true,
			})
		}
	}
	return t
}

func (f formModel) valid() bool { return f.session.AnyQueued() }

// update handles one key on the form and reports whether the user chose to run
// or cancel. Toggles route to the Session; the next render reflects the result.
func (f formModel) update(key string) (formModel, formAction) {
	switch key {
	case "tab":
		f.focus = (f.focus + 1) % paneCount
	case "shift+tab":
		f.focus = (f.focus + paneCount - 1) % paneCount
	case "1":
		f.focus = paneFilters
	case "2":
		f.focus = paneHarness
	case "3":
		f.focus = paneModels
	case "4":
		f.focus = paneTree
	case "up", "k":
		f.moveCursor(-1)
	case "down", "j":
		f.moveCursor(1)
	case "left", "h":
		if f.focus == paneTree {
			f.tree.expand(false)
		}
	case "right", "l":
		if f.focus == paneTree {
			f.tree.expand(true)
		}
	case "]":
		if f.focus == paneTree {
			f.tree.expandLevel()
		}
	case "[":
		if f.focus == paneTree {
			f.tree.collapseLevel()
		}
	case "g", "home":
		f.toEnd(true)
	case "G", "end":
		f.toEnd(false)
	case " ", "space", "enter":
		f.toggle()
	case "r":
		if f.valid() {
			return f, actionRun
		}
	case "esc", "q", "ctrl+c":
		return f, actionCancel
	}
	return f, actionNone
}

// moveCursor advances the focused region's cursor.
func (f *formModel) moveCursor(delta int) {
	switch f.focus {
	case paneFilters:
		f.filters.move(delta)
	case paneHarness:
		f.harness.move(delta)
	case paneModels:
		f.models.move(delta)
	case paneTree:
		f.tree.move(delta)
	}
}

// toEnd jumps the focused region's cursor to the top (start=true) or bottom.
func (f *formModel) toEnd(start bool) {
	jump := func(l *list) {
		if start {
			l.top()
		} else {
			l.bottom()
		}
	}
	switch f.focus {
	case paneFilters:
		jump(&f.filters)
	case paneHarness:
		jump(&f.harness)
	case paneModels:
		jump(&f.models)
	case paneTree:
		if start {
			f.tree.top()
		} else {
			f.tree.bottom()
		}
	}
}

// toggle applies the toggle key to the focused region's current selection.
func (f *formModel) toggle() {
	switch f.focus {
	case paneFilters:
		if it, ok := f.filters.current(); ok {
			cur := f.session.FilterState()
			switch it.id {
			case "new":
				f.session.SetNewFilter(!cur.New)
			case "modified":
				f.session.SetModifiedFilter(!cur.Modified)
			case "failed":
				f.session.SetFailedFilter(!cur.Failed)
			}
		}
	case paneHarness:
		if it, ok := f.harness.current(); ok && f.harnessAvailable(it.id) {
			f.session.EnableHarness(it.id, !f.session.HarnessEnabled(it.id))
		}
	case paneModels:
		if it, ok := f.models.current(); ok && f.modelRunnable(it.id) {
			f.session.EnableModel(it.id, !f.session.ModelEnabled(it.id))
		}
	case paneTree:
		if i := f.tree.currentNode(); i >= 0 {
			f.cycleNode(f.tree.caseLeaves(i))
		}
	}
}

// cycleNode advances a node through auto → off → on → auto (the auto slot is
// skipped when nothing under the node is queued).
func (f *formModel) cycleNode(refs []plan.CaseRef) {
	switch f.session.NodeSel(refs) {
	case plan.SelForceOff:
		f.session.SetCases(refs, plan.On)
	case plan.SelForceOn:
		if f.session.AutoAvailable(refs) {
			f.session.SetCases(refs, plan.Partial)
		} else {
			f.session.SetCases(refs, plan.Off)
		}
	default: // any auto state
		f.session.SetCases(refs, plan.Off)
	}
}

func (f formModel) harnessAvailable(id string) bool {
	for _, h := range f.session.Harnesses() {
		if h.Harness.ID() == id {
			return h.Available
		}
	}
	return false
}

func (f formModel) modelRunnable(key string) bool {
	for _, m := range f.session.Models() {
		if m.Key() == key {
			return f.session.ModelRunnable(m)
		}
	}
	return false
}

// request turns the current Session state into a RunRequest: the enabled
// (model, harness) selections and the resolved plan.Selection. The engine and
// dashboard re-Build from these — the same inputs the Session's Plan() uses — so
// what runs matches what the form showed.
func (f formModel) request() RunRequest {
	return RunRequest{Models: f.session.EnabledSelections(), Selection: f.session.Selection()}
}

// view renders the filter/harness/model column beside the case tree, with a
// button/hint footer.
func (f formModel) view() string {
	const footerH = 4
	paneH := max(f.h-footerH, 8)
	leftW := max(f.w/3, 20)
	rightW := max(f.w-leftW, 20)

	filtersH := 5
	harnessH := min(len(f.harness.items)+2, max(paneH/3, 5))
	modelsH := max(paneH-filtersH-harnessH, 4)

	cw := panelContentWidth(leftW)
	filters := panel(1, "Filters", "", "",
		f.renderFilters(cw, filtersH-2), f.focus == paneFilters, leftW, filtersH)
	harness := panel(2, "Harnesses", "", "",
		f.renderHarness(cw, harnessH-2), f.focus == paneHarness, leftW, harnessH)
	models := panel(3, "Models", f.modelCount(), "",
		f.renderModels(cw, modelsH-2), f.focus == paneModels, leftW, modelsH)
	left := lipgloss.JoinVertical(lipgloss.Left, filters, harness, models)

	right := panel(4, "Plugins / Skills / Cases", "", "",
		f.renderTree(panelContentWidth(rightW), paneH-2), f.focus == paneTree, rightW, paneH)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	runBtn := buttonStyle.Render("r  RUN")
	if f.valid() {
		runBtn = buttonActive.Render("r  RUN")
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, buttonStyle.Render("esc  CANCEL"), "  ", runBtn)
	hint := footerHint.Render("1/2/3/4 pane · ↑↓/jk move · ←→/hl fold · space toggle · g/G ends")
	footer := lipgloss.JoinVertical(lipgloss.Left, buttons, hint)

	return lipgloss.JoinVertical(lipgloss.Left, panes, footer)
}

func (f formModel) modelCount() string {
	on := 0
	for _, m := range f.session.Models() {
		if f.session.ModelEnabled(m.Key()) {
			on++
		}
	}
	return countLabel(on, len(f.models.items))
}

// renderFilters draws the new/modified/failed checkboxes.
func (f formModel) renderFilters(w, h int) string {
	cur := f.session.FilterState()
	on := map[string]bool{"new": cur.New, "modified": cur.Modified, "failed": cur.Failed}
	return renderRows(f.filters.items, f.filters.cursor, f.focus == paneFilters, w, h,
		func(it listItem) string {
			return checkGlyph(on[it.id]) + " " + it.label
		})
}

// renderHarness draws the harness rows, greying any whose CLI is off PATH.
func (f formModel) renderHarness(w, h int) string {
	return renderRows(f.harness.items, f.harness.cursor, f.focus == paneHarness, w, h,
		func(it listItem) string {
			if !f.harnessAvailable(it.id) {
				return errStyle.Render("[!]") + " " + mutedStyle.Render(it.label+" (n/a)")
			}
			return checkGlyph(f.session.HarnessEnabled(it.id)) + " " + it.label
		})
}

// renderModels draws the model rows, greying any not runnable under the enabled
// harnesses.
func (f formModel) renderModels(w, h int) string {
	return renderRows(f.models.items, f.models.cursor, f.focus == paneModels, w, h,
		func(it listItem) string {
			if !f.modelRunnable(it.id) {
				return mutedStyle.Render("[·] " + it.label + " (uns.)")
			}
			return checkGlyph(f.session.ModelEnabled(it.id)) + " " + it.label
		})
}

// renderRows draws a flat list with a cursor marker, scrolled to keep the cursor
// visible, each row rendered by line.
func renderRows(items []listItem, cursor int, focused bool, w, h int, line func(listItem) string) string {
	rows := max(h, 1)
	start := 0
	if cursor >= rows {
		start = cursor - rows + 1
	}
	end := min(start+rows, len(items))
	var b strings.Builder
	for i := start; i < end; i++ {
		row := clip(line(items[i]), w-2)
		if i == cursor && focused {
			row = selectedStyle.Render("› ") + row
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderTree draws the visible plugin/skill/case rows, computing each row's
// glyph from the Session, scrolled to keep the cursor on screen.
func (f formModel) renderTree(w, h int) string {
	vis := f.tree.visible()
	if f.tree.cursor >= len(vis) {
		f.tree.cursor = len(vis) - 1
	}
	if f.tree.cursor < 0 {
		f.tree.cursor = 0
	}
	rows := max(h, 1)
	start := 0
	if f.tree.cursor >= rows {
		start = f.tree.cursor - rows + 1
	}
	end := min(start+rows, len(vis))

	var b strings.Builder
	for pos := start; pos < end; pos++ {
		i := vis[pos]
		n := f.tree.nodes[i]
		arrow := "  "
		if !n.leaf && len(n.children) > 0 {
			if n.expanded {
				arrow = "▾ "
			} else {
				arrow = "▸ "
			}
		}
		glyph := selGlyph(f.session.NodeSel(f.tree.caseLeaves(i)))
		marker := ""
		if n.leaf {
			marker = caseMarker(n.kind) + " "
		}
		row := strings.Repeat("  ", n.depth) + arrow + glyph + " " + marker + n.label
		row = clip(row, w-2)
		if pos == f.tree.cursor && f.focus == paneTree {
			row = selectedStyle.Render("› ") + row
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		if pos < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// checkGlyph renders an on/off checkbox.
func checkGlyph(on bool) string {
	if on {
		return passStyle.Render("[x]")
	}
	return mutedStyle.Render("[ ]")
}

// selGlyph maps a node's resolved selection to its rendered glyph.
func selGlyph(s plan.NodeSel) string {
	switch s {
	case plan.SelForceOn:
		return passStyle.Render(glyphForceOn)
	case plan.SelForceOff:
		return mutedStyle.Render(glyphForceOff)
	case plan.SelAutoAll:
		return runStyle.Render(glyphAutoAll)
	case plan.SelAutoPartial:
		return errStyle.Render(glyphPartial)
	default:
		return mutedStyle.Render(glyphAutoNone)
	}
}

func caseMarker(kind plan.Kind) string {
	if kind == plan.KindEvals {
		return mutedStyle.Render(markEval)
	}
	return mutedStyle.Render(markTrigger)
}

// countLabel renders the "checked of total" tag for a pane's bottom border.
func countLabel(checked, total int) string {
	return fmt.Sprintf("%d of %d", checked, total)
}

func triggerLabel(tr evalspec.Trigger) string {
	mark := "−"
	if tr.ShouldTrigger {
		mark = "+"
	}
	return fmt.Sprintf("%s %s", mark, truncate(tr.Query, 70))
}

func evalLabel(ev evalspec.Eval) string {
	if ev.Name != "" {
		return fmt.Sprintf("%s (%s)", ev.Name, ev.ID)
	}
	return ev.ID
}
