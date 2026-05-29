// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package loglinter provides the reusable analyzer and standalone runner for log-linter.
package loglinter

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Config configures repository scope and rule behavior.
type Config struct {
	// Root is the inferred repository root resolved during normalization.
	Root string `yaml:"-" json:"-"`
	// Exclude defines file patterns skipped for the whole run.
	Exclude []string `yaml:"exclude" json:"exclude"`
	// Rules contains per-rule settings.
	Rules Rules `yaml:"rules" json:"rules"`
}

// Rules groups the individual rule configurations.
type Rules struct {
	StdlibLogCalls       StdlibLogCallsRule       `yaml:"stdlib_log_calls" json:"stdlib_log_calls"`
	SlogImports          SlogImportsRule          `yaml:"slog_imports" json:"slog_imports"`
	ZapMessageFormatting ZapMessageFormattingRule `yaml:"zap_message_formatting" json:"zap_message_formatting"`
	ZapMessageSprintf    ZapMessageSprintfRule    `yaml:"zap_message_sprintf" json:"zap_message_sprintf"`
	ZapRootComponent     ZapRootComponentRule     `yaml:"zap_root_component" json:"zap_root_component"`
}

// RuleScope controls whether a rule is enabled and which files it applies to.
type RuleScope struct {
	Enabled *bool    `yaml:"enabled" json:"enabled"`
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
	Allow   []string `yaml:"allow" json:"allow"`
}

// StdlibLogCallsRule controls disallowed stdlib log calls.
type StdlibLogCallsRule struct {
	RuleScope `yaml:",inline" json:",inline"`

	Functions []string `yaml:"functions" json:"functions"`
}

// SlogImportsRule controls disallowed log/slog imports.
type SlogImportsRule struct {
	RuleScope `yaml:",inline" json:",inline"`
}

// ZapMessageFormattingRule controls printf-style format directives in zap messages.
type ZapMessageFormattingRule struct {
	RuleScope `yaml:",inline" json:",inline"`

	Methods []string `yaml:"methods" json:"methods"`
}

// ZapMessageSprintfRule controls fmt.Sprintf-style zap message construction.
type ZapMessageSprintfRule struct {
	RuleScope `yaml:",inline" json:",inline"`

	Methods   []string `yaml:"methods" json:"methods"`
	Functions []string `yaml:"functions" json:"functions"`
}

// ZapRootComponentRule controls root component requirements for configured zap constructors.
type ZapRootComponentRule struct {
	RuleScope `yaml:",inline" json:",inline"`

	Constructors   []string `yaml:"constructors" json:"constructors"`
	ComponentCalls []string `yaml:"component_calls" json:"component_calls"`
}

// golangCIConfig captures the subset of a golangci-lint config that carries
// the log-linter module plugin settings under
// linters.settings.custom.<name>.settings.
type golangCIConfig struct {
	Linters struct {
		Settings struct {
			Custom map[string]struct {
				Settings golangCISettings `yaml:"settings"`
			} `yaml:"custom"`
		} `yaml:"settings"`
	} `yaml:"linters"`
}

// golangCISettings mirrors the module plugin settings block.
type golangCISettings struct {
	Exclude []string `yaml:"exclude"`
	Rules   Rules    `yaml:"rules"`
}

// LoadConfig reads, resolves, and normalizes a YAML configuration file. The
// file may be a standalone log-linter config or a golangci-lint config that
// carries the settings under linters.settings.custom.<AnalyzerName>.settings;
// the format is detected automatically.
func LoadConfig(path string) (Config, error) {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %q: %w", path, err)
	}

	baseDir := filepath.Dir(resolvedPath)

	settings, isGolangCI, err := golangCILinterSettings(data)
	if err != nil {
		return Config{}, fmt.Errorf("parsing golangci-lint config %q: %w", path, err)
	}

	if isGolangCI {
		return NormalizeConfig(Config{Exclude: settings.Exclude, Rules: settings.Rules}, baseDir)
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %q: %w", path, err)
	}

	return NormalizeConfig(cfg, baseDir)
}

