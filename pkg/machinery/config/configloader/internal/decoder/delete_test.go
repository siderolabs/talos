// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader/internal/decoder"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

var (
	//go:embed testdata/delete/delete.yaml
	patchDelete []byte
	//go:embed testdata/delete/delete_expected.yaml
	patchDeleteExpected []byte
)

func TestExtractDeletes(t *testing.T) {
	result, b := must.Values(extractDeletes(patchDelete))(t)

	defer func() {
		if !t.Failed() {
			return
		}

		for _, sel := range result {
			t.Logf("%#v", sel)
		}
	}()

	require.Equal(t, string(patchDeleteExpected), string(b))

	expected := strings.Join(
		[]string{
			"{apiVersion:v1alpha1, kind:SideroLinkConfig, idx:0}",
			"{path:configFiles.[0], apiVersion:v1alpha1, kind:ExtensionServiceConfig, key:content, value:hello, idx:1, name:foo}",
			"{path:machine.hostname, kind:v1alpha1, idx:2}",
			"{path:machine.network.[0], kind:v1alpha1, key:interface, value:eth0, idx:2}",
		},
		"\n",
	)

	actual := strings.Join(
		xslices.Map(result, func(sel config.Document) string { return sel.(fmt.Stringer).String() }),
		"\n",
	)

	require.Equal(t, expected, actual)
}

func extractDeletes(in []byte) (result []config.Document, _ []byte, err error) {
	var cleanedBytes [][]byte

	dec := yaml.NewDecoder(bytes.NewReader(in))

	for i := 0; ; i++ {
		node := &yaml.Node{}

		err = dec.Decode(node)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, nil, err
		}

		result, err = decoder.AppendDeletesTo(node, result, i)
		if err != nil {
			return nil, nil, err
		}

		if !node.IsZero() {
			b, err := encoder.NewEncoder(node, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			if err != nil {
				return nil, nil, err
			}

			cleanedBytes = append(cleanedBytes, b)
		}
	}

	return result, bytes.Join(cleanedBytes, []byte("---\n")), nil
}
