// Copyright 2026 BitWise Media Group Ltd
// SPDX-License-Identifier: MIT

package provider

import (
	"fmt"
	"slices"
	"strings"
)

// All returns the effective provider set: builtins, with any provider whose
// name appears in overrides getting its model list replaced (replace, not
// merge — partial merges create "which price won?" ambiguity).
func All(overrides map[string][]Model) []Provider {
	providers := []Provider{NewAnthropic(), NewOpenAI(), NewGoogle(), NewCursor(), NewCopilot(), NewAntigravity()}
	for _, p := range providers {
		if models, ok := overrides[p.Name()]; ok && len(models) > 0 {
			p.(modelSetter).setModels(models)
		}
	}
	return providers
}

// Selection is one (provider, model) pair a sweep runs.
type Selection struct {
	Provider Provider
	Model    Model
}

// Key is the results-file model key. Provider-qualified because Cursor runs
// other vendors' models, so bare ids could collide across providers.
func (s Selection) Key() string { return s.Provider.Name() + "/" + s.Model.ID }

// Select resolves a --models spec — provider names, model ids,
// provider/model-id pairs, or "all", comma-separated — to an ordered,
// deduplicated selection. An empty spec defaults to "anthropic".
func Select(spec string, providers []Provider) ([]Selection, error) {
	if strings.TrimSpace(spec) == "" {
		spec = "anthropic"
	}
	var tokens []string
	for t := range strings.SplitSeq(spec, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tokens = append(tokens, t)
		}
	}
	if slices.Contains(tokens, "all") {
		tokens = tokens[:0]
		for _, p := range providers {
			tokens = append(tokens, p.Name())
		}
	}

	byName := map[string]Provider{}
	for _, p := range providers {
		byName[p.Name()] = p
	}

	var selected []Selection
	seen := map[string]bool{}
	add := func(matches []Selection) {
		for _, s := range matches {
			if !seen[s.Key()] {
				seen[s.Key()] = true
				selected = append(selected, s)
			}
		}
	}
	for _, token := range tokens {
		var matches []Selection
		switch {
		case byName[token] != nil:
			for _, m := range byName[token].Models() {
				matches = append(matches, Selection{byName[token], m})
			}
		case strings.Contains(token, "/"):
			name, id, _ := strings.Cut(token, "/")
			if p := byName[name]; p != nil {
				for _, m := range p.Models() {
					if m.ID == id {
						matches = append(matches, Selection{p, m})
					}
				}
			}
		default:
			for _, p := range providers {
				for _, m := range p.Models() {
					if m.ID == token {
						matches = append(matches, Selection{p, m})
					}
				}
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("unknown provider or model in --models: %q", token)
		}
		add(matches)
	}
	return selected, nil
}
