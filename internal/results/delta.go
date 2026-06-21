// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package results

// Delta is the signed change of one comparable metric set between two runs
// (current minus prior). Each field is nil unless both runs measured it, so a
// delta is shown only when the comparison is meaningful. Sign interpretation is
// the presenter's job: a rising Rate is an improvement, while rising cost,
// tokens, or run time are regressions.
type Delta struct {
	Rate          *float64 `json:"rate,omitempty"` // eval pass rate or trigger rate
	AvgRunSeconds *float64 `json:"avg_run_seconds,omitempty"`
	InputTokens   *int     `json:"input_tokens,omitempty"`  // measured (evals) or estimate (triggers)
	OutputTokens  *int     `json:"output_tokens,omitempty"` // evals only
	CostUSD       *float64 `json:"cost_usd,omitempty"`
}

// Zero reports whether the delta carries no comparable figure.
func (d Delta) Zero() bool {
	return d.Rate == nil && d.AvgRunSeconds == nil && d.InputTokens == nil &&
		d.OutputTokens == nil && d.CostUSD == nil
}

// EvalSummaryDelta is current-minus-prior over two eval summaries: pass rate,
// run time, and measured input/output tokens and cost.
func EvalSummaryDelta(cur, prior EvalSummary) Delta {
	return Delta{
		Rate:          subFloat(cur.PassRate, prior.PassRate, Round6),
		AvgRunSeconds: subFloat(cur.AvgRunSeconds, prior.AvgRunSeconds, Round1),
		InputTokens:   subInt(measuredInput(cur.Measured), measuredInput(prior.Measured)),
		OutputTokens:  subInt(measuredOutput(cur.Measured), measuredOutput(prior.Measured)),
		CostUSD:       subFloat(measuredCost(cur.Measured), measuredCost(prior.Measured), Round6),
	}
}

// TriggerSummaryDelta is current-minus-prior over two trigger summaries: trigger
// rate, run time, and the estimate's input tokens and cost.
func TriggerSummaryDelta(cur, prior TriggerSummary) Delta {
	return Delta{
		Rate:          subFloat(cur.PassRate, prior.PassRate, Round6),
		AvgRunSeconds: subFloat(cur.AvgRunSeconds, prior.AvgRunSeconds, Round1),
		InputTokens:   subInt(estimateTokens(cur.Estimate), estimateTokens(prior.Estimate)),
		CostUSD:       subFloat(estimateCost(cur.Estimate), estimateCost(prior.Estimate), Round6),
	}
}

// EvalCaseDelta is current-minus-prior over two eval case-metric sets. The rate
// is the per-eval expectation pass rate (distinct from the aggregate eval pass
// rate compared by EvalSummaryDelta).
func EvalCaseDelta(cur, prior EvalCaseMetrics) Delta {
	return Delta{
		Rate:          subFloat(cur.PassRate, prior.PassRate, Round6),
		AvgRunSeconds: subFloat(cur.AvgRunSeconds, prior.AvgRunSeconds, Round1),
		InputTokens:   subInt(measuredInput(cur.Measured), measuredInput(prior.Measured)),
		OutputTokens:  subInt(measuredOutput(cur.Measured), measuredOutput(prior.Measured)),
		CostUSD:       subFloat(measuredCost(cur.Measured), measuredCost(prior.Measured), Round6),
	}
}

// TriggerCaseDelta is current-minus-prior over two trigger case-metric sets. The
// rate is the correctness rate on each side — the fraction of runs that behaved as
// should_trigger expects (firing for a should-trigger query, not firing for a
// should-not-trigger one) — so a rising rate is always an improvement, including
// for negative queries where fewer hits is better.
func TriggerCaseDelta(cur, prior TriggerCaseMetrics, shouldTrigger bool) Delta {
	return Delta{
		Rate: subFloat(correctRate(cur.Hits, cur.Runs, shouldTrigger),
			correctRate(prior.Hits, prior.Runs, shouldTrigger), Round6),
		AvgRunSeconds: subFloat(cur.AvgRunSeconds, prior.AvgRunSeconds, Round1),
		InputTokens:   subInt(estimateTokens(cur.Estimate), estimateTokens(prior.Estimate)),
		CostUSD:       subFloat(estimateCost(cur.Estimate), estimateCost(prior.Estimate), Round6),
	}
}

func subFloat(a, b *float64, round func(float64) float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := round(*a - *b)
	return &v
}

func subInt(a, b *int) *int {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

// correctRate is the fraction of a query's runs that behaved as should_trigger
// expects: hits/runs for a should-trigger query, (runs-hits)/runs for a
// should-not-trigger one. nil when not comparable.
func correctRate(hits, runs *int, shouldTrigger bool) *float64 {
	if hits == nil || runs == nil || *runs == 0 {
		return nil
	}
	correct := *hits
	if !shouldTrigger {
		correct = *runs - *hits
	}
	v := float64(correct) / float64(*runs)
	return &v
}

func measuredInput(m *Measured) *int {
	if m == nil {
		return nil
	}
	return m.InputTokens
}

func measuredOutput(m *Measured) *int {
	if m == nil {
		return nil
	}
	return m.OutputTokens
}

func measuredCost(m *Measured) *float64 {
	if m == nil {
		return nil
	}
	return m.CostUSD
}

func estimateTokens(e *Estimate) *int {
	if e == nil {
		return nil
	}
	v := e.InputTokens
	return &v
}

func estimateCost(e *Estimate) *float64 {
	if e == nil {
		return nil
	}
	return e.InputCostUSD
}
