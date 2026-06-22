// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package provider

import "testing"

// These benchmarks bound the parent-side CPU a trigger run costs. The runner
// drains a subprocess's stdout line by line and hands every line to the
// provider's ScanLine (see runner.go: `if onLine(line)`), so the evolve process
// pays one ScanLine per emitted line, across `--jobs` concurrent runs. A
// multi-turn agent emits thousands of stream-json lines per run, so a cheap-vs-
// expensive scan strategy compounds quickly.
//
// All providers scan the SAME stream-json bytes here, so the delta is purely the
// scan strategy: the JSON-unmarshalling providers (Anthropic/Google/Cursor parse
// every line, most of which are discarded) vs the substring-matching ones
// (OpenAI/Copilot/Antigravity).

// streamLines is a representative slice of one agent run's stdout: mostly text
// and tool traffic that never hits, with a couple of tool_use lines. Realistic
// in shape and length so json.Unmarshal does representative work.
var streamLines = [][]byte{
	[]byte(`{"type":"system","subtype":"init","session_id":"f1e2","tools":["Skill","Read","Bash","Edit"],"model":"claude-opus-4-8","cwd":"/tmp/ws"}`),
	[]byte(`{"type":"assistant","message":{"id":"msg_01","role":"assistant","content":[{"type":"text","text":"Let me figure out what this task needs before touching anything."}]}}`),
	[]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_01","name":"Bash","input":{"command":"ls -la && cat README.md","description":"survey the repo"}}]}}`),
	[]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu_01","content":"total 24\ndrwxr-xr-x  6 u  staff   192 Jun 22 10:01 .\n-rw-r--r--  1 u  staff  4096 Jun 22 10:01 README.md"}]}}`),
	[]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_02","name":"Read","input":{"file_path":"/tmp/ws/.claude/skills/other-skill/SKILL.md"}}]}}`),
	[]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu_02","content":"# Other skill\n\nA long SKILL.md body that the model reads in full before deciding what to do next, spanning many tokens of guidance and examples."}]}}`),
	[]byte(`{"type":"result","subtype":"success","is_error":false,"result":"All done.","usage":{"input_tokens":1400,"output_tokens":320,"cache_read_input_tokens":12000}}`),
}

var hitSink int

// BenchmarkScanLine compares per-line scan cost across providers over identical
// stream-json input. ns/op × lines-per-run × concurrent jobs ≈ the parent CPU a
// trigger sweep burns parsing subprocess output.
func BenchmarkScanLine(b *testing.B) {
	const skill = "go-docs"
	provs := []struct {
		name string
		p    Provider
	}{
		{"anthropic", NewAnthropic()},
		{"google", NewGoogle()},
		{"cursor", NewCursor()},
		{"openai", NewOpenAI()},
		{"copilot", NewCopilot()},
		{"antigravity", NewAntigravity()},
	}
	for _, pr := range provs {
		b.Run(pr.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				if hit, _ := pr.p.ScanLine(streamLines[i%len(streamLines)], skill); hit {
					hitSink++
				}
			}
		})
	}
}

// BenchmarkScanRun measures the parent-side cost of draining one whole agent run
// for the default provider — linesPerRun lines through ScanLine — so the per-run
// CPU reads as an absolute time rather than a per-line figure.
func BenchmarkScanRun(b *testing.B) {
	const (
		skill       = "go-docs"
		linesPerRun = 4000
	)
	a := NewAnthropic()
	transcript := make([][]byte, linesPerRun)
	for i := range transcript {
		transcript[i] = streamLines[i%len(streamLines)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, line := range transcript {
			if hit, _ := a.ScanLine(line, skill); hit {
				hitSink++
			}
		}
	}
}
