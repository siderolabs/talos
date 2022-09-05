// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ast is used to find all structs with expected tag in the given AST.
package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"github.com/fatih/structtag"
	"golang.org/x/tools/go/packages"
)

// FindAllTaggedStructs extracts all structs with comments which contain tag in the given AST.
func FindAllTaggedStructs(pkgs []*packages.Package) TaggedStructs {
	result := TaggedStructs{}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			structs := findAllStructs(file)

			for _, structDef := range structs {
				result.Add(pkg.PkgPath, structDef.Struct.Name.Name, TaggedStruct{
					Comments: formatComments(structDef.Comment),
					Fields:   structDef.Fields,
				})
			}
		}
	}

	return result
}

// TaggedStructs is a map of tagged struct declarations to their data.
type TaggedStructs map[StructDecl]TaggedStruct

// TaggedStruct contains struct comments and fields.
type TaggedStruct struct {
	Comments []string
	Fields   Fields
}

// Get returns the struct data for given pkg and pkgPath.
func (t TaggedStructs) Get(pkg, name string) (TaggedStruct, bool) {
	val, ok := t[StructDecl{pkg, name}]

	return val, ok
}

// Add adds the given struct data to the map.
func (t TaggedStructs) Add(pkg, name string, structData TaggedStruct) {
	t[StructDecl{Pkg: pkg, Name: name}] = structData
}

// findAllStructs finds all structs with comments in the given AST.
func findAllStructs(f ast.Node) []structData {
	var result []structData

	ast.Inspect(f, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		typeSpecs := filter(genDecl.Specs, typeSpecFilter)

		switch len(typeSpecs) {
		case 0:
			return false
		case 1:
			switch {
			case isTarget(genDecl.Doc):
				fields := getStructFieldsWithTags(typeSpecs[0])

				result = append(result, structData{
					Struct:  typeSpecs[0],
					Comment: genDecl.Doc,
					Fields:  fields,
				})
			case isTarget(typeSpecs[0].Doc):
				fields := getStructFieldsWithTags(typeSpecs[0])

				result = append(result, structData{
					Struct:  typeSpecs[0],
					Comment: typeSpecs[0].Doc,
					Fields:  fields,
				})
			}

			return false
		default:
			for _, typeSpec := range typeSpecs {
				if isTarget(typeSpec.Doc) {
					fields := getStructFieldsWithTags(typeSpec)

					result = append(result, structData{
						Struct:  typeSpec,
						Comment: typeSpec.Doc,
						Fields:  fields,
					})
				}
			}

			return false
		}
	})

	return result
}

type structData struct {
	Struct  *ast.TypeSpec
	Comment *ast.CommentGroup
	Fields  Fields
}

func typeSpecFilter(n ast.Spec) (*ast.TypeSpec, bool) {
	typeSpec, ok := n.(*ast.TypeSpec)
	if !ok {
		return nil, false
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil, false
	}

	if structType.Fields.NumFields() == 0 {
		return nil, false
	}

	if !isCapitalCase(typeSpec.Name.Name) {
		return nil, false
	}

	return typeSpec, true
}

// isCapitalCase returns true if the given string is in capital case.
func isCapitalCase(s string) bool {
	return len(s) > 0 && unicode.IsUpper(rune(s[0]))
}

func isTarget(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}

	for _, c := range doc.List {
		if isTargetComment(c.Text) {
			return true
		}
	}

	return false
}

func isTargetComment(str string) bool {
	return str == "//gotagsrewrite:gen"
}

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

// StructDecl is a struct declaration.
type StructDecl struct {
	Pkg  string
	Name string
}

func formatComments(comment *ast.CommentGroup) []string {
	if comment == nil {
		return nil
	}

	result := make([]string, 0, len(comment.List))

	for i := range comment.List {
		if len(comment.List)-1 >= i+1 && isTargetComment(comment.List[i+1].Text) {
			continue
		}

		if isTargetComment(comment.List[i].Text) {
			continue
		}

		result = append(result, comment.List[i].Text)
	}

	return result
}

// Fields represents a struct field and its protobuf number.
type Fields map[string]int

// getStructFieldsWithTags returns all fields of the given struct with their tags.
func getStructFieldsWithTags(structDecl *ast.TypeSpec) Fields {
	result := Fields{}

	structType := structDecl.Type.(*ast.StructType) //nolint:errcheck

	for _, field := range structType.Fields.List {
		if field.Names == nil {
			continue
		}

		for _, name := range field.Names {
			if field.Tag == nil {
				continue
			}

			tagValue := strings.Trim(field.Tag.Value, "`")

			tags, err := structtag.Parse(tagValue)
			if err != nil {
				panic(fmt.Errorf("invalid tag: field '%s', tag '%s': %w", name, tagValue, err))
			}

			tag, err := tags.Get("protobuf")
			if err != nil {
				panic(fmt.Errorf("cannot find protobuf tag: field '%s', tag '%s': %w", name, tagValue, err))
			}

			num, err := strconv.Atoi(tag.Name)
			if err != nil {
				panic(fmt.Errorf("invalid protobuf tag: field '%s', tag '%s': %w", name, tagValue, err))
			}

			result[name.Name] = num
		}
	}

	return result
}
