// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package loglinter

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

func lintSlogImports(ctx fileContext) []Issue {
	rule := ctx.config.Rules.SlogImports
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	issues := make([]Issue, 0)

	for _, spec := range ctx.file.Imports {
		if importPath(spec) != "log/slog" {
			continue
		}

		if ctx.ignored("slog_imports", spec.Pos()) {
			continue
		}

		pos := ctx.position(spec.Pos())
		issues = append(issues, Issue{
			Rule:    "slog_imports",
			Path:    pos.Filename,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: "log/slog import is disallowed in this scope; keep slog only at compatibility boundaries or use zap",
			Pos:     spec.Pos(),
		})
	}

	return issues
}

func lintStdlibLogCalls(ctx fileContext) []Issue {
	rule := ctx.config.Rules.StdlibLogCalls
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	banned := stringSet(rule.Functions)
	issues := make([]Issue, 0)

	ast.Inspect(ctx.file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		qualified := qualifiedFunctionName(call, ctx.typesInfo)
		if !strings.HasPrefix(qualified, "log.") {
			return true
		}

		funcName := strings.TrimPrefix(qualified, "log.")
		if _, ok = banned[funcName]; !ok {
			return true
		}

		if ctx.ignored("stdlib_log_calls", call.Pos()) {
			return true
		}

		pos := ctx.position(call.Pos())
		issues = append(issues, Issue{
			Rule:    "stdlib_log_calls",
			Path:    pos.Filename,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: fmt.Sprintf("stdlib log.%s is disallowed in this scope; use zap for service-plane code or allowlist the file", funcName),
			Pos:     call.Pos(),
		})

		return true
	})

	return issues
}

func lintZapMessageFormatting(ctx fileContext) []Issue {
	rule := ctx.config.Rules.ZapMessageFormatting
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	methods := stringSet(rule.Methods)
	issues := make([]Issue, 0)

	ast.Inspect(ctx.file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		messageExpr, methodName, ok := zapMessageArgument(call, ctx.typesInfo, methods)
		if !ok {
			return true
		}

		message, ok := constantStringValue(messageExpr, ctx.typesInfo)
		if !ok {
			return true
		}

		directive, ok := firstFormattingDirective(message)
		if !ok {
			return true
		}

		if ctx.ignored("zap_message_formatting", messageExpr.Pos()) {
			return true
		}

		pos := ctx.position(messageExpr.Pos())
		issues = append(issues, Issue{
			Rule:   "zap_message_formatting",
			Path:   pos.Filename,
			Line:   pos.Line,
			Column: pos.Column,
			Message: fmt.Sprintf(
				"zap logger method %s uses formatting directive %q in the message; keep the message constant and move data into fields",
				methodName,
				directive,
			),
			Pos: messageExpr.Pos(),
		})

		return true
	})

	return issues
}

func lintZapMessageSprintf(ctx fileContext) []Issue {
	rule := ctx.config.Rules.ZapMessageSprintf
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	methods := stringSet(rule.Methods)
	functions := stringSet(rule.Functions)
	issues := make([]Issue, 0)

	ast.Inspect(ctx.file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		messageExpr, methodName, ok := zapMessageArgument(call, ctx.typesInfo, methods)
		if !ok {
			return true
		}

		messageCall, ok := unwrapParens(messageExpr).(*ast.CallExpr)
		if !ok {
			return true
		}

		qualified := qualifiedFunctionName(messageCall, ctx.typesInfo)
		if _, ok = functions[qualified]; !ok {
			return true
		}

		if ctx.ignored("zap_message_sprintf", messageExpr.Pos()) {
			return true
		}

		pos := ctx.position(messageExpr.Pos())
		issues = append(issues, Issue{
			Rule:    "zap_message_sprintf",
			Path:    pos.Filename,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: fmt.Sprintf("zap logger method %s builds its message with %s; use a constant message and structured fields instead", methodName, qualified),
			Pos:     messageExpr.Pos(),
		})

		return true
	})

	return issues
}

