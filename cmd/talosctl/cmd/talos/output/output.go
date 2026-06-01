// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package output provides writers in different formats.
package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"
)

// Writer interface.
type Writer interface {
	WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error
	WriteResource(node string, r resource.Resource, event state.EventType) error
	Flush() error
}

// NewWriter builds writer from type.
func NewWriter(format string) (Writer, error) {
	writer := os.Stdout

	switch {
	case format == "table":
		return NewTable(writer), nil
	case format == "yaml":
		return NewYAML(writer), nil
	case format == "json":
		return NewJSON(writer), nil
	case strings.HasPrefix(format, "jsonpath="):
		path := format[len("jsonpath="):]

		jp := jsonpath.New("talos")

		if err := jp.Parse(path); err != nil {
			return nil, fmt.Errorf("error parsing jsonpath: %w", err)
		}

		return NewJSONPath(writer, jp), nil
	default:
		return nil, fmt.Errorf("output format %q is not supported", format)
	}
}

// CompleteOutputArg represents tab completion for `--output` argument.
func CompleteOutputArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"json", "table", "yaml", "jsonpath"}, cobra.ShellCompDirectiveNoFileComp
}
