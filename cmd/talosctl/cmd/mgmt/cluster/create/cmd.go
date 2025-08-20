// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	workersFlagName           = "workers"
	controlplanesFlagName     = "controlplanes"
	kubernetesVersionFlagName = "kubernetes-version"
	registryMirrorFlagName    = "registry-mirror"
	networkMTUFlagName        = "mtu"

	// user facing command flags.
	talosconfigDestinationFlagName = "talosconfig-destination"
)

// commonOps are the options that are not specific to a single provider.
type commonOps struct {
	// RootOps are the options from the root cluster command
	rootOps                   *clustercmd.CmdOps
	talosconfigDestination    string
	registryMirrors           []string
	registryInsecure          []string
	kubernetesVersion         string
	applyConfigEnabled        bool
	configDebug               bool
	networkCIDR               string
	networkMTU                int
	networkIPv4               bool
	dnsDomain                 string
	workers                   int
	controlplanes             int
	controlplaneResources     nodeResources
	workerResources           nodeResources
	clusterWait               bool
	clusterWaitTimeout        time.Duration
	forceInitNodeAsEndpoint   bool
	forceEndpoint             string
	inputDir                  string
	controlPlanePort          int
	withInitNode              bool
	customCNIUrl              string
	skipKubeconfig            bool
	skipInjectingConfig       bool
	talosVersion              string
	enableKubeSpan            bool
	enableClusterDiscovery    bool
	configPatch               []string
	configPatchControlPlane   []string
	configPatchWorker         []string
	kubePrismPort             int
	skipK8sNodeReadinessCheck bool
	withJSONLogs              bool
	wireguardCIDR             string
	withUUIDHostnames         bool
}

func addTalosconfigDestinationFlag(flagset *pflag.FlagSet, bind *string, flagName string) {
	flagset.StringVar(bind, flagName, "",
		fmt.Sprintf("The location to save the generated Talos configuration file to. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
}

func addControlplaneCpusFlag(flagset *pflag.FlagSet, bind *string, flagName string) {
	flagset.StringVar(bind, flagName, "2.0", "the share of CPUs as fraction (each control plane/VM)")
}

func addWorkersCpusFlag(flagset *pflag.FlagSet, bind *string, flagName string) {
	flagset.StringVar(bind, flagName, "2.0", "the share of CPUs as fraction (each worker/VM)")
}

func addControlPlaneMemoryFlag(flagset *pflag.FlagSet, bind *int, flagName string) {
	flagset.IntVar(bind, flagName, 2048, "the limit on memory usage in MB (each control plane/VM)")
}

func addWorkersMemoryFlag(flagset *pflag.FlagSet, bind *int, flagName string) {
	flagset.IntVar(bind, flagName, 2048, "the limit on memory usage in MB (each worker/VM)")
}

func addConfigPatchFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
}

func addConfigPatchControlPlaneFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
}

func addConfigPatchWorkerFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to 'worker' type)")
}

func addWorkersFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, workersFlagName, 1, "the number of workers to create")
}

func addControlplanesFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, controlplanesFlagName, 1, "the number of controlplanes to create")
}

func addKubernetesVersionFlag(flagset *pflag.FlagSet, bind *string) {
	flagset.StringVar(bind, kubernetesVersionFlagName, constants.DefaultKubernetesVersion, "desired kubernetes version to run")
}

func addRegistryMirrorFlag(flagset *pflag.FlagSet, bind *[]string) {
	flagset.StringSliceVar(bind, registryMirrorFlagName, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
}

func addNetworkMTUFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, networkMTUFlagName, 1500, "MTU of the cluster network")
}
