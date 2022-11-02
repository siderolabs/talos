// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration && !integration_k8s

package base

import (
	"context"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func discoverNodesK8s(ctx context.Context, client *client.Client, suite *TalosSuite) (cluster.Info, error) {
	return nil, nil
}
