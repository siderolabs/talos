// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package loglinter //nolint:testpackage // exercises the unexported plugin entry point

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestLintDisallowsAllRulesByDefault(t *testing.T) {
	issues := lintFixture(t, writeTestFixture(t), Config{})

	assert.Len(t, issues, 5)

	rules := map[string]int{}
	for _, issue := range issues {
		rules[issue.Rule]++
	}

	for _, rule := range []string{
		"stdlib_log_calls",
		"slog_imports",
		"zap_message_formatting",
		"zap_message_sprintf",
		"zap_root_component",
	} {
		assert.Equal(t, 1, rules[rule])
	}
}

func TestLintAllowsConfiguredExceptions(t *testing.T) {
	allow := RuleScope{Allow: []string{"service/bad.go"}}

	cfg := Config{Rules: Rules{
		StdlibLogCalls:       StdlibLogCallsRule{RuleScope: allow},
		SlogImports:          SlogImportsRule{RuleScope: allow},
		ZapMessageFormatting: ZapMessageFormattingRule{RuleScope: allow},
		ZapMessageSprintf:    ZapMessageSprintfRule{RuleScope: allow},
		ZapRootComponent:     ZapRootComponentRule{RuleScope: allow},
	}}

	assert.Empty(t, lintFixture(t, writeTestFixture(t), cfg))
}

func TestLintRespectsIgnoreComment(t *testing.T) {
	root := writeTestFixture(t)

	require.NoError(t, os.Remove(filepath.Join(root, "service/bad.go")))

	writeTestFile(t, root, "service/ignore.go", `package service

import "log"

func ignored() {
	// loglint:ignore stdlib_log_calls kmsg compatibility shim
	log.Printf("allowed")
}
`)

	assert.Empty(t, lintFixture(t, root, Config{}))
}

// lintFixture lints every package of the fixture module rooted at root, using
// the same entry point as the golangci-lint plugin.
func lintFixture(t *testing.T, root string, cfg Config) []Issue {
	t.Helper()

	cfg, err := NormalizeConfig(cfg, root)
	require.NoError(t, err)

	pkgs, err := packages.Load(&packages.Config{
		Mode:  packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
		Dir:   root,
		Env:   append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off"),
		Tests: false,
	}, "./...")
	require.NoError(t, err)

	issues := make([]Issue, 0)

	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			t.Fatalf("loading fixture package: %v", pkgErr)
		}

		pkgIssues, err := lintSyntaxFiles(cfg, pkg.Fset, pkg.TypesInfo, pkg.Syntax)
		require.NoError(t, err)

		issues = append(issues, pkgIssues...)
	}

	return issues
}

func writeTestFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	writeTestFile(t, root, "go.mod", `module github.com/siderolabs/talos

go 1.26.0

require go.uber.org/zap v1.0.0

replace go.uber.org/zap => ./stubs/zap
`)

	writeTestFile(t, root, "stubs/zap/go.mod", `module go.uber.org/zap

go 1.26.0
`)

	writeTestFile(t, root, "stubs/zap/zap.go", `package zap

type Field struct{}

type Logger struct{}

func (l *Logger) Debug(string, ...Field) {}
func (l *Logger) Info(string, ...Field)  {}
func (l *Logger) Warn(string, ...Field)  {}
func (l *Logger) Error(string, ...Field) {}
func (l *Logger) With(...Field) *Logger  { return l }

func String(string, string) Field { return Field{} }
func NewNop() *Logger             { return &Logger{} }
`)

	writeTestFile(t, root, "pkg/logging/logging.go", `package logging

import "go.uber.org/zap"

func ZapLogger() *zap.Logger {
	return zap.NewNop()
}

func Component(string) zap.Field {
	return zap.Field{}
}
`)

	writeTestFile(t, root, "service/bad.go", `package service

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/siderolabs/talos/pkg/logging"
	"go.uber.org/zap"
)

func run() {
	log.Printf("bad")

	logger := logging.ZapLogger()
	logger.Info("bad %s", zap.String("key", "value"))
	logger.Info(fmt.Sprintf("bad %s", "value"))

	_ = slog.Default()
}
`)

	writeTestFile(t, root, "service/good.go", `package service

import (
	"github.com/siderolabs/talos/pkg/logging"
	"go.uber.org/zap"
)

func good() {
	logger := logging.ZapLogger().With(logging.Component("service"))
	logger.Info("all good", zap.String("key", "value"))
}
`)

	return root
}

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()

	path := filepath.Join(root, relPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
