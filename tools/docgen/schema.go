// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/invopop/jsonschema"
	"github.com/microcosm-cc/bluemonday"
	validatejsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const ConfigSchemaURLFormat = "https://talos.dev/%s/schemas/%s"

// SchemaWrapper wraps jsonschema.Schema to provide correct YAML unmarshalling using its internal JSON marshaller.
type SchemaWrapper struct {
	jsonschema.Schema
}

// UnmarshalYAML unmarshals the schema from YAML.
//
// This converts the YAML that was read from the comments to JSON,
// then uses the custom JSON unmarshaler of the wrapped Schema.
func (t *SchemaWrapper) UnmarshalYAML(unmarshal func(any) error) error {
	var data map[string]any

	err := unmarshal(&data)
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return t.UnmarshalJSON(jsonBytes)
}

type SchemaTypeInfo struct {
	typeName string
	ref      string
}

type SchemaDefinitionInfo struct {
	typeInfo           SchemaTypeInfo
	arrayItemsTypeInfo *SchemaTypeInfo
	mapValueTypeInfo   *SchemaTypeInfo
	enumValues         []any
}

func goTypeToTypeInfo(pkg, goType string) *SchemaTypeInfo {
	switch goType {
	case "string":
		return &SchemaTypeInfo{typeName: "string"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return &SchemaTypeInfo{typeName: "integer"}
	case "bool":
		return &SchemaTypeInfo{typeName: "boolean"}
	default:
		return &SchemaTypeInfo{ref: "#/$defs/" + pkg + "." + goType}
	}
}

func fieldToDefinitionInfo(pkg string, field *Field) SchemaDefinitionInfo {
	goType := field.Type

	if field.Text != nil {
		// skip enumifying boolean fields so that we don't pick up the "on", "off", "yes", "no" from the values
		if goType != "bool" && field.Text.Values != nil {
			enumValues := make([]any, 0, len(field.Text.Values))

			for _, val := range field.Text.Values {
				enumValues = append(enumValues, val)
			}

			return SchemaDefinitionInfo{enumValues: enumValues}
		}
	}

	if strings.HasPrefix(goType, "[]") {
		return SchemaDefinitionInfo{
			typeInfo:           SchemaTypeInfo{typeName: "array"},
			arrayItemsTypeInfo: goTypeToTypeInfo(pkg, strings.TrimPrefix(goType, "[]")),
		}
	}

	if strings.HasPrefix(goType, "map[string]") {
		return SchemaDefinitionInfo{
			typeInfo:         SchemaTypeInfo{typeName: "object"},
			mapValueTypeInfo: goTypeToTypeInfo(pkg, strings.TrimPrefix(goType, "map[string]")),
		}
	}

	return SchemaDefinitionInfo{
		typeInfo: *goTypeToTypeInfo(pkg, goType),
	}
}

func typeInfoToSchema(typeInfo *SchemaTypeInfo) *jsonschema.Schema {
	schema := jsonschema.Schema{}

	if typeInfo.typeName != "" {
		schema.Type = typeInfo.typeName
	}

	if typeInfo.ref != "" {
		schema.Ref = typeInfo.ref
	}

	return &schema
}

func fieldToSchema(pkg string, field *Field) *jsonschema.Schema {
	schema := jsonschema.Schema{}

	if field.Text != nil {
		// if there is an explicit schema, use it
		if field.Text.Schema != nil {
			schema = field.Text.Schema.Schema
		}

		// if no title is provided on the explicit schema, grab it from the comment
		if schema.Title == "" {
			schema.Title = strings.ReplaceAll(field.Tag, "\\n", "\n")
		}

		populateDescriptionFields(field.Text.Description, &schema)

		// if an explicit schema was provided, return it
		if field.Text.Schema != nil {
			return &schema
		}
	}

	// schema was not explicitly provided, generate it from the comment

	info := fieldToDefinitionInfo(pkg, field)

	if info.typeInfo.ref != "" {
		schema.Ref = info.typeInfo.ref
	}

	if info.enumValues != nil {
		schema.Enum = info.enumValues
	}

	if info.typeInfo.typeName != "" {
		schema.Type = info.typeInfo.typeName
	}

	if info.arrayItemsTypeInfo != nil {
		schema.Items = typeInfoToSchema(info.arrayItemsTypeInfo)
	}

	if info.mapValueTypeInfo != nil {
		schema.PatternProperties = map[string]*jsonschema.Schema{
			".*": typeInfoToSchema(info.mapValueTypeInfo),
		}
	}

	return &schema
}

func populateDescriptionFields(description string, schema *jsonschema.Schema) {
	if schema.Extras == nil {
		schema.Extras = make(map[string]any)
	}

	markdownDescription := normalizeDescription(description)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	htmlDescription := string(markdown.ToHTML([]byte(markdownDescription), nil, renderer))

	policy := bluemonday.StrictPolicy()

	plaintextDescription := policy.Sanitize(htmlDescription)

	// set description
	if schema.Description == "" {
		schema.Description = plaintextDescription
	}

	// set markdownDescription for vscode/monaco editor
	if schema.Extras["markdownDescription"] == nil {
		schema.Extras["markdownDescription"] = markdownDescription
	}

	// set htmlDescription for Jetbrains IDEs
	if schema.Extras["x-intellij-html-description"] == nil {
		schema.Extras["x-intellij-html-description"] = htmlDescription
	}
}

func structToSchema(pkg string, st *Struct, allStructs []*Struct) *jsonschema.Schema {
	schema := jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
	}

	var requiredFields []string

	properties := orderedmap.New[string, *jsonschema.Schema]()

	if st.Text != nil && st.Text.SchemaMeta != "" {
		parts := strings.Split(st.Text.SchemaMeta, "/")
		if len(parts) != 2 {
			log.Fatalf("invalid schema meta: %s", st.Text.SchemaMeta)
		}

		apiVersionVal := parts[0]
		kindVal := parts[1]

		apiVersionSchema := &jsonschema.Schema{
			Title: "apiVersion",
			Enum:  []any{apiVersionVal},
		}

		kindSchema := &jsonschema.Schema{
			Title: "kind",
			Enum:  []any{kindVal},
		}

		populateDescriptionFields("apiVersion is the API version of the resource.", apiVersionSchema)
		populateDescriptionFields("kind is the kind of the resource.", kindSchema)

		properties.Set("apiVersion", apiVersionSchema)
		properties.Set("kind", kindSchema)

		requiredFields = append(requiredFields, "apiVersion", "kind")
	}

	for _, field := range st.Fields {
		if field.Inline {
			var inlinedStruct *Struct

			for _, otherStruct := range allStructs {
				if otherStruct.Name == field.Type {
					inlinedStruct = otherStruct

					break
				}
			}

			if inlinedStruct != nil {
				for _, inlineField := range inlinedStruct.Fields {
					if inlineField.Tag == "" {
						// skip unknown/untagged field
						continue
					}

					if inlineField.Text != nil && inlineField.Text.SchemaRequired {
						requiredFields = append(requiredFields, inlineField.Tag)
					}

					properties.Set(inlineField.Tag, fieldToSchema(pkg, inlineField))
				}
			}
		}

		if field.Tag == "" {
			// skip unknown/untagged field
			continue
		}

		if field.Text != nil && field.Text.SchemaRequired {
			requiredFields = append(requiredFields, field.Tag)
		}

		properties.Set(field.Tag, fieldToSchema(pkg, field))
	}

	slices.Sort(requiredFields)

	schema.Properties = properties
	schema.Required = requiredFields

	if st.Text.Description != "" {
		schema.Description = st.Text.Description
	}

	return &schema
}