// golangCILinterSettings reports whether data is a golangci-lint config and, if
// so, returns the log-linter settings extracted from it. A golangci-lint config
// is identified by a top-level "linters" or "version" key.
func golangCILinterSettings(data []byte) (golangCISettings, bool, error) {
	var probe map[string]yaml.Node
	if err := yaml.Unmarshal(data, &probe); err != nil {
		// Top-level is not a mapping; treat it as a standalone config and let the
		// direct decode surface any parse error.
		return golangCISettings{}, false, nil //nolint:nilerr
	}

	_, hasLinters := probe["linters"]
	_, hasVersion := probe["version"]

	if !hasLinters && !hasVersion {
		return golangCISettings{}, false, nil
	}

	var gc golangCIConfig
	if err := yaml.Unmarshal(data, &gc); err != nil {
		return golangCISettings{}, false, err
	}

	return gc.Linters.Settings.Custom[AnalyzerName].Settings, true, nil
}

func resolveConfigPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current working directory: %w", err)
	}

	if _, candidate, ok := SearchUpward(cwd, path); ok {
		return candidate, nil
	}

	return path, nil
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
	cfg.Rules.StdlibLogCalls = cfg.Rules.StdlibLogCalls.withDefaults()
	cfg.Rules.SlogImports = cfg.Rules.SlogImports.withDefaults()
	cfg.Rules.ZapMessageFormatting = cfg.Rules.ZapMessageFormatting.withDefaults()
	cfg.Rules.ZapMessageSprintf = cfg.Rules.ZapMessageSprintf.withDefaults()
	cfg.Rules.ZapRootComponent = cfg.Rules.ZapRootComponent.withDefaults()

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

func (rule StdlibLogCallsRule) withDefaults() StdlibLogCallsRule {
	rule.RuleScope = rule.RuleScope.withDefaults()
	if len(rule.Functions) == 0 {
		rule.Functions = []string{"Print", "Printf", "Println", "Fatal", "Fatalf", "Panic", "Panicf"}
	} else {
		rule.Functions = slices.Clone(rule.Functions)
	}

	return rule
}

// Enabled reports whether the stdlib log calls rule is active.
func (rule StdlibLogCallsRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}

func (rule SlogImportsRule) withDefaults() SlogImportsRule {
	rule.RuleScope = rule.RuleScope.withDefaults()

	return rule
}

// Enabled reports whether the slog imports rule is active.
func (rule SlogImportsRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}

func (rule ZapMessageFormattingRule) withDefaults() ZapMessageFormattingRule {
	rule.RuleScope = rule.RuleScope.withDefaults()
	if len(rule.Methods) == 0 {
		rule.Methods = []string{"Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal"}
	} else {
		rule.Methods = slices.Clone(rule.Methods)
	}

	return rule
}

// Enabled reports whether the zap message formatting rule is active.
func (rule ZapMessageFormattingRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}

func (rule ZapMessageSprintfRule) withDefaults() ZapMessageSprintfRule {
	rule.RuleScope = rule.RuleScope.withDefaults()
	if len(rule.Methods) == 0 {
		rule.Methods = []string{"Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal"}
	} else {
		rule.Methods = slices.Clone(rule.Methods)
	}

	if len(rule.Functions) == 0 {
		rule.Functions = []string{"fmt.Sprintf"}
	} else {
		rule.Functions = slices.Clone(rule.Functions)
	}

	return rule
}

// Enabled reports whether the zap message sprintf rule is active.
func (rule ZapMessageSprintfRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}

func (rule ZapRootComponentRule) withDefaults() ZapRootComponentRule {
	rule.RuleScope = rule.RuleScope.withDefaults()
	if len(rule.Constructors) == 0 {
		rule.Constructors = []string{"github.com/siderolabs/talos/pkg/logging.ZapLogger"}
	} else {
		rule.Constructors = slices.Clone(rule.Constructors)
	}

	if len(rule.ComponentCalls) == 0 {
		rule.ComponentCalls = []string{"github.com/siderolabs/talos/pkg/logging.Component"}
	} else {
		rule.ComponentCalls = slices.Clone(rule.ComponentCalls)
	}

	return rule
}

// Enabled reports whether the zap root component rule is active.
func (rule ZapRootComponentRule) Enabled() bool {
	return ruleEnabled(rule.RuleScope.Enabled)
}
