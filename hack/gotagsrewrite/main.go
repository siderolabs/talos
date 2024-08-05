// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/fatih/structtag"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "gotagsrewrite path",
	Short:   "This CLI is used to add `protobuf:<n>` tags to structs with //gotagsrewrite:gen comment",
	Example: "gotagsrewrite .",
	Args:    cobra.ExactArgs(1),
	Version: "v1.0.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run(args[0])
	},
	SilenceUsage:      true,
	DisableAutoGenTag: true,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Run runs the main logic of the program.
func Run(path string) error {
	paths, err := findGoFiles(path)
	if err != nil {
		return fmt.Errorf("failed to find Go files: %w", err)
	}

	filesAndStructs := map[string]fileStruct{}
	tokenSet := token.NewFileSet()

	for _, path := range paths {
		parsedAST, err := parseAst(tokenSet, path) //nolint:govet
		if err != nil {
			return fmt.Errorf("failed to parse AST for %s: %w", path, err)
		}

		structs := findAllStructs(parsedAST)
		if len(structs) == 0 {
			continue
		}

		filesAndStructs[path] = fileStruct{
			path:    path,
			fullAST: parsedAST,
			structs: append(filesAndStructs[path].structs, structs...),
		}
	}

	err = updateStructs(filesAndStructs, tokenSet)
	if err != nil {
		return err
	}

	return nil
}

func updateStructs(filesAndStructs map[string]fileStruct, tokenSet *token.FileSet) error {
	for path, val := range filesAndStructs {
		for _, t := range val.structs {
			err := updateProtoTags(t)
			if err != nil {
				return err
			}

			goFile, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", path, err)
			}

			err = format.Node(goFile, tokenSet, val.fullAST)
			if err != nil {
				return err
			}
		}

		fmt.Printf("updated %s\n", path)
	}

	return nil
}

// findGoFiles recursevly finds all Go files in the given directory.
func findGoFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if info.Name() == "testdata" || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

type fileStruct struct {
	path    string
	fullAST *ast.File
	structs []*ast.TypeSpec
}

func updateProtoTags(t *ast.TypeSpec) error {
	structNode := t.Type.(*ast.StructType) //nolint:errcheck

	num, err := findHighestProtoNum(structNode)
	if err != nil {
		return fmt.Errorf("failed to update proto tags for %s: %w", t.Name.Name, err)
	}

	if num == -1 {
		return nil
	}

	err = forEachFieldTag(structNode, func(tags *structtag.Tags) (*structtag.Tags, error) {
		_, err := tags.Get("protobuf") //nolint:govet
		if err == nil {
			return nil, nil
		}

		num++
		newTag := &structtag.Tag{
			Key:     "protobuf",
			Name:    strconv.Itoa(num),
			Options: nil,
		}

		tags.Set(newTag) //nolint:errcheck

		return tags, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// findHighestProtoNum returns the highest proto num in the given struct.
// It returns -1 if struct has no exported fields. It returns 0 if there is no fields with "protobuf" tag.
// Otherwise, it returns the highest proto num extracted from the fields tags.
func findHighestProtoNum(structNode *ast.StructType) (int, error) {
	highestNum := -1

	err := forEachFieldTag(structNode, func(tags *structtag.Tags) (*structtag.Tags, error) {
		if highestNum == -1 {
			highestNum = 0
		}

		tag, err := tags.Get("protobuf")
		if err != nil {
			return nil, nil
		}

		num, err := strconv.Atoi(tag.Name)
		if err != nil {
			return nil, err
		}

		highestNum = max(highestNum, num)

		return nil, nil
	})

	return highestNum, err
}

func forEachFieldTag(structNode *ast.StructType, fn func(tags *structtag.Tags) (*structtag.Tags, error)) error {
	for _, field := range structNode.Fields.List {
		if len(field.Names) < 1 {
			continue
		}

		fieldName := field.Names[0]
		if fieldName == nil || !isCapitalCase(fieldName.Name) {
			continue
		}

		tags := &structtag.Tags{}
		tagValue := ""

		if field.Tag != nil {
			tagValue = strings.Trim(field.Tag.Value, "`")

			var err error

			tags, err = structtag.Parse(tagValue)
			if err != nil {
				return fmt.Errorf("invalid tag: field '%s', tag '%s': %w", fieldName.Name, tagValue, err)
			}
		}

		newTags, err := fn(tags)

		switch {
		case err != nil:
			return fmt.Errorf("tag failure: field '%s', tag '%s': %w", fieldName.Name, tagValue, err)
		case newTags == nil:
			continue
		}

		if field.Tag == nil {
			field.Tag = &ast.BasicLit{
				Kind: token.STRING,
			}
		}

		field.Tag.Value = "`" + newTags.String() + "`"
	}

	return nil
}

// isCapitalCase returns true if the given string is in capital case.
func isCapitalCase(s string) bool {
	return len(s) > 0 && unicode.IsUpper(rune(s[0]))
}

const fileMode = 0o644

// CopyFile copies the contents of src to dst atomically.
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer wrapErr(&err, in.Close)

	tmp, err := ioutil.TempFile(filepath.Dir(dst), "copyfile")
	if err != nil {
		return err
	}

	_, err = io.Copy(tmp, in)
	if err != nil {
		panicOnErr(tmp.Close())
		panicOnErr(os.Remove(tmp.Name()))

		return err
	}

	if err := tmp.Close(); err != nil {
		panicOnErr(os.Remove(tmp.Name()))

		return err
	}

	if err := os.Chmod(tmp.Name(), fileMode); err != nil {
		panicOnErr(os.Remove(tmp.Name()))

		return err
	}

	if err := os.Rename(tmp.Name(), dst); err != nil {
		panicOnErr(os.Remove(tmp.Name()))

		return err
	}

	return nil
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func wrapErr(e *error, c func() error) {
	err := c()
	if err != nil && *e == nil {
		*e = err
	}
}
