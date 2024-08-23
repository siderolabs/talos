// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package proto contains the protobuf generation logic.
package proto

//nolint:gci
import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/typ.v4/slices"

	"github.com/siderolabs/structprotogen/consts"
	"github.com/siderolabs/structprotogen/sliceutil"
	"github.com/siderolabs/structprotogen/types"
)

// Pkg represents a protobuf package.
type Pkg struct {
	Name  string
	GoPkg string

	isInit    bool
	protoDefs slices.Sorted[*protoDef]
	imports   slices.Sorted[string]
}

func protoPkgsCmp(left, right *Pkg) int {
	return strings.Compare(left.Name, right.Name)
}

func (p *Pkg) init() {
	if !p.isInit {
		p.protoDefs = slices.NewSortedCompare([]*protoDef{}, protoDefCmp)
		p.imports = slices.NewSortedCompare([]string{}, strings.Compare)
		p.isInit = true
	}
}

// Defs returns the list of definitions.
func (p *Pkg) Defs() *slices.Sorted[*protoDef] {
	p.init()

	return &p.protoDefs
}

// Imports returns the list of imports.
func (p *Pkg) Imports() *slices.Sorted[string] {
	p.init()

	return &p.imports
}

// WriteDebug is like Format, but writes additional debug info.
func (p *Pkg) WriteDebug(w io.Writer) {
	pkgName := p.Name

	fmt.Fprint(w, "syntax = \"proto3\";\n\n")
	fmt.Fprintf(w, "package talos.resource.definitions.%s; // %s\n\n", p.Name, p.GoPkg)
	fmt.Fprintf(w, "option go_package = \"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/%s\";\n", pkgName) // TODO: insert proper path
	fmt.Fprintf(w, "option java_package = \"dev.talos.api.resource.definitions.%s\";\n\n", pkgName)

	if p.imports.Len() > 0 {
		for i := 0; i < p.imports.Len(); i++ {
			importPath := p.imports.Get(i)
			if !strings.ContainsRune(importPath, '.') {
				importPath = "talos.resource.definitions." + importPath
			}

			fmt.Fprintf(w, "import \"%s\";\n", importPath)
		}

		fmt.Fprintln(w, ``)
	}

	for i := 0; i < p.protoDefs.Len(); i++ {
		p.protoDefs.Get(i).WriteDebug(w)
		fmt.Fprintln(w)
	}
}

// Format formats the protobuf data.
func (p *Pkg) Format(w io.Writer) {
	pkgName := p.Name

	fmt.Fprint(w, "syntax = \"proto3\";\n\n")
	fmt.Fprintf(w, "package talos.resource.definitions.%s;\n\n", p.Name)
	fmt.Fprintf(w, "option go_package = \"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/%s\";\n", pkgName) // TODO: insert proper path
	fmt.Fprintf(w, "option java_package = \"dev.talos.api.resource.definitions.%s\";\n\n", pkgName)

	if p.imports.Len() > 0 {
		for i := 0; i < p.imports.Len(); i++ {
			importPath := p.imports.Get(i)
			if !strings.ContainsRune(importPath, '.') {
				importPath = "talos.resource.definitions." + importPath
			}

			fmt.Fprintf(w, "import \"%s\";\n", importPath)
		}

		fmt.Fprintln(w, ``)
	}

	for i := 0; i < p.protoDefs.Len(); i++ {
		p.protoDefs.Get(i).Format(w)
		fmt.Fprintln(w)
	}
}

type protoDef struct {
	name string

	goPkg    string
	comments []string

	isInit bool
	fields slices.Sorted[protoField]
}

func protoDefCmp(left, right *protoDef) int {
	return strings.Compare(left.name, right.name)
}

func (p *protoDef) init() {
	if !p.isInit {
		p.fields = slices.NewSortedCompare([]protoField{}, protoFieldCmp)
		p.isInit = true
	}
}

