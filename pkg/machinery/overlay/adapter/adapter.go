// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package adapter provides an adapter for the overlay installer.
package adapter

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/overlay"
)

// Execute executes the overlay installer.
func Execute[T any](ctx context.Context, installer overlay.Installer[T]) {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, "missing command")

		os.Exit(1)
	}

	switch os.Args[1] {
	case "install":
		install(ctx, installer)
	case "get-options":
		getOptions(ctx, installer)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s", os.Args[1])

		os.Exit(1)
	}
}

func getOptions[T any](ctx context.Context, installer overlay.Installer[T]) {
	var opts T

	withErrorHandler(yaml.NewDecoder(os.Stdin).Decode(&opts))

	opt, err := installer.GetOptions(ctx, opts)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())

		os.Exit(1)
	}

	withErrorHandler(yaml.NewEncoder(os.Stdout).Encode(opt))
}

func install[T any](ctx context.Context, installer overlay.Installer[T]) {
	var opts overlay.InstallOptions[T]

	withErrorHandler(yaml.NewDecoder(os.Stdin).Decode(&opts))

	withErrorHandler(installer.Install(ctx, opts))
}

func withErrorHandler(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())

		os.Exit(1)
	}
}
