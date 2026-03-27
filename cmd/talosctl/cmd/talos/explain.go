// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/meta/spec"
	"github.com/spf13/cobra"
	yaml "go.yaml.in/yaml/v4"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var explainCmdFlags struct {
	insecure  bool
	namespace string
}

// explainCmd represents the explain command.
var explainCmd = &cobra.Command{
	Use:   "explain <type>[.field[.field...]]",
	Short: "Explain a Talos resource type",
	Long: `Show detailed information about a Talos resource type, similar to 'kubectl explain'.
Displays the resource definition metadata including type, default namespace,
aliases, and field information.

Supports dot notation to drill into nested fields:
  talosctl explain links            - show top-level resource fields
  talosctl explain links.spec       - show spec fields
  talosctl explain links.bondMaster - shorthand for links.spec.bondMaster

Note: Field descriptions are not available in the COSI resource model.
Spec field names and types are inferred from a sample resource on the node.

Use 'talosctl get rd' to see all available resource types.`,
	Example: `  talosctl explain links
  talosctl explain links.spec
  talosctl explain links.spec.bondMaster
  talosctl explain MachineStatuses
  talosctl explain VolumeStatuses.block.talos.dev`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeResourceDefinition(toComplete != "")
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if explainCmdFlags.insecure {
			return WithClientMaintenance(nil, explainResource(args[0]))
		}

		return WithClient(explainResource(args[0]))
	},
}

// resolveExplainArg splits the argument into a resource type and an optional field path.
// It first tries to resolve the full argument as a resource type. If that fails,
// it splits on the first dot and tries the prefix as the resource type.
func resolveExplainArg(ctx context.Context, c *client.Client, namespace *string, arg string) (*meta.ResourceDefinition, []string, error) {
	savedNamespace := *namespace

	rd, err := c.ResolveResourceKind(ctx, namespace, arg)
	if err == nil {
		return rd, nil, nil
	}

	*namespace = savedNamespace

	dotIdx := strings.IndexByte(arg, '.')
	if dotIdx == -1 {
		return nil, nil, err
	}

	resourceType := arg[:dotIdx]
	fieldPath := arg[dotIdx+1:]

	rd, resolveErr := c.ResolveResourceKind(ctx, namespace, resourceType)
	if resolveErr != nil {
		return nil, nil, fmt.Errorf("could not resolve resource type %q: %w", resourceType, resolveErr)
	}

	path := strings.Split(fieldPath, ".")

	return rd, path, nil
}

// normalizeFieldPath ensures the path starts with "metadata" or "spec".
// If it starts with neither, "spec" is prepended as a shorthand.
func normalizeFieldPath(path []string) []string {
	if len(path) == 0 {
		return path
	}

	if path[0] != "metadata" && path[0] != "spec" {
		return append([]string{"spec"}, path...)
	}

	return path
}

//nolint:gocyclo,cyclop
func explainResource(arg string) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		if err := helpers.ClientVersionCheck(ctx, c); err != nil {
			return err
		}

		// COSI methods don't support one-to-many proxying, so pin to first node
		md, _ := metadata.FromOutgoingContext(ctx)
		nodes := md.Get("nodes")

		if len(nodes) == 0 {
			nodes = []string{""}
		}

		nodeCtx := client.WithNode(ctx, nodes[0])
		namespace := explainCmdFlags.namespace

		rd, fieldPath, err := resolveExplainArg(nodeCtx, c, &namespace, arg)
		if err != nil {
			return err
		}

		rdSpec := rd.TypedSpec()
		fieldPath = normalizeFieldPath(fieldPath)

		// Print resource header
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "RESOURCE:\t%s <%s>\n", rdSpec.DisplayType, rdSpec.Type)
		fmt.Fprintf(w, "NAMESPACE:\t%s\n", rdSpec.DefaultNamespace)

		if len(fieldPath) == 0 {
			fmt.Fprintf(w, "ID:\t%s\n", rd.Metadata().ID())

			if len(rdSpec.Aliases) > 0 {
				fmt.Fprintf(w, "ALIASES:\t%s\n", strings.Join(rdSpec.Aliases, ", "))
			}

			sensitivity := "non-sensitive"
			if rdSpec.Sensitivity == meta.Sensitive {
				sensitivity = "sensitive"
			}

			fmt.Fprintf(w, "SENSITIVITY:\t%s\n", sensitivity)
		}

		w.Flush() //nolint:errcheck

		// Show FIELD line when navigating into a path
		if len(fieldPath) > 0 {
			fmt.Printf("\nFIELD: %s\n", strings.Join(fieldPath, "."))
		}

		// Show print columns only at top level
		if len(fieldPath) == 0 && len(rdSpec.PrintColumns) > 0 {
			fmt.Println()
			fmt.Println("PRINT COLUMNS:")

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(tw, "  NAME\tJSON PATH\n")

			for _, col := range rdSpec.PrintColumns {
				fmt.Fprintf(tw, "  %s\t%s\n", col.Name, col.JSONPath)
			}

			tw.Flush() //nolint:errcheck
		}

		// Show fields based on path
		fmt.Println()
		fmt.Println("FIELDS:")

		switch {
		case len(fieldPath) == 0:
			printTopLevelFields()
		case fieldPath[0] == "metadata":
			printMetadataFields(fieldPath[1:])
		default:
			// spec path: strip the "spec" prefix and navigate
			printSpecFieldsAtPath(nodeCtx, c, rdSpec, namespace, fieldPath[1:])
		}

		return nil
	}
}

