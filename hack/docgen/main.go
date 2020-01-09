// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

var tpl = `
{{ $tick := "` + "`" + `" -}}
### {{ .Name }}

{{ range $entry := .Entries -}}
#### {{ $entry.Name }}

{{ $entry.Text.Description }}
Type: {{ $tick }}{{ $entry.Type }}{{ $tick }}

{{ if $entry.Text.Values -}}
Valid Values:

{{ range $value := $entry.Text.Values -}}
- {{ $tick }}{{ $value }}{{ $tick }}
{{ end }}
{{ end -}}
{{ if $entry.Text.Examples -}}
Examples:
{{ range $example := $entry.Text.Examples }}
{{ $tick }}{{ $tick }}{{ $tick }}yaml
{{ $example }}
{{ $tick }}{{ $tick }}{{ $tick }}
{{ end }}
{{ end }}
{{- if $entry.Note -}}
> {{ $entry.Note }}
{{ end }}
{{- end -}}
---
`

type Doc struct {
	Title    string
	Sections []*Section
}

type Section struct {
	Name    string
	Entries []*Entry
}

type Entry struct {
	Name string
	Type string
	Text *Text
	Note string
}

type Text struct {
	Description string   `json:"description"`
	Values      []string `json:"values"`
	Examples    []string `json:"examples"`
}

func in(p string) (string, error) {
	return filepath.Abs(p)
}

func out(p string) (*os.File, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	return os.Create(abs)
}

type structType struct {
	name string
	pos  token.Pos
	node *ast.StructType
}

func collectStructs(node ast.Node) []*structType {
	structs := []*structType{}

	collectStructs := func(n ast.Node) bool {
		g, ok := n.(*ast.GenDecl)
		if !ok {
			return true
		}

		if g.Doc != nil {
			for _, comment := range g.Doc.List {
				if strings.Contains(comment.Text, "docgen: nodoc") {
					return true
				}
			}
		}

		for _, spec := range g.Specs {
			t, ok := spec.(*ast.TypeSpec)
			if !ok {
				return true
			}

			if t.Type == nil {
				return true
			}

			x, ok := t.Type.(*ast.StructType)
			if !ok {
				return true
			}

			structName := t.Name.Name

			s := &structType{
				name: structName,
				node: x,
				pos:  x.Pos(),
			}

			structs = append(structs, s)
		}

		return true
	}

	ast.Inspect(node, collectStructs)

	return structs
}

type field struct {
}

func parseFieldType(p interface{}) string {
	switch t := p.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		return "array"
	case *ast.MapType:
		return "map"
	case *ast.StructType:
		return "struct"
	case *ast.StarExpr:
		return parseFieldType(t.X)
	case *ast.SelectorExpr:
		return parseFieldType(t.Sel)
	default:
		log.Printf("unknown: %#v", t)
		return ""
	}
}

func collectFields(s *structType) (entries []*Entry) {
	entries = []*Entry{}

	for _, field := range s.node.Fields.List {
		if field.Tag == nil {
			if field.Names == nil {
				// This is an embedded struct.
				continue
			}
			log.Fatalf("field %q is missing a yaml tag", field.Names[0].Name)
		}

		if field.Doc == nil {
			log.Fatalf("field %q is missing a documentation", field.Names[0].Name)
		}

		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		name := tag.Get("yaml")
		name = strings.Split(name, ",")[0]

		fieldType := parseFieldType(field.Type)

		text := &Text{}
		if err := yaml.Unmarshal([]byte(field.Doc.Text()), text); err != nil {
			log.Fatal(err)
		}

		entry := &Entry{
			Name: name,
			Type: fieldType,
			Text: text,
		}

		if field.Comment != nil {
			entry.Note = field.Comment.Text()
		}

		entries = append(entries, entry)
	}

	return entries
}

func render(section *Section, f *os.File) {
	t := template.Must(template.New("section.tpl").Parse(tpl))
	err := t.Execute(f, section)
	if err != nil {
		panic(err)
	}
}

func main() {
	abs, err := in(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("creating package file set: %q\n", abs)

	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, abs, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var structs []*structType

	for _, pkg := range pkgs {
		for _, astFile := range pkg.Files {
			tokenFile := fset.File(astFile.Pos())
			if tokenFile == nil {
				continue
			}

			fmt.Printf("parsing file in package %q: %s\n", pkg.Name, tokenFile.Name())
			structs = append(structs, collectStructs(astFile)...)
		}
	}

	if len(structs) == 0 {
		log.Fatalf("failed to find types that could be documented in %s", abs)
	}

	doc := &Doc{
		Sections: []*Section{},
	}

	for _, s := range structs {
		fmt.Printf("generating docs for type: %q\n", s.name)

		entries := collectFields(s)

		section := &Section{
			Name:    s.name,
			Entries: entries,
		}

		doc.Sections = append(doc.Sections, section)
	}

	node, err := parser.ParseFile(fset, filepath.Join(abs, "doc.go"), nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, err := out(os.Args[2])
	defer out.Close()

	out.WriteString("---\n")
	out.WriteString("title: " + node.Name.Name + "\n")
	out.WriteString("---\n")
	out.WriteString("\n")
	out.WriteString("<!-- markdownlint-disable MD024 -->")
	out.WriteString("\n")
	out.WriteString("\n")
	out.WriteString(node.Doc.Text())

	for _, section := range doc.Sections {
		render(section, out)
	}
}
