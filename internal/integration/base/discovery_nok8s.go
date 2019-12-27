// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration,!integration_k8s

package base

import "github.com/talos-systems/talos/cmd/osctl/pkg/client"

func discoverNodesK8s(client *client.Client, suite *TalosSuite) ([]string, error) {
	return nil, nil
}