func printTopLevelFields() {
	fmt.Println("  metadata\t<object>")
	fmt.Println("    Resource metadata (namespace, type, id, version, phase, owner, labels, annotations, finalizers).")
	fmt.Println()
	fmt.Println("  spec\t<object>")
	fmt.Println("    Resource specification.")
}

// metadataFieldDefs defines the static metadata fields common to all COSI resources.
var metadataFieldDefs = []fieldInfo{
	{Name: "namespace", Type: "string"},
	{Name: "type", Type: "string"},
	{Name: "id", Type: "string"},
	{Name: "version", Type: "string"},
	{Name: "phase", Type: "string"},
	{Name: "owner", Type: "string"},
	{Name: "created", Type: "timestamp"},
	{Name: "updated", Type: "timestamp"},
	{Name: "labels", Type: "map[string]string"},
	{Name: "annotations", Type: "map[string]string"},
	{Name: "finalizers", Type: "[]string"},
}

func printMetadataFields(path []string) {
	if len(path) == 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		for _, f := range metadataFieldDefs {
			fmt.Fprintf(tw, "  %s\t<%s>\n", f.Name, f.Type)
		}

		tw.Flush() //nolint:errcheck

		return
	}

	for _, f := range metadataFieldDefs {
		if f.Name == path[0] {
			fmt.Printf("  %s\t<%s>\n", f.Name, f.Type)

			return
		}
	}

	fmt.Printf("  error: field %q not found in metadata\n", strings.Join(path, "."))
}

func printSpecFieldsAtPath(ctx context.Context, c *client.Client, rdSpec *spec.ResourceDefinitionSpec, namespace string, path []string) {
	specMap := fetchSpecMap(ctx, c, rdSpec.Type, namespace)
	if specMap == nil {
		fmt.Println("  (no resources found on node to determine spec fields)")

		return
	}

	// Navigate to the requested path
	var current any = specMap

	for i, component := range path {
		m, ok := current.(map[string]any)
		if !ok {
			fmt.Printf("  error: %q is not an object, cannot navigate further\n", strings.Join(path[:i], "."))

			return
		}

		val, exists := m[component]
		if !exists {
			fmt.Printf("  error: field %q not found in spec\n", strings.Join(path[:i+1], "."))

			return
		}

		current = val
	}

	// Show one level of fields at the current position
	switch v := current.(type) {
	case map[string]any:
		fields := extractFields(v)
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		for _, f := range fields {
			fmt.Fprintf(tw, "  %s\t<%s>\n", f.Name, f.Type)
		}

		tw.Flush() //nolint:errcheck
	default:
		typeName := inferYAMLType(current)

		if len(path) > 0 {
			fmt.Printf("  %s is a <%s> field\n", path[len(path)-1], typeName)
		} else {
			fmt.Printf("  (spec is a <%s>)\n", typeName)
		}
	}
}

func fetchSpecMap(ctx context.Context, c *client.Client, resourceType string, namespace string) map[string]any {
	items, err := c.COSI.List(ctx,
		resource.NewMetadata(namespace, resourceType, "", resource.VersionUndefined),
	)
	if err != nil || len(items.Items) == 0 {
		return nil
	}

	specData, err := yaml.Marshal(items.Items[0].Spec())
	if err != nil {
		return nil
	}

	var specMap map[string]any
	if err := yaml.Unmarshal(specData, &specMap); err != nil {
		return nil
	}

	return specMap
}

type fieldInfo struct {
	Name string
	Type string
}

func extractFields(m map[string]any) []fieldInfo {
	fields := make([]fieldInfo, 0, len(m))

	for name, value := range m {
		fields = append(fields, fieldInfo{
			Name: name,
			Type: inferYAMLType(value),
		})
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields
}

func inferYAMLType(value any) string {
	if value == nil {
		return "object"
	}

	switch v := value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int:
		return "integer"
	case int64:
		return "integer"
	case uint64:
		return "integer"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}

		return "number"
	case []any:
		if len(v) > 0 {
			elemType := inferYAMLType(v[0])

			return "[]" + elemType
		}

		return "[]object"
	case map[string]any:
		return "Object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func init() {
	explainCmd.Flags().StringVar(&explainCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	explainCmd.Flags().BoolVarP(&explainCmdFlags.insecure, "insecure", "i", false, "explain resources using the insecure (encrypted with no auth) maintenance service")
	addCommand(explainCmd)
}
