// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"net/url"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/provision"
)

//nolint:gocyclo,cyclop
func getQemuClusterRequest(
	ctx context.Context,
	qOps clusterops.Qemu,
	cOps clusterops.Common,
	cqOps createQemuOps,
	provisioner provision.Provisioner,
) (clusterops.ClusterConfigs, error) {
	if cOps.TalosVersion == "" || cOps.TalosVersion[0] != 'v' {
		return clusterops.ClusterConfigs{}, fmt.Errorf("failed to parse talos version: version string must start with a 'v'")
	}

	_, err := config.ParseContractFromVersion(cOps.TalosVersion)
	if err != nil {
		return clusterops.ClusterConfigs{}, fmt.Errorf("failed to parse talos version: %s", err)
	}

	if cqOps.schematicID == "" {
		cqOps.schematicID = emptySchemanticID
	}

	factoryURL, err := url.Parse(cqOps.imageFactoryURL)
	if err != nil {
		return clusterops.ClusterConfigs{}, fmt.Errorf("malformed Image Factory URL: %q: %w", cqOps.imageFactoryURL, err)
	}

	if factoryURL.Scheme == "" || factoryURL.Host == "" {
		return clusterops.ClusterConfigs{}, fmt.Errorf("image Factory URL must include scheme and host: %q", cqOps.imageFactoryURL)
	}

	qOps.NodeISOPath, err = url.JoinPath(factoryURL.String(), "image", cqOps.schematicID, cOps.TalosVersion, "metal-"+qOps.TargetArch+".iso")
	cli.Should(err)
	qOps.NodeInstallImage, err = url.JoinPath(factoryURL.Host, "metal-installer", cqOps.schematicID+":"+cOps.TalosVersion)
	cli.Should(err)

	if err := downloadBootAssets(ctx, &qOps); err != nil {
		return clusterops.ClusterConfigs{}, err
	}

	return configmaker.GetQemuConfigs(configmaker.QemuOptions{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: provisioner,
	})
}