func (p *protoDef) Fields() *slices.Sorted[protoField] {
	p.init()

	return &p.fields
}

func (p *protoDef) WriteDebug(w io.Writer) {
	for _, comment := range p.comments {
		fmt.Fprintf(w, "%s\n", comment)
	}

	fmt.Fprintf(w, "message %s { //%s.%s\n", p.name, p.goPkg, p.name)

	for i := 0; i < p.fields.Len(); i++ {
		fmt.Fprintf(w, "  ")
		p.fields.Get(i).WriteDebug(w)
	}

	fmt.Fprintln(w, "}")
}

func (p *protoDef) Format(w io.Writer) {
	for _, comment := range p.comments {
		fmt.Fprintf(w, "%s\n", comment)
	}

	fmt.Fprintf(w, "message %s {\n", p.name)

	for i := 0; i < p.fields.Len(); i++ {
		fmt.Fprintf(w, "  ")
		p.fields.Get(i).Format(w)
	}

	fmt.Fprintln(w, "}")
}

type protoField struct {
	name string
	typ  string
	num  int

	goType string
}

func protoFieldCmp(left, right protoField) int {
	if left.num == 0 {
		panic(fmt.Errorf("left field '%s' has no number", left.name))
	}

	if right.num == 0 {
		panic(fmt.Errorf("right field '%s' has no number", right.name))
	}

	switch {
	case left.num < right.num:
		return -1
	case left.num > right.num:
		return 1
	default:
		return 0
	}
}

func (pf protoField) WriteDebug(w io.Writer) {
	fmt.Fprintf(w, "%s %s = %d; // %s \n", pf.typ, ToSnakeCase(pf.name), pf.num, pf.goType)
}

func (pf protoField) Format(w io.Writer) {
	fmt.Fprintf(w, "%s %s = %d;\n", pf.typ, ToSnakeCase(pf.name), pf.num)
}

