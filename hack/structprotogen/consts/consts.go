// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package consts is used to find all consts with expected tag in the given AST.
package consts

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const tag = "structprotogen:gen_enum"

// FindIn looks up all const blocks with the specific comment in the given packages.
//
//nolint:gocyclo
func FindIn(pkgs []*packages.Package) (ConstBlocks, error) {
	var result ConstBlocks

	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			for _, constBlock := range findGenDecls(f.Decls) {
				var consts []Constant

				var typeData typeData

				valueSpecs := filter(constBlock.Specs, func(spec ast.Spec) (*ast.ValueSpec, bool) {
					valueSpec, ok := spec.(*ast.ValueSpec)

					return valueSpec, ok
				})

				for _, valueSpec := range valueSpecs {
					for _, name := range valueSpec.Names {
						def := pkg.TypesInfo.Defs[name]

						if !def.Exported() {
							continue
						}

						td, err := getTypeData(pkg.Syntax, def.Type())
						if err != nil {
							return nil, fmt.Errorf("%s: const named '%s': %w", pkg.PkgPath, def.Name(), err)
						}

						if typeData.name == "" {
							typeData = td
						} else if typeData.name != td.name {
							return nil, fmt.Errorf("const type mismatch: %s != %s", typeData.name, def.Type().String())
						}

						val, err := getValue(def)
						if err != nil {
							return nil, err
						}

						consts = append(consts, Constant{
							Name:         name.Name,
							Value:        val,
							CommentLines: commentToStrings(valueSpec.Doc),
						})
					}
				}

				if len(consts) == 0 {
					return nil, fmt.Errorf("%s: const block with no exported consts", pkg.PkgPath)
				}

				result = append(result, ConstBlock{
					TypeName:     typeData.name,
					TypePkg:      typeData.pkgName,
					TypePath:     typeData.pkgPath,
					CommentLines: typeData.comments,
					Consts:       consts,
				})
			}
		}
	}

	return result, nil
}

func getValue(obj types.Object) (string, error) {
	result := obj.(*types.Const).Val().String()

	_, err := strconv.Atoi(result)
	if err != nil {
		return "", fmt.Errorf("value %s is not an integer: %s", obj.Name(), result)
	}

	return result, nil
}

func findGenDecls(decl []ast.Decl) []*ast.GenDecl {
	return filter(decl, func(decl ast.Decl) (*ast.GenDecl, bool) {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok ||
			genDecl.Tok != token.CONST ||
			genDecl.Lparen == token.NoPos || // single const declaration, ignore
			len(genDecl.Specs) == 0 {
			return nil, false
		}

		strs := commentToStrings(genDecl.Doc)
		if len(strs) == 0 {
			return nil, false
		}

		if findInStrings(strs, tag) == -1 {
			return nil, false
		}

		return genDecl, true
	})
}

// findInStrings finds a string in a list of strings.
func findInStrings(strs []string, find string) int {
	for i, str := range strs {
		if strings.Contains(str, find) {
			return i
		}
	}

	return -1
}

func getTypeData(files []*ast.File, t types.Type) (typeData, error) {
	switch t := t.(type) {
	case *types.Named:
		commentGroup, err := findTypeComment(files, t.Obj().Name())
		if err != nil {
			return typeData{}, err
		}

		return typeData{
			name:     t.Obj().Name(),
			pkgName:  t.Obj().Pkg().Name(),
			pkgPath:  t.Obj().Pkg().Path(),
			comments: commentToStrings(commentGroup),
		}, nil
	default:
		return typeData{}, fmt.Errorf("unsupported type: %s", t.String())
	}
}

type typeData struct {
	name     string
	pkgName  string
	pkgPath  string
	comments []string
}

