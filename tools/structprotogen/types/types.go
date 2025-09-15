// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package types contains utils to work with types.
package types

//nolint:gci
import (
	"errors"
	"fmt"
	"go/types"
	"path"
	"strings"

	"golang.org/x/tools/go/packages"
	"gopkg.in/typ.v4/slices"

	"github.com/siderolabs/talos/tools/structprotogen/ast"
	"github.com/siderolabs/talos/tools/structprotogen/sliceutil"
)

// PkgDecl is a struct which contains package path and tagged struct declarations.
type PkgDecl struct {
	path string

	decl []types.Object
}

func pkgCmp(left, right *PkgDecl) int { return strings.Compare(left.path, right.path) }

// FindPkgDecls finds all declarations in the given packages.
func FindPkgDecls(taggedStructs ast.TaggedStructs, loadedPkgs []*packages.Package) (slices.Sorted[*PkgDecl], error) {
	result := slices.NewSortedCompare([]*PkgDecl{}, pkgCmp)

	for _, loadedPkg := range loadedPkgs {
		forEachTaggedStruct(taggedStructs, loadedPkg, func(decl types.Object) {
			v := sliceutil.GetOrAdd(&result, &PkgDecl{
				path: decl.Pkg().Path(),
			})

			v.decl = append(v.decl, decl)
		})
	}

	if result.Len() == 0 {
		return slices.Sorted[*PkgDecl]{}, errors.New("no definitions found")
	}

	return result, nil
}

func forEachTaggedStruct(taggedStructs ast.TaggedStructs, pkg *packages.Package, f func(decl types.Object)) {
	scope := pkg.Types.Scope()

	for _, name := range scope.Names() {
		_, ok := taggedStructs.Get(pkg.PkgPath, name)
		if !ok {
			continue
		}

		obj := scope.Lookup(name)

		// This is special resource - ignore it.
		if pkg.PkgPath == "github.com/siderolabs/talos/pkg/machinery/resources/network" && name == "DeviceConfigSpecSpec" {
			continue
		}

		f(obj)
	}
}

// Type is a struct which contains type pkg, name, comments and fields.
type Type struct {
	Pkg      string
	Name     string
	Comments []string

	isInit bool
	fields slices.Sorted[FieldData]
}

// PkgName returns a package name for the given type.
func (t *Type) PkgName() string {
	return path.Base(t.Pkg)
}

// Fields returns a list of fields for the given type.
func (t *Type) Fields() *slices.Sorted[FieldData] {
	if !t.isInit {
		t.fields = slices.NewSortedCompare([]FieldData{}, fieldCmp)
		t.isInit = true
	}

	return &t.fields
}

func pkgTypeCmp(left, right *Type) int {
	pathCmp := strings.Compare(left.Pkg, right.Pkg)
	if pathCmp != 0 {
		return pathCmp
	}

	return strings.Compare(left.Name, right.Name)
}

// FieldData is a struct which contains field name, proto num and type data.
type FieldData struct {
	Name     string
	Num      int
	TypeData *types.Var
}

func fieldCmp(left, right FieldData) int { return strings.Compare(left.Name, right.Name) }

// ParseDeclsData parses all declarations and returns a list of packages with proper types.
func ParseDeclsData(sortedPkgs slices.Sorted[*PkgDecl], taggedStructs ast.TaggedStructs) (slices.Sorted[*Type], error) {
	result := slices.NewSortedCompare([]*Type{}, pkgTypeCmp)

	for i := 0; i < sortedPkgs.Len(); i++ {
		pkg := sortedPkgs.Get(i)

		for _, decl := range pkg.decl {
			structName := decl.Name()

			structType, ok := decl.Type().Underlying().(*types.Struct)
			if !ok {
				return slices.Sorted[*Type]{}, fmt.Errorf("type %s is not a struct", structName)
			}

			taggedStruct, ok := taggedStructs.Get(pkg.path, structName)
			if !ok {
				return slices.Sorted[*Type]{}, fmt.Errorf("type %s is unknown struct", structName)
			}

			for j := 0; j < structType.NumFields(); j++ {
				field := structType.Field(j)

				if !field.Exported() {
					continue
				}

				v := sliceutil.GetOrAdd(&result, &Type{
					Pkg:      pkg.path,
					Name:     structName,
					Comments: taggedStruct.Comments,
				})

				v.Fields().Add(FieldData{
					Name:     field.Name(),
					Num:      taggedStruct.Fields[field.Name()],
					TypeData: field,
				})
			}
		}
	}

	return result, nil
}

// ExternalType is a struct which contains external type pkg and name.
type ExternalType struct {
	Pkg  string
	Name string
}

// String returns a string representation of the ExternalType.
func (e ExternalType) String() string {
	return fmt.Sprintf("%s.%s", e.Pkg, e.Name)
}

func externalTypesCmp(left, right ExternalType) int {
	pkgCmp := strings.Compare(left.Pkg, right.Pkg)
	if pkgCmp != 0 {
		return pkgCmp
	}

	return strings.Compare(left.Name, right.Name)
}

