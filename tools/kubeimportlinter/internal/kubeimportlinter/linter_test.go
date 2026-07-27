// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeimportlinter //nolint:testpackage // exercises the unexported plugin entry point

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestLintFlagsBareVersionedImports(t *testing.T) {
	issues := lintFixture(t, writeTestFixture(t), Config{})

	// bad.go: bare `v1` default import + explicit `v1beta1` alias = 2 findings.
	require.Len(t, issues, 2)

	for _, issue := range issues {
		assert.Equal(t, "versioned_imports", issue.Rule)
		assert.Equal(t, "service/bad.go", issue.Path)
	}
}

func TestLintAllowsConfiguredExceptions(t *testing.T) {
	cfg := Config{Rules: Rules{
		VersionedImports: VersionedImportsRule{RuleScope: RuleScope{Allow: []string{"service/bad.go"}}},
	}}

	assert.Empty(t, lintFixture(t, writeTestFixture(t), cfg))
}

func TestLintRespectsIgnoreComment(t *testing.T) {
	root := writeTestFixture(t)

	require.NoError(t, os.Remove(filepath.Join(root, "service/bad.go")))

	writeTestFile(t, root, "service/ignore.go", `package service

import (
	// kubeimportlint:ignore versioned_imports intentional bare alias
	v1 "k8s.io/api/core/v1"
)

var _ = v1.Pod{}
`)

	assert.Empty(t, lintFixture(t, root, Config{}))
}

func TestLintAllowsDescriptiveAliasAndNonKube(t *testing.T) {
	root := writeTestFixture(t)

	require.NoError(t, os.Remove(filepath.Join(root, "service/bad.go")))

	writeTestFile(t, root, "service/aliases.go", `package service

import (
	corev1 "k8s.io/api/core/v1"

	// module version suffix, package is named klog
	"k8s.io/klog/v2"

	// not a kubernetes host, must be ignored
	v1 "example.com/pkg/v1"

	// blank imports carry no usable identifier
	_ "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	_ = corev1.Pod{}
	_ = v1.X{}
	_ = klog.Enabled
)
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

require (
	example.com/pkg v0.0.0
	k8s.io/api v0.0.0
	k8s.io/klog/v2 v2.0.0
	sigs.k8s.io/gateway-api v0.0.0
)

replace (
	example.com/pkg => ./stubs/pkg
	k8s.io/api => ./stubs/k8sapi
	k8s.io/klog/v2 => ./stubs/klog
	sigs.k8s.io/gateway-api => ./stubs/gatewayapi
)
`)

	writeTestFile(t, root, "stubs/k8sapi/go.mod", "module k8s.io/api\n\ngo 1.26.0\n")
	writeTestFile(t, root, "stubs/k8sapi/core/v1/types.go", "package v1\n\ntype Pod struct{}\n")
	writeTestFile(t, root, "stubs/klog/go.mod", "module k8s.io/klog/v2\n\ngo 1.26.0\n")
	writeTestFile(t, root, "stubs/klog/klog.go", "package klog\n\nvar Enabled bool\n")
	writeTestFile(t, root, "stubs/gatewayapi/go.mod", "module sigs.k8s.io/gateway-api\n\ngo 1.26.0\n")
	writeTestFile(t, root, "stubs/gatewayapi/apis/v1beta1/types.go", "package v1beta1\n\ntype Gateway struct{}\n")
	writeTestFile(t, root, "stubs/pkg/go.mod", "module example.com/pkg\n\ngo 1.26.0\n")
	writeTestFile(t, root, "stubs/pkg/v1/types.go", "package v1\n\ntype X struct{}\n")

	writeTestFile(t, root, "service/bad.go", `package service

import (
	"k8s.io/api/core/v1"

	v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	_ = v1.Pod{}
	_ = v1beta1.Gateway{}
)
`)

	writeTestFile(t, root, "service/good.go", `package service

import (
	corev1 "k8s.io/api/core/v1"
)

var _ = corev1.Pod{}
`)

	return root
}

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()

	path := filepath.Join(root, relPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
