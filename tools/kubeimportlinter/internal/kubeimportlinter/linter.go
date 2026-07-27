// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeimportlinter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Issue describes a single finding reported by kubeimportlinter.
type Issue struct {
	Rule    string
	Path    string
	Line    int
	Column  int
	Message string
	Pos     token.Pos
}

type fileContext struct {
	config    Config
	file      *ast.File
	fset      *token.FileSet
	typesInfo *types.Info
	relPath   string
	ignores   []ignoreComment
}

type ignoreComment struct {
	StartLine int
	EndLine   int
	Rules     map[string]struct{}
}

// lintSyntaxFiles lints already-loaded files; used by the golangci-lint plugin.
func lintSyntaxFiles(cfg Config, fset *token.FileSet, typesInfo *types.Info, files []*ast.File) ([]Issue, error) {
	issues := make([]Issue, 0)

	for _, file := range files {
		filename := fset.Position(file.Pos()).Filename

		relPath, err := repoRelativePath(cfg.Root, filename)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(relPath, "../") || relPath == ".." {
			continue
		}

		ctx := fileContext{
			config:    cfg,
			file:      file,
			fset:      fset,
			typesInfo: typesInfo,
			relPath:   relPath,
			ignores:   collectIgnoreComments(file, fset),
		}

		issues = append(issues, lintFile(ctx)...)
	}

	sortIssues(issues)

	return issues, nil
}

func lintFile(ctx fileContext) []Issue {
	return lintVersionedImports(ctx)
}

func issueKey(issue Issue) string {
	return fmt.Sprintf("%s:%d:%d:%s:%s", issue.Path, issue.Line, issue.Column, issue.Rule, issue.Message)
}

func sortIssues(issues []Issue) {
	slices.SortFunc(issues, func(a, b Issue) int {
		if diff := strings.Compare(a.Path, b.Path); diff != 0 {
			return diff
		}

		if a.Line != b.Line {
			return a.Line - b.Line
		}

		if a.Column != b.Column {
			return a.Column - b.Column
		}

		if diff := strings.Compare(a.Rule, b.Rule); diff != 0 {
			return diff
		}

		return strings.Compare(a.Message, b.Message)
	})
}

func repoRelativePath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolving %q relative to %s: %w", path, root, err)
	}

	return filepath.ToSlash(filepath.Clean(rel)), nil
}

func (ctx fileContext) position(pos token.Pos) token.Position {
	position := ctx.fset.Position(pos)
	position.Filename = ctx.relPath

	return position
}

func (ctx fileContext) ignored(rule string, pos token.Pos) bool {
	issueLine := ctx.position(pos).Line

	for _, ignore := range ctx.ignores {
		if issueLine != ignore.EndLine && issueLine != ignore.EndLine+1 {
			continue
		}

		if _, ok := ignore.Rules[rule]; ok {
			return true
		}

		if _, ok := ignore.Rules["all"]; ok {
			return true
		}
	}

	return false
}

func collectIgnoreComments(file *ast.File, fset *token.FileSet) []ignoreComment {
	ignores := make([]ignoreComment, 0)

	for _, group := range file.Comments {
		rules := parseIgnoreRules(group.Text())
		if len(rules) == 0 {
			continue
		}

		start := fset.Position(group.Pos()).Line
		end := fset.Position(group.End()).Line

		ruleSet := make(map[string]struct{}, len(rules))
		for _, rule := range rules {
			ruleSet[rule] = struct{}{}
		}

		ignores = append(ignores, ignoreComment{
			StartLine: start,
			EndLine:   end,
			Rules:     ruleSet,
		})
	}

	return ignores
}

func parseIgnoreRules(text string) []string {
	const marker = "kubeimportlint:ignore"

	_, after, ok := strings.Cut(text, marker)
	if !ok {
		return nil
	}

	remainder := strings.TrimSpace(after)
	if remainder == "" {
		return nil
	}

	token := strings.Fields(remainder)[0]
	parts := strings.Split(token, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		out = append(out, part)
	}

	return out
}

func matchRuleScope(relPath string, globalExclude []string, scope RuleScope) bool {
	if matchAnyPattern(relPath, globalExclude) {
		return false
	}

	if matchAnyPattern(relPath, scope.Exclude) {
		return false
	}

	if len(scope.Include) > 0 && !matchAnyPattern(relPath, scope.Include) {
		return false
	}

	if matchAnyPattern(relPath, scope.Allow) {
		return false
	}

	return true
}

func matchAnyPattern(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, relPath)
		if err != nil {
			continue
		}

		if matched {
			return true
		}

		if !strings.ContainsAny(pattern, "*?[") && (relPath == pattern || strings.HasPrefix(relPath, pattern+"/")) {
			return true
		}
	}

	return false
}

func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return spec.Path.Value
	}

	return path
}