// PrepareProtoData prepares the data for the protobuf generation.
//
//nolint:gocyclo,cyclop
func PrepareProtoData(pkgsTypes slices.Sorted[*types.Type], constants consts.ConstBlocks) slices.Sorted[*Pkg] {
	result := slices.NewSortedCompare([]*Pkg{}, protoPkgsCmp)

	for i := 0; i < pkgsTypes.Len(); i++ {
		pkgType := pkgsTypes.Get(i)

		protoPkg := sliceutil.GetOrAdd(&result, &Pkg{
			Name:  pkgType.PkgName(),
			GoPkg: pkgType.Pkg,
		})

		def := sliceutil.GetOrAdd(protoPkg.Defs(), &protoDef{
			name:     pkgType.Name,
			goPkg:    pkgType.Pkg,
			comments: pkgType.Comments,
		})

		for j := 0; j < pkgType.Fields().Len(); j++ {
			field := pkgType.Fields().Get(j)

			fieldTypeData := types.TypeInfo(field.TypeData.Type())

			if fieldTyp, ok := types.MatchTypeData[types.Complex](fieldTypeData); ok {
				importName, typeName := mustFormatTypeName(fieldTyp.Pkg, fieldTyp.Name, pkgType.Pkg)

				if importName != "" {
					sliceutil.AddIfNotFound(protoPkg.Imports(), importName)
				}

				sliceutil.AddIfNotFound(def.Fields(), protoField{
					name:   field.Name,
					typ:    typeName,
					num:    field.Num,
					goType: field.TypeData.Type().String(),
				})

				continue
			}

			if fieldTyp, ok := types.MatchTypeData[types.Basic](fieldTypeData); ok {
				var importName, typeName string

				if block, ok := constants.Get(fieldTyp.Pkg, fieldTyp.Name); ok {
					importName = "resource/definitions/enums/enums.proto"
					typeName = "talos.resource.definitions.enums." + block.ProtoMessageName()
				} else {
					importName, typeName = mustFormatBasicTypeName(fieldTyp.Pkg, fieldTyp.Name)
				}

				if importName != "" {
					sliceutil.AddIfNotFound(protoPkg.Imports(), importName)
				}

				sliceutil.AddIfNotFound(def.Fields(), protoField{
					name:   field.Name,
					typ:    typeName,
					num:    field.Num,
					goType: field.TypeData.Type().String(),
				})

				continue
			}

			if fieldTyp, ok := types.MatchTypeData[types.Slice](fieldTypeData); ok {
				var importName, typeName string

				block, isEnum := constants.Get(fieldTyp.Pkg, fieldTyp.Name)

				switch {
				case isEnum:
					importName = "resource/definitions/enums/enums.proto"
					typeName = "repeated talos.resource.definitions.enums." + block.ProtoMessageName()
				case fieldTyp.Pkg == "" && fieldTyp.Name == "byte" && fieldTyp.Is2DSlice: //nolint:goconst
					typeName = "repeated bytes"
				case fieldTyp.Pkg == "" && fieldTyp.Name == "byte":
					typeName = "bytes"
				case fieldTyp.Pkg == "":
					typeName = fmt.Sprintf("repeated %s", getProtoBasicName(fieldTyp.Name))
				default:
					importName, typeName = mustFormatTypeName(fieldTyp.Pkg, fieldTyp.Name, pkgType.Pkg)
					typeName = fmt.Sprintf("repeated %s", typeName)
				}

				if importName != "" {
					sliceutil.AddIfNotFound(protoPkg.Imports(), importName)
				}

				sliceutil.AddIfNotFound(def.Fields(), protoField{
					name:   field.Name,
					typ:    typeName,
					num:    field.Num,
					goType: field.TypeData.Type().String(),
				})

				continue
			}

			if fieldTyp, ok := types.MatchTypeData[types.Map](fieldTypeData); ok {
				// key cannot be anything but a basic type
				importKeyName, keyTypeName := mustFormatBasicTypeName(fieldTyp.KeyTypePkg, fieldTyp.KeyTypeName)
				if importKeyName != "" {
					panic(fmt.Errorf("map key type '%s.%s' is not basic type", fieldTyp.KeyTypePkg, fieldTyp.KeyTypeName))
				}

				var (
					typText    string
					importElem string
				)

				switch {
				case fieldTyp.ElemTypeName == "interface{}": // handle map[key]interface{}
					importElem = "google/protobuf/struct.proto"
					typText = "google.protobuf.Struct"
				case fieldTyp.ElemTypePkg == "":
					elemTypeName := getProtoBasicName(fieldTyp.ElemTypeName)
					typText = fmt.Sprintf("map<%s, %s>", keyTypeName, elemTypeName)
				case fieldTyp.ElemTypePkg == pkgType.Pkg:
					var elemTypeName string
					importElem, elemTypeName = mustFormatTypeName(fieldTyp.ElemTypePkg, fieldTyp.ElemTypeName, pkgType.Pkg)
					typText = fmt.Sprintf("map<%s, %s>", keyTypeName, elemTypeName)
				default:
					panic(fmt.Errorf("map value type '%s.%s' is not known type", fieldTyp.ElemTypePkg, fieldTyp.ElemTypeName))
				}

				if importElem != "" {
					sliceutil.AddIfNotFound(protoPkg.Imports(), importElem)
				}

				sliceutil.AddIfNotFound(def.Fields(), protoField{
					name:   field.Name,
					typ:    typText,
					num:    field.Num,
					goType: field.TypeData.Type().String(),
				})

				continue
			}
		}
	}

	return result
}

func mustFormatTypeName(fieldTypePkg string, fieldType string, declPkg string) (string, string) {
	importPath, name := formatTypeName(fieldTypePkg, fieldType, declPkg)
	if name == "" {
		panic(fmt.Errorf("unknown type %s.%s", fieldTypePkg, fieldType))
	}

	return importPath, name
}