func docsToSchema(docs []*Doc, schemaURL string) *jsonschema.Schema {
	schema := jsonschema.Schema{
		Version:     jsonschema.Version,
		ID:          jsonschema.ID(schemaURL),
		Definitions: make(jsonschema.Definitions),
	}

	for _, doc := range docs {
		for _, docStruct := range doc.Structs {
			name := doc.Package + "." + docStruct.Name

			if docStruct.Text != nil && docStruct.Text.SchemaRoot {
				schema.OneOf = append(schema.OneOf, &jsonschema.Schema{
					Ref: "#/$defs/" + name,
				})
			}

			schema.Definitions[name] = structToSchema(doc.Package, docStruct, doc.Structs)
		}
	}

	return &schema
}

func renderSchema(docs []*Doc, destinationFile, versionTagFile string) {
	version := readMajorMinorVersion(versionTagFile)
	schemaFileName := filepath.Base(destinationFile)
	schemaURL := fmt.Sprintf(ConfigSchemaURLFormat, version, schemaFileName)

	schema := docsToSchema(docs, schemaURL)

	marshaled, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal schema: %v", err)
	}

	validateSchema(string(marshaled), schemaURL)

	destDir := filepath.Dir(destinationFile)

	if err = os.MkdirAll(destDir, 0o755); err != nil {
		log.Fatalf("failed to create destination directory %q: %v", destDir, err)
	}

	err = os.WriteFile(destinationFile, marshaled, 0o644)
	if err != nil {
		log.Fatalf("failed to write schema to %s: %v", destinationFile, err)
	}
}

// validateSchema validates the schema itself by compiling it.
func validateSchema(schema, schemaURL string) {
	_, err := validatejsonschema.CompileString(schemaURL, schema)
	if err != nil {
		log.Fatalf("failed to compile schema: %v", err)
	}
}

func normalizeDescription(description string) string {
	description = strings.ReplaceAll(description, `\n`, "\n")
	description = strings.ReplaceAll(description, `\"`, `"`)
	description = strings.TrimSpace(description)

	return description
}

func readMajorMinorVersion(filePath string) string {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read version file: %v", err)
	}

	version := string(fileBytes)

	versionParts := strings.Split(version, ".")

	if len(versionParts) < 2 {
		log.Fatalf("unexpected version in version file: %s", version)
	}

	return versionParts[0] + "." + versionParts[1]
}