// FindExternalTypes finds all external types in the given list of types.
func FindExternalTypes(pkgsTypes slices.Sorted[*Type], taggedStructs ast.TaggedStructs) slices.Sorted[ExternalType] {
	result := slices.NewSortedCompare([]ExternalType{}, externalTypesCmp)

	for i := 0; i < pkgsTypes.Len(); i++ {
		typ := pkgsTypes.Get(i)

		for j := 0; j < typ.fields.Len(); j++ {
			field := typ.fields.Get(j)

			if !field.TypeData.Exported() {
				continue
			}

			typeData := TypeInfo(field.TypeData.Type())

			if typePkg := typeData.typePkg(); typePkg != "" {
				typeName := typeData.typeName()

				if _, ok := taggedStructs.Get(typePkg, typeName); !ok {
					sliceutil.AddIfNotFound(&result, ExternalType{
						Pkg:  typePkg,
						Name: typeName,
					})
				}
			}

			if typ, ok := MatchTypeData[Basic](typeData); ok && typ.Pkg != "" {
				sliceutil.AddIfNotFound(&result, ExternalType{
					Pkg:  typ.Pkg,
					Name: typ.Name,
				})
			}

			if typ, ok := MatchTypeData[Map](typeData); ok && typ.ElemTypePkg != "" {
				sliceutil.AddIfNotFound(&result, ExternalType{
					Pkg:  typ.ElemTypePkg,
					Name: typ.ElemTypeName,
				})
			}
		}
	}

	return result
}

// TypeInfo extracts all type data from the given type.
//
//nolint:gocyclo
func TypeInfo(t types.Type) TypeInfoData {
	switch t := t.(type) {
	case *types.Slice:
		return makeSliceType(TypeInfo(t.Elem()))
	case *types.Map:
		return makeMapType(TypeInfo(t.Key()), TypeInfo(t.Elem()))
	case *types.Basic:
		return makeType[Basic]("", t.Name())
	case *types.Pointer:
		return TypeInfo(t.Elem())
	case *types.Alias:
		return TypeInfo(types.Unalias(t))
	case *types.Named:
		if _, ok := t.Underlying().(*types.Basic); ok {
			return makeType[Basic](t.Obj().Pkg().Path(), t.Obj().Name())
		}

		if underlying, ok := t.Underlying().(*types.Slice); ok {
			return makeSliceType(TypeInfo(underlying.Elem()))
		}

		return makeType[Complex](t.Obj().Pkg().Path(), t.Obj().Name())
	case *types.Interface:
		return makeType[Complex]("", "interface{}")
	case *types.Struct:
		panic(fmt.Errorf("unsupported unnamed struct: %T", t))
	case *types.Chan:
		panic(fmt.Errorf("unsupported type: %T", t))
	default:
		panic(fmt.Errorf("unknown type: %T", t))
	}
}

func makeSliceType(td TypeInfoData) TypeInfoData {
	isSliceInSlice := false

	if td, ok := MatchTypeData[Slice](td); ok {
		if td.Pkg != "" || td.Name != "byte" {
			panic(fmt.Errorf("unsupported slice in slice: %s.%s", td.Pkg, td.Name))
		}

		isSliceInSlice = true
	}

	return MakeTypeData(Slice{
		Pkg:       td.typePkg(),
		Name:      td.typeName(),
		Is2DSlice: isSliceInSlice,
	})
}

func makeType[T Basic | Complex](pkg string, name string) TypeInfoData {
	return MakeTypeData(T{Pkg: pkg, Name: name})
}

func makeMapType(key, elem TypeInfoData) TypeInfoData {
	return MakeTypeData(Map{
		KeyTypePkg:   key.typePkg(),
		KeyTypeName:  key.typeName(),
		ElemTypePkg:  elem.typePkg(),
		ElemTypeName: elem.typeName(),
	})
}

// MakeTypeData creates a new TypeInfoData from the given type.
func MakeTypeData[T Basic | Complex | Map | Slice](v T) TypeInfoData {
	return TypeInfoData{v: v}
}

// MatchTypeData matches the given TypeInfoData to the given type.
func MatchTypeData[T Basic | Complex | Map | Slice](d TypeInfoData) (T, bool) {
	v, ok := d.v.(T)

	return v, ok
}

// TypeInfoData is a struct which contains type data.
type TypeInfoData struct{ v any }

func (td TypeInfoData) typeName() string {
	switch v := td.v.(type) {
	case Basic:
		return v.Name
	case Complex:
		return v.Name
	case Map:
		return v.KeyTypeName
	case Slice:
		return v.Name
	default:
		panic(fmt.Errorf("unknown type: %T", v))
	}
}

func (td TypeInfoData) typePkg() string {
	switch v := td.v.(type) {
	case Basic:
		return v.Pkg
	case Complex:
		return v.Pkg
	case Map:
		return v.KeyTypePkg
	case Slice:
		return v.Pkg
	default:
		panic(fmt.Errorf("unknown type: %T", v))
	}
}

// Basic is a basic type.
type Basic struct {
	Name string
	Pkg  string
}

// Complex is a complex type.
type Complex struct {
	Name string
	Pkg  string
}

// Slice is a slice type.
type Slice struct {
	Name      string
	Pkg       string
	Is2DSlice bool
}

// Map is a map type.
type Map struct {
	KeyTypeName string
	KeyTypePkg  string

	ElemTypeName string
	ElemTypePkg  string
}
