// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitwise-media-group/evolve/internal/evalspec"
	"github.com/bitwise-media-group/evolve/internal/layout"
	"github.com/bitwise-media-group/evolve/internal/results"
	"github.com/bitwise-media-group/evolve/internal/workspace"
)

// SweepOptions configures an interleaved triggers+evals sweep: for each skill in
// repository order, every selected model runs its triggers then its evals before
// the next model, and the next skill follows only once the current one is done.
// This is the execution order the live dashboard mirrors — a skill/model pair
// finishes both tiers before the run moves on, rather than running all triggers
// across everything and then all evals.
type SweepOptions struct {
	Options
	Tiers      Tiers
	Runs       int    // runs per query (triggers tier)
	EvalFilter string // restrict evals to a single id ("" = all)
	JudgeModel string

	// Per-tier timeouts; each falls back to Options.Timeout when zero.
	TriggerTimeout time.Duration
	EvalTimeout    time.Duration

	// Filters narrows the run per resolved model, keyed by Selection.Key(). When
	// non-nil, a model whose entry is nil is skipped entirely — the TUI uses this
	// to honor --new per model. When nil, Options.Filter applies to every model.
	Filters map[string]*Filter
}

// Sweep runs the configured tiers in execution order: skill → model → tiers.
// failed reports whether any executed case failed; err reports interruption or
// setup problems.
func Sweep(ctx context.Context, opts SweepOptions) (failed bool, err error) {
	sets, err := opts.Repo.EvalSets()
	if err != nil {
		return false, err
	}
	for _, set := range sets {
		if opts.SkillFilter != "" && set.Skill != opts.SkillFilter {
			continue
		}
		setFailed, err := runSweepSet(ctx, opts, set)
		failed = failed || setFailed
		if err != nil {
			return failed, err
		}
	}
	return failed, nil
}

// runSweepSet runs every selected model's triggers then evals for one skill,
// sharing a single results file, skill payload, and read-only trigger workspace.
func runSweepSet(ctx context.Context, opts SweepOptions, set layout.EvalSet) (failed bool, err error) {
	runTriggers := opts.Tiers.Triggers && set.TriggersPath != ""
	runEvalsTier := opts.Tiers.Evals && set.EvalsPath != ""
	if !runTriggers && !runEvalsTier {
		return false, nil
	}

	var triggers []evalspec.Trigger
	if runTriggers {
		spec, err := evalspec.LoadTriggers(set.TriggersPath)
		if err != nil {
			return false, err
		}
		warnSkillName(&opts.Options, set, set.TriggersPath, spec.SkillName)
		triggers = spec.Triggers
	}
	var evals []evalspec.Eval
	if runEvalsTier {
		spec, err := evalspec.LoadEvals(set.EvalsPath)
		if err != nil {
			return false, err
		}
		warnSkillName(&opts.Options, set, set.EvalsPath, spec.SkillName)
		evals = filterEvals(spec.Evals, opts.EvalFilter)
		runEvalsTier = len(evals) > 0
	}
	if !runTriggers && !runEvalsTier {
		return false, nil // both tiers narrowed away (e.g. --eval matched nothing)
	}

	file, skillMD, err := loadSet(opts.Options, set)
	if err != nil {
		return false, err
	}
	rep := opts.reporter()

	// The trigger sweep shares one read-only workspace across every model;
	// evals stage their own per-eval workspaces.
	var ws string
	if runTriggers {
		// Triggers need every sibling skill present (the model must pick the
		// right one); evals isolate the skill under test in runEval.
		skills, err := workspace.SkillDirs(set.Plugin.SkillsDir)
		if err != nil {
			return false, err
		}
		w, cleanup, err := workspace.New("triggers.", skills,
			unionSkillDirs(opts.Selected), nil, opts.KeepWorkspaces)
		if err != nil {
			return false, err
		}
		defer cleanup()
		if opts.KeepWorkspaces {
			rep.Warn("  workspace kept: %s\n", w)
		}
		ws = w
	}

	for _, sel := range opts.Selected {
		filter := opts.Filter
		if opts.Filters != nil {
			filter = opts.Filters[sel.Key()]
			if filter == nil {
				continue // this model is already complete (per --new)
			}
		}
		if runTriggers {
			to := opts.Options
			to.Filter = filter
			to.Timeout = pickTimeout(opts.TriggerTimeout, opts.Timeout)
			tf, err := runTriggerUnit(ctx, TriggerOptions{Options: to, Runs: opts.Runs},
				set, sel, file, skillMD, triggers, ws)
			failed = failed || tf
			if err != nil {
				return failed, err
			}
		}
		if runEvalsTier {
			eo := opts.Options
			eo.Filter = filter
			eo.Timeout = pickTimeout(opts.EvalTimeout, opts.Timeout)
			ef, err := runEvalUnit(ctx, EvalOptions{Options: eo, EvalFilter: opts.EvalFilter, JudgeModel: opts.JudgeModel},
				set, sel, file, skillMD, evals)
			failed = failed || ef
			if err != nil {
				return failed, err
			}
		}
	}
	return failed, nil
}

func pickTimeout(tier, fallback time.Duration) time.Duration {
	if tier > 0 {
		return tier
	}
	return fallback
}

// filterEvals restricts a skill's evals to a single id, mirroring --eval; an
// empty id keeps them all.
func filterEvals(evals []evalspec.Eval, id string) []evalspec.Eval {
	if id == "" {
		return evals
	}
	var out []evalspec.Eval
	for _, c := range evals {
		if c.ID == id {
			out = append(out, c)
		}
	}
	return out
}

// loadSet loads the shared per-skill state every unit reads and writes: the
// results file (warning on a discarded incompatible schema) and the skill payload.
func loadSet(opts Options, set layout.EvalSet) (*results.File, []byte, error) {
	rep := opts.reporter()
	file, reset := results.LoadDir(set.ResultsDir, set.Plugin.Name, set.Skill)
	if reset {
		rep.Warn("  warn: %s has an old or unreadable results schema; starting fresh (schema %d)\n",
			opts.Repo.Rel(results.Find(set.ResultsDir)), results.Schema)
	}
	skillMD, err := os.ReadFile(filepath.Join(set.SkillDir, "SKILL.md"))
	if err != nil {
		return nil, nil, fmt.Errorf("reading skill under test: %w", err)
	}
	return file, skillMD, nil
}
