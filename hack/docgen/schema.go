// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/iancoleman/orderedmap"
	"github.com/invopop/jsonschema"
	validatejsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/siderolabs/gen/slices"
)

const ConfigSchemaURLFormat = "https://talos.dev/%s/schemas/v1alpha1_config.schema.json"

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

func goTypeToTypeInfo(goType string) *SchemaTypeInfo {
	switch goType {
	case "string":
		return &SchemaTypeInfo{typeName: "string"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return &SchemaTypeInfo{typeName: "integer"}
	case "bool":
		return &SchemaTypeInfo{typeName: "boolean"}
	default:
		return &SchemaTypeInfo{ref: "#/$defs/" + goType}
	}
}

func fieldToDefinitionInfo(field *Field) SchemaDefinitionInfo {
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
			arrayItemsTypeInfo: goTypeToTypeInfo(strings.TrimPrefix(goType, "[]")),
		}
	}

	if strings.HasPrefix(goType, "map[string]") {
		return SchemaDefinitionInfo{
			typeInfo:         SchemaTypeInfo{typeName: "object"},
			mapValueTypeInfo: goTypeToTypeInfo(strings.TrimPrefix(goType, "map[string]")),
		}
	}

	return SchemaDefinitionInfo{
		typeInfo: *goTypeToTypeInfo(goType),
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

func fieldToSchema(field *Field) *jsonschema.Schema {
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

		// if no description is provided on the explicit schema, grab it from the comment
		if schema.Description == "" {
			schema.Description = normalizeDescription(field.Text.Description)
		}

		// if an explicit schema was provided, return it
		if field.Text.Schema != nil {
			return &schema
		}
	}

	// schema was not explicitly provided, generate it from the comment

	info := fieldToDefinitionInfo(field)

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

func structToSchema(st *Struct) *jsonschema.Schema {
	schema := jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
	}

	properties := orderedmap.New()

	for _, field := range st.Fields {
		if field.Tag == "" {
			// skip unknown/untagged field
			continue
		}
		properties.Set(field.Tag, fieldToSchema(field))
	}

	schema.Properties = properties

	return &schema
}

func docToSchema(doc *Doc, schemaURL string) *jsonschema.Schema {
	schema := jsonschema.Schema{
		Version: jsonschema.Version,
		ID:      jsonschema.ID(schemaURL),
		Ref:     "#/$defs/Config",
	}

	schema.Definitions = slices.ToMap(doc.Structs, func(st *Struct) (string, *jsonschema.Schema) {
		return st.Name, structToSchema(st)
	})

	return &schema
}

func renderSchema(doc *Doc, destinationFile, versionTagFile string) {
	version := readMajorMinorVersion(versionTagFile)

	schemaURL := fmt.Sprintf(ConfigSchemaURLFormat, version)

	schema := docToSchema(doc, schemaURL)

	marshaled, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal schema: %v", err)
	}

	validateSchema(string(marshaled), schemaURL)

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
