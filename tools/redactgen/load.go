// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// loadPackage loads a single (non-test) package with full type information.
func loadPackage(pkgPath string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %q: %w", pkgPath, err)
	}

	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected a single package to be loaded from %q, got %d", pkgPath, len(pkgs))
	}

	pkg := pkgs[0]

	if len(pkg.Errors) > 0 {
		var builder strings.Builder

		fmt.Fprintf(&builder, "errors loading package %q:", pkgPath)

		for _, pkgErr := range pkg.Errors {
			fmt.Fprintf(&builder, "\n  %s", pkgErr)
		}

		return nil, errors.New(builder.String())
	}

	return pkg, nil
}
