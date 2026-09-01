// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package output provides writers in different formats.
package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
)

// Writer interface.
type Writer interface {
	WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error
	WriteResource(node string, r resource.Resource, event state.EventType) error
	Flush() error
}

// NewWriter builds a writer over out, taking the raw stream so that the choice of
// which formats are filtered is testable.
//
// A resource comes from the node, so its ID, its spec and even the print columns
// of its definition are node-supplied. The JSON and YAML encoders escape control
// characters themselves and are handed the raw stream; the table and jsonpath
// writers render values verbatim and are handed the filtered one.
func NewWriter(format string, out io.Writer) (Writer, error) {
	filtered := safeout.NewWriter(out)

	switch {
	case format == "table":
		return filterFlusher{NewTable(filtered), filtered}, nil
	case format == "yaml":
		return NewYAML(out), nil
	case format == "json":
		return NewJSON(out), nil
	case strings.HasPrefix(format, "jsonpath="):
		path := format[len("jsonpath="):]

		jp := jsonpath.New("talos")

		if err := jp.Parse(path); err != nil {
			return nil, fmt.Errorf("error parsing jsonpath: %w", err)
		}

		// a jsonpath expression selecting a scalar renders it verbatim, unlike the
		// JSON branch of the same writer. The filter is the identity on printable
		// UTF-8, so `-o jsonpath=` stays usable from a script.
		return filterFlusher{NewJSONPath(filtered, jp), filtered}, nil
	default:
		return nil, fmt.Errorf("output format %q is not supported", format)
	}
}

// filterFlusher extends Flush down to the escaping stream, which otherwise holds
// on to the trailing bytes of a value that ends mid-rune.
type filterFlusher struct {
	Writer

	filter *safeout.Writer
}

func (f filterFlusher) Flush() error {
	if err := f.Writer.Flush(); err != nil {
		return err
	}

	return f.filter.Flush()
}

// CompleteOutputArg represents tab completion for `--output` argument.
func CompleteOutputArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"json", "table", "yaml", "jsonpath"}, cobra.ShellCompDirectiveNoFileComp
}
