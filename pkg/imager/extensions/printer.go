// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/siderolabs/talos/internal/pkg/extensions"
)

func (builder *Builder) printExtensions(extensions []*extensions.Extension) error {
	builder.Printf("discovered system extensions:")

	var b bytes.Buffer

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprint(w, "NAME\tVERSION\tAUTHOR\n")

	for _, ext := range extensions {
		fmt.Fprintf(w, "%s\t%s\t%s\n", ext.Manifest.Metadata.Name, ext.Manifest.Metadata.Version, ext.Manifest.Metadata.Author)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for {
		line, err := b.ReadString('\n')
		if err != nil {
			break
		}

		builder.Printf("%s", line)
	}

	return nil //nolint:nilerr
}
