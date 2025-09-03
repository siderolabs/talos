// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

//nolint:gocyclo,cyclop
func getQemuClusterRequest(
	ctx context.Context,
	cOps commonOps,
	qOps qemuOps,
	cqOps createQemuOps,
	provisioner provision.Provisioner,
) (clusterCreateRequestData, error) {
	if cOps.talosVersion == "" || cOps.talosVersion[0] != 'v' {
		return clusterCreateRequestData{}, fmt.Errorf("failed to parse talos version: version string must start with a 'v'")
	}

	_, err := config.ParseContractFromVersion(cOps.talosVersion)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("failed to parse talos version: %s", err)
	}

	if cqOps.schematicID == "" {
		cqOps.schematicID = emptySchemanticID
	}

	factoryURL, err := url.Parse(cqOps.imageFactoryURL)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("malformed Image Factory URL: %q: %w", cqOps.imageFactoryURL, err)
	}

	if factoryURL.Scheme == "" || factoryURL.Host == "" {
		return clusterCreateRequestData{}, fmt.Errorf("image Factory URL must include scheme and host: %q", cqOps.imageFactoryURL)
	}

	qOps.nodeISOPath, err = url.JoinPath(factoryURL.String(), "image", cqOps.schematicID, cOps.talosVersion, "metal-"+qOps.targetArch+".iso")
	cli.Should(err)
	qOps.nodeInstallImage, err = url.JoinPath(factoryURL.Host, "metal-installer", cqOps.schematicID+":"+cOps.talosVersion)
	cli.Should(err)

	if err := downloadBootAssets(ctx, &qOps); err != nil {
		return clusterCreateRequestData{}, err
	}

	return createClusterRequest(createClusterRequestOps{
		commonOps:   cOps,
		provisioner: provisioner,
		withExtraGenOpts: func(cr provision.ClusterRequest) []generate.Option {
			genOptions := []generate.Option{
				generate.WithInstallImage(qOps.nodeInstallImage),
			}

			endpointList := xslices.Map(cr.Nodes.ControlPlaneNodes(), func(n provision.NodeRequest) string { return n.IPs[0].String() })

			genOptions = append(genOptions, generate.WithEndpointList(endpointList))

			return genOptions
		},
		withExtraProvisionOpts: func(cr provision.ClusterRequest) []provision.Option {
			return []provision.Option{
				provision.WithUEFI(qOps.uefiEnabled),
				provision.WithTargetArch(qOps.targetArch),
				provision.WithSiderolinkAgent(qOps.withSiderolinkAgent.IsEnabled()),
			}
		},
		modifyClusterRequest: func(cr provision.ClusterRequest) (provision.ClusterRequest, error) {
			nameserverIPs, err := getNameserverIPs(qOps)
			if err != nil {
				return cr, err
			}

			cr.Network.Nameservers = nameserverIPs
			cr.ISOPath = qOps.nodeISOPath

			cr.Network.CNI = provision.CNIConfig{
				BinPath:  qOps.cniBinPath,
				ConfDir:  qOps.cniConfDir,
				CacheDir: qOps.cniCacheDir,

				BundleURL: qOps.cniBundleURL,
			}

			return cr, nil
		},
		modifyNodes: func(cr provision.ClusterRequest, cp, w []provision.NodeRequest) (controlplanes, workers []provision.NodeRequest, err error) {
			primaryDisks, workerDisks, err := getDisks(qOps)
			if err != nil {
				return nil, nil, err
			}

			for i := range cp {
				cp[i].Disks = primaryDisks
			}

			for i := range w {
				w[i].Disks = slices.Concat(primaryDisks, workerDisks)
			}

			return cp, w, nil
		},
	})
}
