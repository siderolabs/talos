// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clustermaker

import (
	"context"
	"net/netip"
	"time"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

// PartialClusterRequest is the provision.ClusterRequest with only common provider options applied.
// PartialClusterRequest can be modified and then has to be handed to CreateCluster.
type PartialClusterRequest provision.ClusterRequest

// ClusterMaker is an abstraction around cluster creation.
type ClusterMaker interface {
	// GetPartialClusterRequest returns a partially rendered cluster request.
	// This request can be medified and then has to be passed to CreateCluster,
	// which adds final configs and creates the cluster.
	GetPartialClusterRequest() PartialClusterRequest
	GetVersionContract() *config.VersionContract
	GetCIDR4() netip.Prefix

	AddGenOps(opts ...generate.Option)
	AddProvisionOps(opts ...provision.Option)
	AddCfgBundleOpts(opts ...bundle.Option)

	// SetInClusterEndpoint can be optionally used to override the in cluster endpoint.
	SetInClusterEndpoint(endpoint string)

	// CreateCluster finalizes the clusterRequest by rendering and applying configs,
	// after which it creates the cluster via the provisioner.
	CreateCluster(ctx context.Context, request PartialClusterRequest) error
	PostCreate(ctx context.Context) error
}

// Options to make a cluster.
type Options struct {
	// RootOps are the options from the root cluster command
	RootOps                   *cluster.CmdOps
	Talosconfig               string
	RegistryMirrors           []string
	RegistryInsecure          []string
	KubernetesVersion         string
	ApplyConfigEnabled        bool
	ConfigDebug               bool
	NetworkCIDR               string
	NetworkMTU                int
	NetworkIPv4               bool
	DNSDomain                 string
	Workers                   int
	Controlplanes             int
	ControlPlaneCpus          string
	WorkersCpus               string
	ControlPlaneMemory        int
	WorkersMemory             int
	ClusterWait               bool
	ClusterWaitTimeout        time.Duration
	ForceInitNodeAsEndpoint   bool
	ForceEndpoint             string
	InputDir                  string
	ControlPlanePort          int
	WithInitNode              bool
	CustomCNIUrl              string
	SkipKubeconfig            bool
	SkipInjectingConfig       bool
	TalosVersion              string
	EnableKubeSpan            bool
	EnableClusterDiscovery    bool
	ConfigPatch               []string
	ConfigPatchControlPlane   []string
	ConfigPatchWorker         []string
	KubePrismPort             int
	SkipK8sNodeReadinessCheck bool
	WithJSONLogs              bool
	WireguardCIDR             string
	NetworkIPv6               bool
}
