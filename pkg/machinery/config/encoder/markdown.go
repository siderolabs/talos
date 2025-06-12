// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	yaml "gopkg.in/yaml.v3"
)

//go:embed "markdown.tmpl"
var markdownTemplate string

// FileDoc represents a single go file documentation.
type FileDoc struct {
	// Name will be used in md file name pattern.
	Name string
	// Description file description, supports markdown.
	Description string
	// Structs structs defined in the file.
	Structs []*Doc
	// Types is map of all non-trivial types defined in the file.
	Types map[string]*Doc
}

// Encode encodes file documentation as MD file.
func (fd *FileDoc) Encode(root *Doc, frontmatter func(title, description string) string) ([]byte, error) {
	t := template.Must(template.New("markdown.tmpl").
		Funcs(template.FuncMap{
			"yaml":        encodeYaml,
			"fmtDesc":     formatDescription,
			"dict":        tmplDict,
			"repeat":      strings.Repeat,
			"trimPrefix":  strings.TrimPrefix,
			"add":         func(a, b int) int { return a + b },
			"frontmatter": frontmatter,
			"min":         minInt,
		}).
		Parse(markdownTemplate))

	var buf bytes.Buffer

	if err := t.Execute(&buf, struct {
		Root  *Doc
		Types map[string]*Doc
	}{
		Root:  root,
		Types: fd.Types,
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Write dumps documentation string to folder.
//
//nolint:gocyclo
func (fd *FileDoc) Write(path string, frontmatter func(title, description string) string) error {
	if stat, err := os.Stat(path); !os.IsNotExist(err) {
		if !stat.IsDir() {
			return errors.New("destination path should be a directory")
		}
	} else {
		if err := os.MkdirAll(path, 0o777); err != nil {
			return err
		}
	}

	// generate _index.md
	if err := os.WriteFile(filepath.Join(path, "_index.md"), []byte(frontmatter(fd.Name, fd.Description)), 0o666); err != nil {
		return err
	}

	// find map of all types
	fd.Types = map[string]*Doc{}

	for _, t := range fd.Structs {
		if t.Type == "" || strings.ToLower(t.Type) == t.Type {
			continue
		}

		fd.Types[t.Type] = t
	}

	// find root nodes
	var roots []*Doc

	for _, t := range fd.Structs {
		if len(t.AppearsIn) == 0 {
			roots = append(roots, t)
		}
	}

	for _, root := range roots {
		contents, err := fd.Encode(root, frontmatter)
		if err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("%s.%s", strings.ToLower(root.Type), "md")), contents, 0o666); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo
func encodeYaml(in any, path string) string {
	if path != "" {
		parts := strings.Split(path, ".")

		parts = parts[1:] // strip first segment, it's root element

		// if the last element is ""/"-", it means we're at the root of the slice, so we don't need to wrap it once again
		if len(parts) > 0 && (parts[len(parts)-1] == "" || parts[len(parts)-1] == "-") {
			parts = parts[:len(parts)-1]
		}

		slices.Reverse(parts)

		for _, part := range parts {
			switch part {
			case "":
				in = []any{in}
			case "-":
				in = map[string]any{
					"example.com": in,
				}
			default:
				in = map[string]any{
					part: in,
				}
			}
		}
	}

	node, err := toYamlNode(in, newOptions(WithComments(CommentsAll)))
	if err != nil {
		return fmt.Sprintf("yaml encoding failed %s", err)
	}

	data, err := yaml.Marshal(node)
	if err != nil {
		return fmt.Sprintf("yaml encoding failed %s", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	return fmt.Sprintf("{{< highlight yaml >}}\n%s{{< /highlight >}}", strings.Join(lines, "\n"))
}

func formatDescription(description string) string {
	return strings.ReplaceAll(description, "\n", "<br>")
}

func tmplDict(vals ...any) (map[string]any, error) {
	if len(vals)%2 != 0 {
		return nil, fmt.Errorf("invalid number of arguments: %d", len(vals))
	}

	res := map[string]any{}

	for i := 0; i < len(vals); i += 2 {
		key, ok := vals[i].(string)
		if !ok {
			return nil, fmt.Errorf("invalid key type: %T", vals[i])
		}

		res[key] = vals[i+1]
	}

	return res, nil
}

func minInt(a, b int) int {
	return min(a, b)
}
