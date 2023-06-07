// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	yaml "gopkg.in/yaml.v3"
)

var markdownTemplate = `
{{ .Description }}
{{- $anchors := .Anchors -}}
{{- $tick := "` + "`" + `" -}}
{{ range $struct := .Structs }}
---
## {{ $struct.Type }}
{{ if $struct.Description -}}
{{ $struct.Description }}
{{ end }}
{{ if $struct.AppearsIn -}}
Appears in:

{{ range $appearance := $struct.AppearsIn -}}
- <code>{{ encodeType $appearance.TypeName }}.{{ $appearance.FieldName }}</code>
{{ end -}}
{{ end }}

{{ range $example := $struct.Examples }}
{{ yaml $example.GetValue "" }}
{{ end }}

{{ if $struct.Fields -}}
| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
{{ range $field := $struct.Fields -}}
{{ if $field.Name -}}
| {{- $tick }}{{ $field.Name }}{{ $tick }} |
{{- encodeType $field.Type }} |
{{- fmtDesc $field.Description }} {{ with $field.Examples }}<details><summary>Show example(s)</summary>{{ range . }}{{ yaml .GetValue $field.Name }}{{ end }}</details>{{ end }} |
{{- range $value := $field.Values }}{{ $tick }}{{ $value }}{{ $tick }}<br />{{ end }} |
{{ end -}}
{{ end }}
{{ end }}
{{ end }}`

// FileDoc represents a single go file documentation.
type FileDoc struct {
	// Name will be used in md file name pattern.
	Name string
	// Description file description, supports markdown.
	Description string
	// Structs structs defined in the file.
	Structs []*Doc
	Anchors map[string]string

	t *template.Template
}

// Encode encodes file documentation as MD file.
func (fd *FileDoc) Encode() ([]byte, error) {
	anchors := map[string]string{}
	for _, t := range fd.Structs {
		anchors[t.Type] = strings.ToLower(t.Type)
	}

	fd.Anchors = anchors

	fd.t = template.Must(template.New("file_markdown.tpl").
		Funcs(template.FuncMap{
			"yaml":       encodeYaml,
			"fmtDesc":    formatDescription,
			"encodeType": fd.encodeType,
		}).
		Parse(markdownTemplate))

	buf := bytes.Buffer{}

	if err := fd.t.Execute(&buf, fd); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Write dumps documentation string to folder.
func (fd *FileDoc) Write(path, frontmatter string) error {
	data, err := fd.Encode()
	if err != nil {
		return err
	}

	if stat, e := os.Stat(path); !os.IsNotExist(e) {
		if !stat.IsDir() {
			return fmt.Errorf("destination path should be a directory")
		}
	} else {
		if e := os.MkdirAll(path, 0o777); e != nil {
			return e
		}
	}

	f, err := os.Create(filepath.Join(path, fmt.Sprintf("%s.%s", strings.ToLower(fd.Name), "md")))
	if err != nil {
		return err
	}

	if _, err := f.WriteString(frontmatter); err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}

func (fd *FileDoc) encodeType(t string) string {
	re := regexp.MustCompile(`\w+`)

	for _, s := range re.FindAllString(t, -1) {
		if anchor, ok := fd.Anchors[s]; ok {
			t = strings.ReplaceAll(t, s, formatLink(s, "#"+anchor))
		}
	}

	return t
}

func encodeYaml(in interface{}, name string) string {
	if name != "" {
		in = map[string]interface{}{
			name: in,
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

func formatLink(text, link string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, link, text)
}

func formatDescription(description string) string {
	lines := strings.Split(description, "\n")
	if len(lines) <= 1 {
		return strings.Join(lines, "<br />")
	}

	return fmt.Sprintf("<details><summary>%s</summary>%s</details>", lines[0], strings.Join(lines[1:], "<br />"))
}
