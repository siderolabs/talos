// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nodename_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/nodename"
)

func TestFromHostname(t *testing.T) {
	for _, test := range []struct {
		hostname string

		expectedNodeName string
		expectedError    string
	}{
		{
			hostname: "foo",

			expectedNodeName: "foo",
		},
		{
			hostname: "foo_ია",

			expectedNodeName: "foo",
		},
		{
			hostname: "Node1",

			expectedNodeName: "node1",
		},
		{
			hostname: "MY_test_server_",

			expectedNodeName: "my-test-server",
		},
		{
			hostname: "123",

			expectedNodeName: "123",
		},
		{
			hostname: "-my-server-",

			expectedNodeName: "my-server",
		},
		{
			hostname: "კომპიუტერი",

			expectedError: "could not convert hostname \"კომპიუტერი\" to a valid Kubernetes Node name",
		},
		{
			hostname: "foo.bar.tld.",

			expectedNodeName: "foo.bar.tld",
		},
	} {
		t.Run(test.hostname, func(t *testing.T) {
			nodename, err := nodename.FromHostname(test.hostname)
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedNodeName, nodename)
			}
		})
	}
}
