// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/constants"
	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/preset"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

//nolint:gocyclo,cyclop
func createQemuCluster(
	ctx context.Context,
	qOps clusterops.Qemu,
	cOps clusterops.Common,
	presetOptions presetOptions,
	provisioner provision.Provisioner,
) error {
	if cOps.TalosVersion == "" || cOps.TalosVersion[0] != 'v' {
		return fmt.Errorf("failed to parse talos version: version string must start with a 'v'")
	}

	_, err := config.ParseContractFromVersion(cOps.TalosVersion)
	if err != nil {
		return fmt.Errorf("failed to parse talos version: %s", err)
	}

	if presetOptions.schematicID == "" {
		presetOptions.schematicID = constants.ImageFactoryEmptySchematicID
	}

	factoryURL, err := url.Parse(presetOptions.imageFactoryURL)
	if err != nil {
		return fmt.Errorf("malformed Image Factory URL: %q: %w", presetOptions.imageFactoryURL, err)
	}

	if factoryURL.Scheme == "" || factoryURL.Host == "" {
		return fmt.Errorf("image Factory URL must include scheme and host: %q", presetOptions.imageFactoryURL)
	}

	if slices.Contains(presetOptions.presets, preset.Maintenance{}.Name()) && cOps.OmniAPIEndpoint != "" {
		fmt.Println("omni-api-endpoint specified along with the 'maintenance' preset")
		fmt.Println("machine configuration containing 'SideroLinkConfig' will be written to the working path but will not be applied to the nodes")
	}

	err = preset.Apply(
		preset.Options{
			SchematicID:     presetOptions.schematicID,
			ImageFactoryURL: factoryURL,
		}, &cOps, &qOps, presetOptions.presets)
	if err != nil {
		return err
	}

	if err := downloadBootAssets(ctx, &qOps); err != nil {
		return err
	}

	clusterConfigs, err := configmaker.GetQemuConfigs(configmaker.QemuOptions{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: provisioner,
	})
	if err != nil {
		return err
	}

	err = preCreate(cOps, clusterConfigs)
	if err != nil {
		return err
	}

	cluster, err := provisioner.Create(ctx, clusterConfigs.ClusterRequest, clusterConfigs.ProvisionOptions...)
	if err != nil {
		return err
	}

	err = postCreate(ctx, cOps, cluster, clusterConfigs)
	if err != nil {
		return err
	}

	return clustercmd.ShowCluster(cluster)
}

func preCreate(cOps clusterops.Common, clusterConfigs clusterops.ClusterConfigs) error {
	// write machine config
	if cOps.SkipInjectingConfig {
		if err := writeMachineconfig(clusterConfigs, cOps); err != nil {
			return err
		}
	}

	return nil
}

func writeMachineconfig(clusterConfigs clusterops.ClusterConfigs, cOps clusterops.Common) error {
	if clusterConfigs.ConfigBundle != nil {
		types := []machine.Type{machine.TypeControlPlane, machine.TypeWorker}

		if cOps.WithInitNode {
			types = slices.Insert(types, 0, machine.TypeInit)
		}

		return clusterConfigs.ConfigBundle.Write(".", encoder.CommentsAll, types...)
	}

	// no configbundle, just write the machine config as-is
	cfgBytes, err := clusterConfigs.ClusterRequest.Nodes[0].Config.Bytes()
	if err != nil {
		return err
	}

	fullFilePath := filepath.Join(".", "machineconfig.yaml")
	if err = os.WriteFile(fullFilePath, cfgBytes, 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "created %s\n", fullFilePath)

	return nil
}
