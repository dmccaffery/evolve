// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package provider

import (
	"encoding/json"
	"slices"
	"testing"
)

// flagValue returns the argument following flag in argv, or "" when flag is
// absent (or has no following value).
func flagValue(argv []string, flag string) string {
	for i, a := range argv {
		if a == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

// When evolve already wraps the run in its own OS sandbox, the agent CLI must
// disable its own to avoid an illegal nested Seatbelt sandbox; when evolve runs
// unconfined, the agent keeps its sandbox as the sole protection.

func TestClaudeSandboxOffIsValidDisablingJSON(t *testing.T) {
	var got struct {
		Sandbox struct {
			Enabled bool `json:"enabled"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal([]byte(claudeSandboxOff), &got); err != nil {
		t.Fatalf("claudeSandboxOff is not valid JSON: %v", err)
	}
	if got.Sandbox.Enabled {
		t.Error("claudeSandboxOff must set sandbox.enabled=false")
	}
}

func TestAnthropicDisablesOwnSandboxWhenHostSandboxed(t *testing.T) {
	a := NewAnthropic()

	on := a.TriggerSpec("/ws", "q", "claude-opus-4-8", true)
	if got := flagValue(on.Argv, "--settings"); got != claudeSandboxOff {
		t.Errorf("host-sandboxed trigger: --settings = %q, want %q", got, claudeSandboxOff)
	}
	if off := a.TriggerSpec("/ws", "q", "claude-opus-4-8", false); slices.Contains(off.Argv, "--settings") {
		t.Errorf("unconfined trigger must keep Claude's own sandbox: %v", off.Argv)
	}

	onE := a.EvalSpec("/ws", EvalInput{Prompt: "p", HostSandboxed: true}, "claude-opus-4-8")
	if got := flagValue(onE.Argv, "--settings"); got != claudeSandboxOff {
		t.Errorf("host-sandboxed eval: --settings = %q, want %q", got, claudeSandboxOff)
	}
	if offE := a.EvalSpec("/ws", EvalInput{Prompt: "p"}, "claude-opus-4-8"); slices.Contains(offE.Argv, "--settings") {
		t.Errorf("unconfined eval must keep Claude's own sandbox: %v", offE.Argv)
	}
}

func TestOpenAISandboxModeFollowsHostSandbox(t *testing.T) {
	o := NewOpenAI()

	if got := flagValue(o.EvalSpec("/ws", EvalInput{Prompt: "p", HostSandboxed: true}, "gpt-5.4").Argv, "--sandbox"); got != "danger-full-access" {
		t.Errorf("host-sandboxed eval: --sandbox = %q, want danger-full-access", got)
	}
	if got := flagValue(o.EvalSpec("/ws", EvalInput{Prompt: "p"}, "gpt-5.4").Argv, "--sandbox"); got != "workspace-write" {
		t.Errorf("unconfined eval: --sandbox = %q, want workspace-write", got)
	}
	if got := flagValue(o.TriggerSpec("/ws", "q", "gpt-5.4", true).Argv, "--sandbox"); got != "danger-full-access" {
		t.Errorf("host-sandboxed trigger: --sandbox = %q, want danger-full-access", got)
	}
	if off := o.TriggerSpec("/ws", "q", "gpt-5.4", false); slices.Contains(off.Argv, "--sandbox") {
		t.Errorf("unconfined trigger must keep codex's default sandbox (no override): %v", off.Argv)
	}
}

func TestGoogleDisablesOwnSandboxWhenHostSandboxed(t *testing.T) {
	g := NewGoogle()

	if env := g.TriggerSpec("/ws", "q", "gemini-3.5-flash", true).Env; !slices.Contains(env, "GEMINI_SANDBOX=false") {
		t.Errorf("host-sandboxed trigger env = %v, want GEMINI_SANDBOX=false", env)
	}
	if env := g.TriggerSpec("/ws", "q", "gemini-3.5-flash", false).Env; slices.Contains(env, "GEMINI_SANDBOX=false") {
		t.Errorf("unconfined trigger must not force gemini's sandbox off: %v", env)
	}
}

// Providers whose agent CLI applies no OS sandbox of its own ignore the flag
// entirely: the command is identical whether or not evolve sandboxes the run.
func TestProvidersWithoutOwnSandboxIgnoreHostSandbox(t *testing.T) {
	for _, p := range []Provider{NewCursor(), NewCopilot(), NewAntigravity()} {
		on := p.TriggerSpec("/ws", "q", "m", true)
		off := p.TriggerSpec("/ws", "q", "m", false)
		if !slices.Equal(on.Argv, off.Argv) || !slices.Equal(on.Env, off.Env) {
			t.Errorf("%s: hostSandboxed changed the spec: on=%v/%v off=%v/%v",
				p.Name(), on.Argv, on.Env, off.Argv, off.Env)
		}
	}
}
