// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package results

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigratable(t *testing.T) {
	for _, schema := range []int{3, 4} {
		if !Migratable(schema) {
			t.Errorf("schema %d must be migratable", schema)
		}
	}
	for _, schema := range []int{0, 1, 2, Schema, Schema + 1, 99} {
		if Migratable(schema) {
			t.Errorf("schema %d must not be migratable", schema)
		}
	}
}

// v4File is a minimal pre-v5 (tier-major) results file, the oldest structural
// shape MigrateFile still upgrades.
const v4File = `{
  "schema": 4, "plugin": "p", "skill": "s",
  "triggers": {
    "fake/m1": {
      "provider": "fake", "model": "m1", "executed": true, "ran_at": "2026-06-10T00:00:00Z",
      "results": [{"query": "q1", "should_trigger": true, "hits": 2, "runs": 3, "passed": true}],
      "summary": {"passed": 1, "total": 1}
    }
  }
}`

func TestMigrateFileUpgradesOlderSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	if err := os.WriteFile(path, []byte(v4File), 0o644); err != nil {
		t.Fatal(err)
	}

	from, upgraded, err := MigrateFile(dir, "p", "s", "json")
	if err != nil {
		t.Fatal(err)
	}
	if from != 4 || !upgraded {
		t.Fatalf("MigrateFile = (from %d, upgraded %v), want (4, true)", from, upgraded)
	}

	// The file on disk now carries the current schema and the new model-major shape.
	loaded, reset := LoadDir(dir, "p", "s")
	if reset {
		t.Fatal("rewritten file must not reset on reload")
	}
	if loaded.Schema != Schema {
		t.Errorf("on-disk schema = %d, want %d", loaded.Schema, Schema)
	}
	if loaded.Trigger("fake/m1") == nil {
		t.Error("trigger history lost after MigrateFile")
	}
}

func TestMigrateFileCurrentIsNoOp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "evals", "go-testing")
	saveDir(t, sample(), dir, "json")
	before, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		t.Fatal(err)
	}

	from, upgraded, err := MigrateFile(dir, "golang", "go-testing", "json")
	if err != nil {
		t.Fatal(err)
	}
	if from != Schema || upgraded {
		t.Fatalf("MigrateFile = (from %d, upgraded %v), want (%d, false)", from, upgraded, Schema)
	}
	after, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a current-schema file must be left untouched")
	}
}

func TestMigrateFileMissingIsNoOp(t *testing.T) {
	from, upgraded, err := MigrateFile(t.TempDir(), "p", "s", "json")
	if err != nil || from != 0 || upgraded {
		t.Fatalf("MigrateFile on empty dir = (from %d, upgraded %v, err %v), want (0, false, nil)", from, upgraded, err)
	}
}

// TestMigrateFilePreservesUnmigratable pins the deliberate divergence from
// LoadDir: a schema this binary cannot convert is reported as an error and left
// byte-for-byte on disk, never discarded the way LoadDir resets it.
func TestMigrateFilePreservesUnmigratable(t *testing.T) {
	cases := map[string]string{
		"newer":      `{"schema": 99, "models": {"m": {}}}`,
		"too-old":    `{"schema": 1, "plugin": "p", "skill": "s"}`,
		"unreadable": "{corrupt",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "results.json")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, upgraded, err := MigrateFile(dir, "p", "s", "json"); err == nil || upgraded {
				t.Fatalf("MigrateFile = (upgraded %v, err %v), want (false, error)", upgraded, err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != content {
				t.Error("an unmigratable file must be left untouched, not overwritten")
			}
		})
	}
}
