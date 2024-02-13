// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package loader is used to load all packages from the given path.
package loader

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadPackages loads all packages from the given path.
func LoadPackages(pkgPath string) ([]*packages.Package, error) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles |
		packages.NeedCompiledGoFiles | packages.NeedImports |
		packages.NeedDeps | packages.NeedTypes | packages.NeedTypesSizes |
		packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load pkgs from '%s': %w", pkgPath, err)
	}

	if len(pkgs) == 0 {
		return nil, errors.New("no packages found")
	}

	err = collectErrors(pkgs)
	if err != nil {
		return nil, fmt.Errorf("error during processing '%s' packages: %w", pkgPath, err)
	}

	return pkgs, nil
}

func collectErrors(pkgs []*packages.Package) error {
	var (
		builder strings.Builder
		n       int
	)

	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			fmt.Fprintf(&builder, "Error '%d': %s\n", n, err)
			n++
		}
	})

	if n > 0 {
		return errors.New(builder.String())
	}

	return nil
}
