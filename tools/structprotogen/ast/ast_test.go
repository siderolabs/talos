// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ast_test

import (
	goast "go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/siderolabs/talos/tools/structprotogen/ast"
)

func TestFieldComments(t *testing.T) {
	const src = `package p

type Spec struct {
	// MaxRetries is the limit; nil is unlimited.
	//structprotogen:optional
	MaxRetries *uint32 ` + "`protobuf:\"1\"`" + `
	// Plain is a plain field.
	Plain uint32 ` + "`protobuf:\"2\"`" + ` // trailing
}
`

	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	fields := file.Decls[0].(*goast.GenDecl).Specs[0].(*goast.TypeSpec).Type.(*goast.StructType).Fields.List //nolint:forcetypeassert,errcheck

	for _, test := range []struct {
		field    *goast.Field
		comments []string
		optional bool
	}{
		{field: fields[0], comments: []string{"// MaxRetries is the limit; nil is unlimited."}, optional: true},
		{field: fields[1], comments: []string{"// Plain is a plain field.", "// trailing"}},
	} {
		comments, optional := ast.FieldComments(test.field)

		if !slices.Equal(comments, test.comments) || optional != test.optional {
			t.Errorf("%s: got %q, %v; want %q, %v", test.field.Names[0], comments, optional, test.comments, test.optional)
		}
	}
}