func formatTypeName(fieldTypePkg string, fieldType string, declPkg string) (string, string) {
	if fieldTypePkg == declPkg {
		return "", fieldType
	}

	type typeData struct {
		pkg  string
		name string
	}

	td := typeData{
		name: fieldType,
		pkg:  fieldTypePkg,
	}

	const commoProto = "common/common.proto"

	switch td {
	case typeData{"time", "Time"}:
		return "google/protobuf/timestamp.proto", "google.protobuf.Timestamp"
	case typeData{"net/url", "URL"}:
		return commoProto, "common.URL"
	case typeData{"net/netip", "Prefix"}:
		return commoProto, "common.NetIPPrefix"
	case typeData{"net/netip", "AddrPort"}:
		return commoProto, "common.NetIPPort"
	case typeData{"net/netip", "Addr"}:
		return commoProto, "common.NetIP"
	case typeData{"github.com/opencontainers/runtime-spec/specs-go", "Mount"}:
		return "resource/definitions/proto/proto.proto", "talos.resource.definitions.proto.Mount"
	case typeData{"github.com/siderolabs/crypto/x509", "PEMEncodedCertificateAndKey"}:
		return commoProto, "common.PEMEncodedCertificateAndKey"
	case typeData{"github.com/siderolabs/crypto/x509", "PEMEncodedKey"}:
		return commoProto, "common.PEMEncodedKey"
	case typeData{"github.com/siderolabs/crypto/x509", "PEMEncodedCertificate"}:
		return commoProto, "common.PEMEncodedCertificate"
	default:
		return "", ""
	}
}

func mustFormatBasicTypeName(fieldTypePkg string, fieldType string) (string, string) {
	if fieldTypePkg == "" {
		return "", getProtoBasicName(fieldType)
	}

	importPath, fullName := formatBasicTypeName(fieldTypePkg, fieldType)
	if fullName == "" {
		panic(fmt.Errorf("unknown type %s.%s", fieldTypePkg, fieldType))
	}

	return importPath, fullName
}

// IsSupportedExternalType checks if external type is supported.
func IsSupportedExternalType(typ types.ExternalType) bool {
	if _, name := formatBasicTypeName(typ.Pkg, typ.Name); name != "" {
		return true
	}

	if _, name := formatTypeName(typ.Pkg, typ.Name, ""); name != "" {
		return true
	}

	return false
}

//nolint:gocyclo,cyclop
func formatBasicTypeName(typPkg string, typ string) (importPath, fullName string) {
	type typeData struct {
		pkg  string
		name string
	}

	td := typeData{
		name: typ,
		pkg:  typPkg,
	}

	switch td {
	case typeData{"time", "Duration"}:
		return "google/protobuf/duration.proto", "google.protobuf.Duration"
	case typeData{"io/fs", "FileMode"}:
		return "", "uint32" //nolint:goconst
	case typeData{"github.com/siderolabs/talos/pkg/machinery/nethelpers", "AddressFlags"}:
		return "", "uint32"
	case typeData{"github.com/siderolabs/talos/pkg/machinery/nethelpers", "LinkFlags"}:
		return "", "uint32"
	case typeData{"github.com/siderolabs/talos/pkg/machinery/nethelpers", "RouteFlags"}:
		return "", "uint32"
	default:
		return "", ""
	}
}

//nolint:gocyclo
func getProtoBasicName(typ string) string {
	switch typ {
	case "bool":
		return "bool"
	case "int8", "int16":
		return "fixed32"
	case "int32":
		return "int32"
	case "int64", "int":
		return "int64"
	case "byte", "uint8", "uint16":
		return "fixed32"
	case "uint32":
		return "uint32"
	case "uint64", "uint":
		return "uint64"
	case "float32":
		return "float"
	case "float64":
		return "double"
	case "string":
		return "string"
	default:
		panic(fmt.Sprintf("unknown type %s", typ))
	}
}

// ToSnakeCase converts a string to snake case.
func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	snake = strings.ToLower(snake)

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
