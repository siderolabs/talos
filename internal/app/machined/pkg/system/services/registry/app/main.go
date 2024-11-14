// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
)

func main() {
	if err := app(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func app() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	development, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("failed to create development logger: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	it := func(yield func(fs.StatFS) bool) {
		for _, root := range []string{"registry-cache-2", "registry-cache"} {
			if !yield(os.DirFS(filepath.Join(homeDir, root)).(fs.StatFS)) {
				return
			}
		}
	}

	return registry.NewService(registry.NewMultiPathFS(it), development).Run(ctx)
}
