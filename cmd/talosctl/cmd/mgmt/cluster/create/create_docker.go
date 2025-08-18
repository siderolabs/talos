// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"fmt"
	"net/netip"
	"strings"

	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

type clusterCreateRequestData struct {
	clusterRequest   provision.ClusterRequest
	provisionOptions []provision.Option
	talosconfig      *clientconfig.Config
}

//nolint:gocyclo,cyclop
func getDockerClusterRequest(
	cOps commonOps,
	dOps dockerOps,
	provisioner provision.Provisioner,
) (clusterCreateRequestData, error) {
	parts := strings.Split(dOps.talosImage, ":")
	cOps.talosVersion = parts[len(parts)-1]

	_, err := config.ParseContractFromVersion(cOps.talosVersion)
	if err != nil {
		currentVersion := helpers.GetTag()
		cOps.talosVersion = currentVersion
		fmt.Printf("failed to derrive Talos version from the docker image, defaulting to %s\n", currentVersion)
	}

	controlplaneResources, err := parseResources(cOps.controlplaneResources)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("error parsing controlplane resources: %s", err)
	}

	workerResources, err := parseResources(cOps.workerResources)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("error parsing worker resources: %s", err)
	}

	cidr, err := getCidr4(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	nodeIPs, err := getIps(cidr, cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	gatewayIP, err := sideronet.NthIPInNetwork(cidr, gatewayOffset)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	clusterRequest := getBaseClusterRequest(cOps, []netip.Prefix{cidr}, []netip.Addr{gatewayIP})

	var genOptions []generate.Option

	registryMirrorOps, err := getRegistryMirrorGenOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	genOptions = append(genOptions, registryMirrorOps...)
	genOptions = append(genOptions, provisioner.GenOptions(clusterRequest.Network)...)

	versionContractGenOps, _, err := getVersionContractGenOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	genOptions = append(genOptions, versionContractGenOps...)
	genOptions = append(genOptions, provisioner.GenOptions(clusterRequest.Network)...)

	endpointList := provisioner.GetTalosAPIEndpoints(clusterRequest.Network)
	genOptions = append(genOptions, getWithAdditionalSubjectAltNamesGenOps(endpointList)...)
	genOptions = append(genOptions, generate.WithEndpointList(endpointList))

	configBundleOpts := []bundle.Option{}
	configBundleOpts = append(configBundleOpts,
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: cOps.rootOps.ClusterName,
				Endpoint:    provisioner.GetInClusterKubernetesControlPlaneEndpoint(clusterRequest.Network, cOps.controlPlanePort),
				KubeVersion: strings.TrimPrefix(cOps.kubernetesVersion, "v"),
				GenOptions:  genOptions,
			}),
	)

	provisionOptions := []provision.Option{
		provision.WithDockerPortsHostIP(dOps.hostIP),
		provision.WithKubernetesEndpoint(provisioner.GetExternalKubernetesControlPlaneEndpoint(clusterRequest.Network, cOps.controlPlanePort)),
	}

	if dOps.ports != "" {
		portList := strings.Split(dOps.ports, ",")
		provisionOptions = append(provisionOptions, provision.WithDockerPorts(portList))
	}

	configPatchBundleOps, err := getConfigPatchBundleOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	configBundleOpts = append(configBundleOpts, configPatchBundleOps...)

	configBundle, err := bundle.NewBundle(configBundleOpts...)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	bundleTalosconfig := configBundle.TalosConfig()

	provisionOptions = append(provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))

	controlplanes, workers, err := createNodeRequests(cOps, controlplaneResources, workerResources, [][]netip.Addr{nodeIPs})
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	for i := range controlplanes {
		controlplanes[i].Config = configBundle.ControlPlane()
	}

	for i := range workers {
		workers[i].Config = configBundle.Worker()
	}

	clusterRequest.Nodes = append(clusterRequest.Nodes, controlplanes...)
	clusterRequest.Nodes = append(clusterRequest.Nodes, workers...)

	for i := range clusterRequest.Nodes {
		clusterRequest.Nodes[i].Mounts = dOps.mountOpts.Value()
	}

	clusterRequest.Network.DockerDisableIPv6 = dOps.disableIPv6
	clusterRequest.Image = dOps.talosImage

	return clusterCreateRequestData{
		clusterRequest:   clusterRequest,
		provisionOptions: provisionOptions,
		talosconfig:      bundleTalosconfig,
	}, nil
}
