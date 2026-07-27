// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeimportlinter

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"
)

// versionSegment matches a bare Kubernetes API version identifier such as
// v1, v2, v1alpha1, or v1beta1.
var versionSegment = regexp.MustCompile(`^v[0-9]+((alpha|beta)[0-9]+)?$`)

func lintVersionedImports(ctx fileContext) []Issue {
	rule := ctx.config.Rules.VersionedImports
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	issues := make([]Issue, 0)

	for _, spec := range ctx.file.Imports {
		path := importPath(spec)
		if !matchesHost(path, rule.Hosts) {
			continue
		}

		// The effective identifier is the alias when present, otherwise the
		// imported package's real name. Only flag when that identifier is a bare
		// version: this correctly skips module version suffixes like
		// "k8s.io/klog/v2" whose package is named "klog", while catching genuine
		// API packages named "v1" that were left unaliased or aliased to "vN".
		name, ok := effectiveImportName(spec, ctx.typesInfo)
		if !ok || !versionSegment.MatchString(name) {
			continue
		}

		if ctx.ignored("versioned_imports", spec.Pos()) {
			continue
		}

		pos := ctx.position(spec.Pos())
		issues = append(issues, Issue{
			Rule:   "versioned_imports",
			Path:   pos.Filename,
			Line:   pos.Line,
			Column: pos.Column,
			Message: fmt.Sprintf(
				"versioned import %q must be aliased to a descriptive name (e.g. corev1); the bare version %q is not allowed",
				path,
				name,
			),
			Pos: spec.Pos(),
		})
	}

	return issues
}

// effectiveImportName returns the identifier the import is referenced by and
// whether it should be considered at all. Blank and dot imports return false.
func effectiveImportName(spec *ast.ImportSpec, info *types.Info) (string, bool) {
	if spec.Name != nil {
		switch spec.Name.Name {
		case "_", ".":
			return "", false
		default:
			return spec.Name.Name, true
		}
	}

	// No alias: resolve the real package name from type info.
	if info != nil {
		if pkgName, ok := info.Implicits[spec].(*types.PkgName); ok {
			return pkgName.Name(), true
		}
	}

	// Without type info, fall back to the last path segment.
	path := importPath(spec)

	return path[strings.LastIndex(path, "/")+1:], true
}

func matchesHost(path string, hosts []string) bool {
	for _, host := range hosts {
		if path == host || strings.HasPrefix(path, host+"/") {
			return true
		}
	}

	return false
}
