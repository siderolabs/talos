// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package loglinter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/tools/go/packages"
)

// Issue describes a single finding reported by log-linter.
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
	parents   map[ast.Node]ast.Node
	ignores   []ignoreComment
}

type ignoreComment struct {
	StartLine int
	EndLine   int
	Rules     map[string]struct{}
}

// Run executes log-linter using configPath and optional target filters.
//
//nolint:gocyclo
func Run(configPath string, rawTargets []string) ([]Issue, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	targets, err := normalizeTargets(cfg.Root, rawTargets)
	if err != nil {
		return nil, err
	}

	moduleRoots, err := discoverModuleRoots(cfg.Root)
	if err != nil {
		return nil, err
	}

	if len(moduleRoots) == 0 {
		return nil, fmt.Errorf("no go.mod files found under %s", cfg.Root)
	}

	issues := make([]Issue, 0)
	seen := map[string]struct{}{}

	for _, moduleRoot := range moduleRoots {
		loadModuleRoot, err := shouldLoadModule(cfg, moduleRoot, targets)
		if err != nil {
			return nil, err
		}

		if !loadModuleRoot {
			continue
		}

		pkgs, err := loadModule(moduleRoot)
		if err != nil {
			return nil, err
		}

		for _, pkg := range pkgs {
			fileIssues, err := lintPackage(cfg, pkg, targets)
			if err != nil {
				return nil, err
			}

			for _, issue := range fileIssues {
				key := issueKey(issue)
				if _, ok := seen[key]; ok {
					continue
				}

				seen[key] = struct{}{}

				issues = append(issues, issue)
			}
		}
	}

	sortIssues(issues)

	return issues, nil
}

//nolint:gocyclo
func shouldLoadModule(cfg Config, moduleRoot string, targets []string) (bool, error) {
	relPath, err := repoRelativePath(cfg.Root, moduleRoot)
	if err != nil {
		return false, err
	}

	if strings.HasPrefix(relPath, "../") || relPath == ".." {
		return false, nil
	}

	if matchAnyPattern(relPath, cfg.Exclude) {
		return false, nil
	}

	if len(targets) == 0 {
		return true, nil
	}

	if relPath == "." {
		return true, nil
	}

	for _, target := range targets {
		if target == "." || target == relPath || strings.HasPrefix(target, relPath+"/") {
			return true, nil
		}
	}

	return false, nil
}

func lintPackage(cfg Config, pkg *packages.Package, targets []string) ([]Issue, error) {
	if pkg == nil {
		return nil, nil
	}

	issues := make([]Issue, 0)

	for i, file := range pkg.Syntax {
		if i >= len(pkg.CompiledGoFiles) {
			continue
		}

		filename := pkg.CompiledGoFiles[i]

		relPath, err := repoRelativePath(cfg.Root, filename)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(relPath, "../") || relPath == ".." {
			continue
		}

		if !matchesTargets(relPath, targets) {
			continue
		}

		ctx := fileContext{
			config:    cfg,
			file:      file,
			fset:      pkg.Fset,
			typesInfo: pkg.TypesInfo,
			relPath:   relPath,
			parents:   buildParentMap(file),
			ignores:   collectIgnoreComments(file, pkg.Fset),
		}

		issues = append(issues, lintFile(ctx)...)
	}

	return issues, nil
}

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
			parents:   buildParentMap(file),
			ignores:   collectIgnoreComments(file, fset),
		}

		issues = append(issues, lintFile(ctx)...)
	}

	sortIssues(issues)

	return issues, nil
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

func lintFile(ctx fileContext) []Issue {
	issues := make([]Issue, 0) //nolint:prealloc

	issues = append(issues, lintSlogImports(ctx)...)
	issues = append(issues, lintStdlibLogCalls(ctx)...)
	issues = append(issues, lintZapMessageFormatting(ctx)...)
	issues = append(issues, lintZapMessageSprintf(ctx)...)
	issues = append(issues, lintZapRootComponent(ctx)...)

	return issues
}

func discoverModuleRoots(root string) ([]string, error) {
	var roots []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}

			return nil
		}

		if d.Name() == "go.mod" {
			roots = append(roots, filepath.Dir(path))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}

	slices.Sort(roots)

	return roots, nil
}

func loadModule(moduleRoot string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Dir:   moduleRoot,
		Tests: false,
	}

	cfg.Env = append(os.Environ(), "GOWORK=off")

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("loading packages in %s: %w", moduleRoot, err)
	}

	var loadErrs []string

	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			loadErrs = append(loadErrs, pkgErr.Error())
		}
	}

	if len(loadErrs) > 0 {
		slices.Sort(loadErrs)

		return nil, fmt.Errorf("package load errors in %s:\n%s", moduleRoot, strings.Join(loadErrs, "\n"))
	}

	return pkgs, nil
}

func normalizeTargets(root string, rawTargets []string) ([]string, error) {
	if len(rawTargets) == 0 {
		return nil, nil
	}

	targets := make([]string, 0, len(rawTargets))
	for _, target := range rawTargets {
		if target == "" {
			continue
		}

		if !filepath.IsAbs(target) {
			target = filepath.Join(root, target)
		}

		abs, err := filepath.Abs(target)
		if err != nil {
			return nil, fmt.Errorf("resolving target %q: %w", target, err)
		}

		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return nil, fmt.Errorf("resolving target %q relative to %s: %w", target, root, err)
		}

		rel = filepath.ToSlash(filepath.Clean(rel))
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return nil, fmt.Errorf("target %q is outside root %s", target, root)
		}

		targets = append(targets, rel)
	}

	return targets, nil
}

func repoRelativePath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolving %q relative to %s: %w", path, root, err)
	}

	return filepath.ToSlash(filepath.Clean(rel)), nil
}

func matchesTargets(relPath string, targets []string) bool {
	if len(targets) == 0 {
		return true
	}

	for _, target := range targets {
		if target == "." {
			return true
		}

		if relPath == target {
			return true
		}

		if strings.HasPrefix(relPath, target+"/") {
			return true
		}
	}

	return false
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
	const marker = "loglint:ignore"

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

func buildParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	stack := make([]ast.Node, 0)

	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]

			return false
		}

		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}

		stack = append(stack, node)

		return true
	})

	return parents
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

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		out[value] = struct{}{}
	}

	return out
}

func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return spec.Path.Value
	}

	return path
}
