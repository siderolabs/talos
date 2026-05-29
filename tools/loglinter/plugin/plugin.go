// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package plugin exposes log-linter as a golangci-lint module plugin.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/siderolabs/talos/tools/loglinter/internal/loglinter"
)

// Name is the registered golangci-lint linter name for this module plugin.
const Name = loglinter.AnalyzerName

//nolint:gochecknoinits // golangci-lint module plugins register themselves via init.
func init() {
	register.Plugin(Name, New)
}

// Settings defines the golangci-lint module plugin configuration.
type Settings struct {
	Exclude []string        `json:"exclude"`
	Rules   loglinter.Rules `json:"rules"`
}

// Linter adapts log-linter to the golangci-lint module plugin interface.
type Linter struct {
	config loglinter.Config
}

// New constructs the golangci-lint module plugin.
func New(conf any) (register.LinterPlugin, error) {
	settings, err := register.DecodeSettings[Settings](conf)
	if err != nil {
		return nil, err
	}

	baseDir, err := detectConfigBaseDir()
	if err != nil {
		return nil, err
	}

	cfg, err := loglinter.NormalizeConfig(loglinter.Config{
		Exclude: settings.Exclude,
		Rules:   settings.Rules,
	}, baseDir)
	if err != nil {
		return nil, err
	}

	return &Linter{config: cfg}, nil
}

// BuildAnalyzers returns the analyzers exposed by this module plugin.
func (l *Linter) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{loglinter.NewAnalyzer(l.config)}, nil
}

// GetLoadMode reports the go/packages load mode required by the plugin.
func (l *Linter) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

func detectConfigBaseDir() (string, error) {
	if configPath, ok := configPathFromArgs(os.Args[1:]); ok {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("resolving golangci-lint config path %q: %w", configPath, err)
		}

		return filepath.Dir(absPath), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current working directory: %w", err)
	}

	if dir, _, ok := loglinter.SearchUpward(cwd, ".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"); ok {
		return dir, nil
	}

	return cwd, nil
}

func configPathFromArgs(args []string) (string, bool) {
	for i := range args {
		arg := args[i]

		switch {
		case arg == "--config" || arg == "-c":
			if i+1 < len(args) {
				return args[i+1], true
			}
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config="), true
		case strings.HasPrefix(arg, "-c="):
			return strings.TrimPrefix(arg, "-c="), true
		}
	}

	return "", false
}
