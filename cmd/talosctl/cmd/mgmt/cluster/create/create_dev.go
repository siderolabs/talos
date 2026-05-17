// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/siderolabs/go-kubeconfig"
	"k8s.io/client-go/tools/clientcmd"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

//nolint:gocyclo,cyclop
func createDevCluster(ctx context.Context, cOps clusterops.Common, qOps clusterops.Qemu) error {
	provisioner, err := selectProvisioner(ctx, cOps)
	if err != nil {
		return err
	}

	if rp, ok := provisioner.(*remote.Provisioner); ok {
		// Delegating to a remote-provision server: target the server's
		// architecture (it runs the VMs), and skip the local download —
		// boot assets are uploaded or fetched server-side.
		arch, archErr := rp.ServerArch(ctx)
		if archErr != nil {
			return archErr
		}

		qOps.TargetArch = arch

		// Resolve ${ARCH} now: the QEMU provisioner substitutes it
		// server-side, but the client-side artifact upload needs real
		// paths to read.
		for _, p := range []*string{
			&qOps.NodeVmlinuzPath,
			&qOps.NodeInitramfsPath,
			&qOps.NodeISOPath,
			&qOps.NodeUSBPath,
			&qOps.NodeUKIPath,
			&qOps.NodeDiskImagePath,
		} {
			*p = strings.ReplaceAll(*p, constants.ArchVariable, arch)
		}
	} else if err := downloadBootAssets(ctx, &qOps); err != nil {
		return err
	}

	if cOps.TalosVersion == "" {
		parts := strings.Split(qOps.NodeInstallImage, ":")
		cOps.TalosVersion = parts[len(parts)-1]
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

	// Create and save the talosctl configuration file.
	err = postCreate(ctx, cOps, cluster, clusterConfigs)
	if err != nil {
		return err
	}

	return clustercmd.ShowCluster(cluster)
}

func saveConfig(talosConfigObj *clientconfig.Config, talosconfigPath string) (err error) {
	c, err := clientconfig.Open(talosconfigPath)
	if err != nil {
		return fmt.Errorf("error opening talos config: %w", err)
	}

	renames := c.Merge(talosConfigObj)
	for _, rename := range renames {
		fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
	}

	return c.Save(talosconfigPath)
}

func mergeKubeconfig(ctx context.Context, clusterAccess *access.Adapter) error {
	kubeconfigPath, err := kubeconfig.SinglePath()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nmerging kubeconfig into %q\n", kubeconfigPath)

	k8sconfig, err := clusterAccess.Kubeconfig(ctx)
	if err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}

	kubeConfig, err := clientcmd.Load(k8sconfig)
	if err != nil {
		return fmt.Errorf("error parsing kubeconfig: %w", err)
	}

	if clusterAccess.ForceEndpoint != "" {
		for name := range kubeConfig.Clusters {
			kubeConfig.Clusters[name].Server = clusterAccess.ForceEndpoint
		}
	}

	_, err = os.Stat(kubeconfigPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		return clientcmd.WriteToFile(*kubeConfig, kubeconfigPath)
	}

	merger, err := kubeconfig.Load(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error loading existing kubeconfig: %w", err)
	}

	err = merger.Merge(kubeConfig, kubeconfig.MergeOptions{
		ActivateContext: true,
		OutputWriter:    os.Stdout,
		ConflictHandler: func(component kubeconfig.ConfigComponent, name string) (kubeconfig.ConflictDecision, error) {
			return kubeconfig.RenameDecision, nil
		},
	})
	if err != nil {
		return fmt.Errorf("error merging kubeconfig: %w", err)
	}

	return merger.Write(kubeconfigPath)
}
