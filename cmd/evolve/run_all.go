// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitwise-media-group/evolve/internal/cli"
	"github.com/bitwise-media-group/evolve/internal/grade"
	"github.com/bitwise-media-group/evolve/internal/run"
)

// allFlags is flag storage only: `run all` never reads it. Values reach the
// tiers through forwardFlags, which copies just the flags the user set, so
// each tier keeps its own defaults (timeout: 120 triggers, 600 evals).
var allFlags = TriggersFlags{}

var runAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Run everything: checks, triggers, evals, then regenerate reports",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := reconcileStaleResults(cmd, isTerminal(cmd)); err != nil {
			return err
		}

		// Checks first, then one interleaved triggers+evals sweep (so a skill/model
		// pair finishes both tiers before the next), then the committed reports.
		var failures bool
		if err := runSub(cmd, checksCmd, &failures); err != nil {
			return err
		}

		common, err := allFlags.sweepOptions(cmd)
		if err != nil {
			return err
		}
		if !allFlags.CountOnly {
			fmt.Fprintf(cmd.OutOrStdout(), "parallelism: %d concurrent agent runs\n", allFlags.Jobs)
		}
		triggerTO, evalTO := perTierTimeouts(cmd, allFlags.Timeout)
		failed, runErr := run.Sweep(cmd.Context(), run.SweepOptions{
			Options:        common,
			Tiers:          run.Tiers{Triggers: true, Evals: true},
			Runs:           allFlags.Runs,
			JudgeModel:     grade.DefaultJudgeModel,
			TriggerTimeout: triggerTO,
			EvalTimeout:    evalTO,
		})
		if e := saveCounter(common.Counter); e != nil {
			return e
		}
		if runErr != nil {
			return runErr
		}
		failures = failures || failed

		if err := runSub(cmd, reportCmd, &failures); err != nil {
			return err
		}
		if failures {
			return failOrWarn(cmd, "run: some checks or cases failed")
		}
		return nil
	},
}

// runSub runs a sibling subcommand (checks/report) inline, folding an
// ErrFailures into the failures flag rather than aborting.
func runSub(cmd, sub *cobra.Command, failures *bool) error {
	sub.SetContext(cmd.Context())
	if err := forwardFlags(cmd.Flags(), sub.Flags()); err != nil {
		return err
	}
	if err := sub.RunE(sub, nil); err != nil {
		if errors.Is(err, cli.ErrFailures) {
			*failures = true
			return nil
		}
		return err
	}
	return nil
}

// forwardFlags applies the flags explicitly set on `run all` to one tier's
// flag set, skipping names the tier does not define (--runs is triggers-only;
// checks and report take none of them).
func forwardFlags(from, to *pflag.FlagSet) error {
	var err error
	from.Visit(func(f *pflag.Flag) {
		if err != nil || to.Lookup(f.Name) == nil {
			return
		}
		err = to.Set(f.Name, f.Value.String())
	})
	return err
}

func init() {
	allFlags.register(runAllCmd, 600)
	runAllCmd.Flags().IntVar(&allFlags.Runs, "runs", 3, "runs per query (triggers tier)")
	runAllCmd.Flags().Lookup("timeout").DefValue = "120 triggers, 600 evals"
}
