// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package loglinter

import (
	"fmt"

	"golang.org/x/tools/go/analysis"
)

// AnalyzerName is the registered analyzer name used by the plugin and CLI.
const AnalyzerName = "loglinter"

// NewAnalyzer builds an analysis.Analyzer from a normalized config.
func NewAnalyzer(cfg Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             AnalyzerName,
		Doc:              "checks logging usage and message-shape conventions",
		RunDespiteErrors: true,
		Run: func(pass *analysis.Pass) (any, error) {
			issues, err := lintSyntaxFiles(cfg, pass.Fset, pass.TypesInfo, pass.Files)
			if err != nil {
				return nil, err
			}

			seen := map[string]struct{}{}

			for _, issue := range issues {
				key := issueKey(issue)
				if _, ok := seen[key]; ok {
					continue
				}

				seen[key] = struct{}{}

				pass.Report(analysis.Diagnostic{
					Pos:      issue.Pos,
					Category: issue.Rule,
					Message:  fmt.Sprintf("[%s] %s", issue.Rule, issue.Message),
				})
			}

			return nil, nil
		},
	}
}

// NewAnalyzerFromConfigPath loads configPath and builds an analyzer from it.
func NewAnalyzerFromConfigPath(configPath string) (*analysis.Analyzer, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return NewAnalyzer(cfg), nil
}
