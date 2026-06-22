// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package harness

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/bitwise-media-group/evolve/internal/model"
)

// Codex drives the `codex` CLI (OpenAI Codex).
type Codex struct {
	base
}

// NewCodex returns the builtin Codex harness.
func NewCodex() *Codex {
	return &Codex{base: base{
		id:        model.HarnessCodex,
		name:      "OpenAI Codex",
		clis:      []string{"codex"},
		envKeys:   []string{"EVOLVE_OPENAI_API_KEY", "OPENAI_API_KEY"},
		skillDirs: []string{filepath.Join(".agents", "skills")},
	}}
}

func (c *Codex) TriggerSpec(ws, query, cliModelID string, hostSandboxed bool) model.CommandSpec {
	argv := []string{"codex", "exec", query, "--json", "--skip-git-repo-check", "-m", cliModelID}
	if hostSandboxed {
		// codex defaults to a read-only Seatbelt sandbox even for exec; that nests
		// illegally inside evolve's, so disable it and let evolve confine.
		argv = append(argv, "--sandbox", "danger-full-access")
	}
	return model.CommandSpec{Argv: argv, Dir: ws}
}

// ScanLine is best-effort: any event-stream line mentioning the skill's
// SKILL.md path counts as an activation.
func (c *Codex) ScanLine(line []byte, skill string) (bool, string) {
	return strings.Contains(string(line), "skills/"+skill+"/SKILL.md"), ""
}

func (c *Codex) EvalSpec(ws string, in model.EvalInput, cliModelID string) model.CommandSpec {
	// codex applies its own macOS Seatbelt sandbox for read-only/workspace-write,
	// which cannot nest inside evolve's. When evolve already confines the run,
	// switch codex to danger-full-access so evolve's sandbox is the sole layer;
	// otherwise keep workspace-write as codex's own confinement.
	sandboxMode := "workspace-write"
	if in.HostSandboxed {
		sandboxMode = "danger-full-access"
	}
	return model.CommandSpec{
		Argv: []string{
			"codex", "exec", in.Prompt,
			"--json", "--skip-git-repo-check",
			"--sandbox", sandboxMode,
			"-m", cliModelID,
		},
		Dir: ws,
	}
}

// ParseEvalOutput concatenates agent messages from the codex event stream and
// captures the last turn's usage. Codex reports tokens but not cost; the engine
// prices the tokens from the model matrix.
func (c *Codex) ParseEvalOutput(stdout []byte) (string, *model.Usage) {
	var texts []string
	var usage *model.Usage
	for line := range strings.SplitSeq(string(stdout), "\n") {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage *struct {
				InputTokens       *int `json:"input_tokens"`
				CachedInputTokens *int `json:"cached_input_tokens"`
				OutputTokens      *int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			texts = append(texts, event.Item.Text)
		}
		if event.Type == "turn.completed" && event.Usage != nil {
			// Codex reports input_tokens as the whole prompt with
			// cached_input_tokens a subset of it. The Usage contract wants fresh
			// (uncached) input on InputTokens and cache hits reported separately,
			// so split the cached portion off rather than letting re-read context
			// inflate the headline input figure.
			u := &model.Usage{OutputTokens: event.Usage.OutputTokens}
			if in := event.Usage.InputTokens; in != nil {
				fresh := *in
				if cached := event.Usage.CachedInputTokens; cached != nil {
					read := min(*cached, fresh)
					fresh -= read
					u.CacheReadTokens = &read
				}
				u.InputTokens = &fresh
			}
			usage = u
		}
	}
	if len(texts) == 0 {
		return string(stdout), usage
	}
	return strings.Join(texts, "\n"), usage
}

// ReportsUsage reports that codex reports token usage (cost is computed from
// pricing).
func (c *Codex) ReportsUsage() bool { return true }

// RuntimeError detects a codex run that produced no agent output (auth blocked,
// crash) so it is reported distinctly from a failed eval. A run that emitted any
// agent_message event is gradable, regardless of exit code.
func (c *Codex) RuntimeError(stdout []byte, exitCode int, timedOut bool) string {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return "empty CLI output"
	}
	for line := range strings.SplitSeq(string(stdout), "\n") {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			return "" // produced agent output — gradable
		}
	}
	if exitCode != 0 {
		return "codex produced no agent output"
	}
	return ""
}