func lintZapRootComponent(ctx fileContext) []Issue {
	rule := ctx.config.Rules.ZapRootComponent
	if !rule.Enabled() || !matchRuleScope(ctx.relPath, ctx.config.Exclude, rule.RuleScope) {
		return nil
	}

	constructors := stringSet(rule.Constructors)
	componentCalls := stringSet(rule.ComponentCalls)
	issues := make([]Issue, 0)

	ast.Inspect(ctx.file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		qualified := qualifiedFunctionName(call, ctx.typesInfo)
		if _, ok = constructors[qualified]; !ok {
			return true
		}

		if hasComponentWrapper(call, ctx.parents, ctx.typesInfo, componentCalls) {
			return true
		}

		if ctx.ignored("zap_root_component", call.Pos()) {
			return true
		}

		pos := ctx.position(call.Pos())
		issues = append(issues, Issue{
			Rule:    "zap_root_component",
			Path:    pos.Filename,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: fmt.Sprintf("logger constructed with %s is not wrapped with .With(logging.Component(...)) in the same expression", qualified),
			Pos:     call.Pos(),
		})

		return true
	})

	return issues
}

func qualifiedFunctionName(call *ast.CallExpr, info *types.Info) string {
	if info == nil {
		return ""
	}

	sel, ok := unwrapParens(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	pkgIdent, ok := unwrapParens(sel.X).(*ast.Ident)
	if !ok {
		return ""
	}

	pkgName, ok := info.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgName.Imported() == nil {
		return ""
	}

	return pkgName.Imported().Path() + "." + sel.Sel.Name
}

func zapMessageArgument(call *ast.CallExpr, info *types.Info, methods map[string]struct{}) (ast.Expr, string, bool) {
	if info == nil {
		return nil, "", false
	}

	sel, ok := unwrapParens(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}

	method := sel.Sel.Name
	if _, ok = methods[method]; !ok {
		return nil, "", false
	}

	selection := info.Selections[sel]
	if selection == nil || !isZapLoggerType(selection.Recv()) {
		return nil, "", false
	}

	messageIndex := 0
	if method == "Check" {
		messageIndex = 1
	}

	if len(call.Args) <= messageIndex {
		return nil, "", false
	}

	return call.Args[messageIndex], method, true
}

func isZapLoggerType(typ types.Type) bool {
	named := namedType(typ)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == "go.uber.org/zap" && named.Obj().Name() == "Logger"
}

func namedType(typ types.Type) *types.Named {
	switch t := typ.(type) {
	case *types.Pointer:
		return namedType(t.Elem())
	case *types.Named:
		return t
	default:
		return nil
	}
}

func constantStringValue(expr ast.Expr, info *types.Info) (string, bool) {
	expr = unwrapParens(expr)

	if info != nil {
		if tv, ok := info.Types[expr]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
			return constant.StringVal(tv.Value), true
		}
	}

	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return value, true
}

func unwrapParens(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}

		expr = paren.X
	}
}

//nolint:gocyclo,cyclop
func firstFormattingDirective(message string) (string, bool) {
	for i := 0; i < len(message); i++ {
		if message[i] != '%' {
			continue
		}

		if i+1 < len(message) && message[i+1] == '%' {
			i++

			continue
		}

		j := i + 1

		if j < len(message) && message[j] == '[' {
			j++
			for j < len(message) && message[j] >= '0' && message[j] <= '9' {
				j++
			}

			if j >= len(message) || message[j] != ']' {
				continue
			}

			j++
		}

		for j < len(message) && strings.ContainsRune("#+- 0", rune(message[j])) {
			j++
		}

		if j < len(message) && message[j] == '*' {
			j++
		} else {
			for j < len(message) && message[j] >= '0' && message[j] <= '9' {
				j++
			}
		}

		if j < len(message) && message[j] == '.' {
			j++
			if j < len(message) && message[j] == '*' {
				j++
			} else {
				for j < len(message) && message[j] >= '0' && message[j] <= '9' {
					j++
				}
			}
		}

		if j < len(message) && strings.ContainsRune("vTtbcdoOqxXUeEfFgGspw", rune(message[j])) {
			return message[i : j+1], true
		}
	}

	return "", false
}

//nolint:gocyclo
func hasComponentWrapper(call *ast.CallExpr, parents map[ast.Node]ast.Node, info *types.Info, componentCalls map[string]struct{}) bool {
	var current ast.Node = call

	for {
		parent, ok := parents[current]
		if !ok {
			return false
		}

		switch node := parent.(type) {
		case *ast.SelectorExpr:
			if node.X != current {
				return false
			}

			current = node
		case *ast.CallExpr:
			if node.Fun != current {
				return false
			}

			sel, ok := unwrapParens(node.Fun).(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "With" {
				for _, arg := range node.Args {
					argCall, ok := unwrapParens(arg).(*ast.CallExpr)
					if !ok {
						continue
					}

					qualified := qualifiedFunctionName(argCall, info)
					if _, ok = componentCalls[qualified]; ok {
						return true
					}
				}
			}

			current = node
		default:
			return false
		}
	}
}
