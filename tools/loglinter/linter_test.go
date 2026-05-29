// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/siderolabs/talos/tools/loglinter"
)

func TestRunDisallowsAllRulesByDefault(t *testing.T) {
	root := writeTestFixture(t)

	configPath := filepath.Join(root, "log-linter.yaml")
	writeTestFile(t, root, "log-linter.yaml", ``)

	issues, err := main.Run(configPath, nil)
	require.NoError(t, err)

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
		assert.Equal(t, rules[rule], 1)
	}
}

func TestRunAllowsConfiguredExceptions(t *testing.T) {
	root := writeTestFixture(t)

	configPath := filepath.Join(root, "log-linter.yaml")
	writeTestFile(t, root, "log-linter.yaml", `rules:
  stdlib_log_calls:
    allow:
      - service/bad.go
  slog_imports:
    allow:
      - service/bad.go
  zap_message_formatting:
    allow:
      - service/bad.go
  zap_message_sprintf:
    allow:
      - service/bad.go
  zap_root_component:
    allow:
      - service/bad.go
`)

	issues, err := main.Run(configPath, nil)
	require.NoError(t, err)

	assert.Len(t, issues, 0)
}

func TestRunRespectsIgnoreComment(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "go.mod", `module example.com/test

go 1.26.0
`)

	writeTestFile(t, root, "service/ignore.go", `package service

import "log"

func run() {
	// loglint:ignore stdlib_log_calls kmsg compatibility shim
	log.Printf("allowed")
}
`)

	configPath := filepath.Join(root, "log-linter.yaml")
	writeTestFile(t, root, "log-linter.yaml", ``)

	issues, err := main.Run(configPath, nil)
	require.NoError(t, err)

	assert.Len(t, issues, 0)
}

func TestLoadConfigResolvesPathFromParentDirectories(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "configs/log-linter.yaml", ``)

	t.Chdir(filepath.Join(root, "configs"))

	cfg, err := main.LoadConfig("configs/log-linter.yaml")
	require.NoError(t, err)

	expectedRoot := filepath.Join(root, "configs")
	assert.Equal(t, expectedRoot, cfg.Root)
}

func TestLoadConfigExtractsGolangCISettings(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, ".golangci.yml", `version: "2"
linters:
  settings:
    custom:
      loglinter:
        type: module
        settings:
          exclude:
            - "vendor/**"
          rules:
            stdlib_log_calls:
              allow:
                - "allowed.go"
`)

	cfg, err := main.LoadConfig(filepath.Join(root, ".golangci.yml"))
	require.NoError(t, err)

	assert.Equal(t, root, cfg.Root)
	assert.Equal(t, []string{"vendor/**"}, cfg.Exclude)
	assert.Equal(t, []string{"allowed.go"}, cfg.Rules.StdlibLogCalls.Allow)
}

func TestLoadConfigLoadsStandaloneConfigDirectly(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "log-linter.yaml", `exclude:
  - "standalone/**"
`)

	cfg, err := main.LoadConfig(filepath.Join(root, "log-linter.yaml"))
	require.NoError(t, err)

	assert.Equal(t, root, cfg.Root)
	assert.Equal(t, []string{"standalone/**"}, cfg.Exclude)
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