func findTypeComment(files []*ast.File, typeName string) (*ast.CommentGroup, error) {
	for _, f := range files {
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok ||
				genDecl.Tok != token.TYPE ||
				len(genDecl.Specs) == 0 {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				if typeSpec.Name.Name == typeName {
					return genDecl.Doc, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("type %s not found", typeName)
}

// commentToStrings converts a list of comments to a list of strings.
func commentToStrings(doc *ast.CommentGroup) []string {
	if doc == nil {
		return nil
	}

	result := make([]string, 0, len(doc.List))
	for _, c := range doc.List {
		result = append(result, c.Text)
	}

	return result
}

// ConstBlock is a block of constants.
type ConstBlock struct {
	TypeName     string
	TypePkg      string
	TypePath     string
	CommentLines []string
	Consts       []Constant
}

// ProtoMessageName returns the name of the proto message for this const block.
func (b *ConstBlock) ProtoMessageName() string {
	return strings.Title(b.TypePkg) + strings.Title(b.TypeName) //nolint:staticcheck
}

// Constant represents a constant.
type Constant struct {
	Name         string
	Value        string
	CommentLines []string
}

// ConstBlocks is a slice of ConstBlock.
type ConstBlocks []ConstBlock

// FormatProtoFile generates proto file from the list of ConstBlocks.
func (b *ConstBlocks) FormatProtoFile(w io.Writer) error {
	fmt.Fprint(w, "syntax = \"proto3\";\n\n")
	fmt.Fprint(w, "package talos.resource.definitions.enums;\n\n")
	fmt.Fprint(w, `option go_package = "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/enums";`+"\n")
	fmt.Fprint(w, `option java_package = "dev.talos.api.resource.definitions.enums";`+"\n\n")

	for _, block := range *b {
		for _, comment := range block.CommentLines {
			fmt.Fprintln(w, strings.ReplaceAll(comment, " "+block.TypeName+" ", " "+block.ProtoMessageName()+" "))
		}

		fmt.Fprintf(w, "enum %s {\n", block.ProtoMessageName())

		hasZeroNotFirstConstValue := slices.IndexFunc(block.Consts, func(c Constant) bool { return c.Value == "0" }) > 0

		if hasDuplicates(block.Consts, func(c Constant) string { return c.Value }) || hasZeroNotFirstConstValue {
			fmt.Fprintln(w, "  option allow_alias = true;")
		}

		for i, constant := range block.Consts {
			for _, comment := range constant.CommentLines {
				fmt.Fprintln(w, " ", comment)
			}

			if i == 0 && constant.Value != "0" {
				fmt.Fprintf(w,
					"  %s_%s_UNSPECIFIED = 0;\n",
					strings.ToUpper(block.TypePkg),
					strings.ToUpper(block.TypeName),
				)
			}

			fmt.Fprintf(w, "  %s = %s;\n", toCapitalSnakeCase(constant.Name), constant.Value)
		}

		fmt.Fprintf(w, "}\n\n")
	}

	return nil
}

// HaveType returns true if the list of ConstBlocks contains a block with the given type.
func (b *ConstBlocks) HaveType(pkgPath, typeName string) bool {
	_, ok := b.Get(pkgPath, typeName)

	return ok
}

// Get returns a ConstBlock for a given type.
func (b *ConstBlocks) Get(pkgPath, typeName string) (ConstBlock, bool) {
	for _, block := range *b {
		if block.TypePath == pkgPath && block.TypeName == typeName {
			return block, true
		}
	}

	return ConstBlock{}, false
}

func hasDuplicates[T any, K comparable](slc []T, fn func(T) K) bool {
	seen := make(map[K]struct{}, len(slc))

	for _, elem := range slc {
		k := fn(elem)
		if _, ok := seen[k]; ok {
			return true
		}

		seen[k] = struct{}{}
	}

	return false
}

// toCapitalSnakeCase converts a string to a capital snake case.
func toCapitalSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	snake = strings.ToUpper(snake)

	// special case for "SomethingsIps"
	if strings.HasSuffix(snake, "_i_ps") {
		snake = strings.TrimSuffix(snake, "_i_ps") + "_ips"
	}

	return snake
}

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func filter[T, V any](slc []T, f func(n T) (V, bool)) []V {
	var result []V

	for _, v := range slc {
		res, ok := f(v)
		if ok {
			result = append(result, res)
		}
	}

	return result
}
