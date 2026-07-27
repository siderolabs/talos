// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubeimportlinter provides the reusable analyzer for kubeimportlinter.
package kubeimportlinter

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Config configures repository scope and rule behavior.
type Config struct {
	// Root is the inferred repository root resolved during normalization.
	Root string `json:"-"`
	// Exclude defines file patterns skipped for the whole run.
	Exclude []string `json:"exclude"`
	// Rules contains per-rule settings.
	Rules Rules `json:"rules"`
}

// Rules groups the individual rule configurations.
type Rules struct {
	VersionedImports VersionedImportsRule `json:"versioned_imports"`
}

// RuleScope controls whether a rule is enabled and which files it applies to.
type RuleScope struct {
	Enabled *bool    `json:"enabled"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	Allow   []string `json:"allow"`
}

// VersionedImportsRule requires versioned Kubernetes imports to be aliased to a
// descriptive name (e.g. corev1) instead of the bare version segment (e.g. v1).
type VersionedImportsRule struct {
	RuleScope `json:",inline"`

	// Hosts lists the import path prefixes the rule applies to.
	Hosts []string `json:"hosts"`
}

// SearchUpward walks from start toward the filesystem root, returning the
// containing directory and full path of the first existing entry among names.
func SearchUpward(start string, names ...string) (dir, path string, ok bool) {
	base := start

	for {
		for _, name := range names {
			candidate := filepath.Join(base, name)
			if _, err := os.Stat(candidate); err == nil {
				return base, candidate, true
			}
		}

		parent := filepath.Dir(base)
		if parent == base {
			return "", "", false
		}

		base = parent
	}
}

// NormalizeConfig applies defaults and resolves the repository root from baseDir.
func NormalizeConfig(cfg Config, baseDir string) (Config, error) {
	absRoot, err := filepath.Abs(baseDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolving config root: %w", err)
	}

	cfg.Root = filepath.Clean(absRoot)
	cfg.Exclude = normalizePatterns(cfg.Exclude)
	cfg.Rules.VersionedImports = cfg.Rules.VersionedImports.withDefaults()

	return cfg, nil
}

func normalizePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}

	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}

		out = append(out, pattern)
	}

	return out
}

func ruleEnabled(enabled *bool) bool {
	if enabled != nil {
		return *enabled
	}

	return true
}

func (scope RuleScope) withDefaults() RuleScope {
	scope.Include = normalizePatterns(scope.Include)
	scope.Exclude = normalizePatterns(scope.Exclude)
	scope.Allow = normalizePatterns(scope.Allow)

	return scope
}

func (rule VersionedImportsRule) withDefaults() VersionedImportsRule {
	rule.RuleScope = rule.RuleScope.withDefaults()
	if len(rule.Hosts) == 0 {
		rule.Hosts = []string{"k8s.io", "sigs.k8s.io"}
	} else {
		rule.Hosts = slices.Clone(rule.Hosts)
	}

	return rule
}

// Enabled reports whether the versioned imports rule is active.
func (rule VersionedImportsRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}
