// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main implements the standalone log-linter CLI.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/siderolabs/talos/tools/loglinter/internal/loglinter"
)

func main() {
	var configPath string

	flag.StringVar(&configPath, "config", "", "path to YAML config file")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -config path/to/log-linter.yaml [file-or-dir ...]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Targets are optional repo-relative or absolute file/directory filters.")
		fmt.Fprintln(flag.CommandLine.Output(), "Patterns and rule scopes come from the YAML config.")
		flag.PrintDefaults()
	}

	flag.Parse()

	if configPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	issues, err := loglinter.Run(configPath, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	for _, issue := range issues {
		fmt.Printf("%s:%d:%d: [%s] %s\n", issue.Path, issue.Line, issue.Column, issue.Rule, issue.Message)
	}

	if len(issues) > 0 {
		fmt.Printf("%d issue(s)\n", len(issues))
		os.Exit(1)
	}
}
