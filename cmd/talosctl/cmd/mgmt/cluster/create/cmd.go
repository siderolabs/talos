// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package create provides way to create talos clusters
package create

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/bytesize"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	workersFlagName           = "workers"
	controlplanesFlagName     = "controlplanes"
	kubernetesVersionFlagName = "kubernetes-version"
	registryMirrorFlagName    = "registry-mirror"
	networkMTUFlagName        = "mtu"
	networkCIDRFlagName       = "cidr"
	talosVersionFlagName      = "talos-version"

	// Flags that have been renamed in the user-facing commands.
	controlPlaneCpusFlagName        = "cpus-controlplanes"
	controlPlaneMemoryFlagName      = "memory-controlplanes"
	workersCpusFlagName             = "cpus-workers"
	workersMemoryFlagName           = "memory-workers"
	configPatchFlagName             = "config-patch"
	configPatchControlPlaneFlagName = "config-patch-controlplanes"
	configPatchWorkerFlagName       = "config-patch-workers"
	talosconfigDestinationFlagName  = "talosconfig-destination"

	// Qemu flags.
	disksFlagName = "disks"
)

// commonOps are the options that are not specific to a single provider.
type commonOps struct {
	// rootOps are the options from the root cluster command
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

func getDefaultCommonOptions() commonOps {
	memory2GB := bytesize.WithDefaultUnit("MiB")
	cli.Should(memory2GB.Set("2.0GiB"))
	defaultResources := nodeResources{
		cpu:    "2.0",
		memory: *memory2GB,
	}

	return commonOps{
		controlplanes:         1,
		controlplaneResources: defaultResources,
		workers:               1,
		workerResources:       defaultResources,

		networkCIDR:            "10.5.0.0/24",
		kubernetesVersion:      constants.DefaultKubernetesVersion,
		networkMTU:             1500,
		clusterWaitTimeout:     20 * time.Minute,
		clusterWait:            true,
		dnsDomain:              "cluster.local",
		controlPlanePort:       constants.DefaultControlPlanePort,
		rootOps:                &clustercmd.PersistentFlags,
		networkIPv4:            true,
		kubePrismPort:          constants.DefaultKubePrismPort,
		enableClusterDiscovery: true,
		talosVersion:           helpers.GetTag(),
	}
}

func getCommonUserFacingFlags(pointer *commonOps) *pflag.FlagSet {
	common := pflag.NewFlagSet("common", pflag.PanicOnError)

	addWorkersFlag(common, &pointer.workers)
	addKubernetesVersionFlag(common, &pointer.kubernetesVersion)
	addTalosconfigDestinationFlag(common, &pointer.talosconfigDestination, talosconfigDestinationFlagName)
	addConfigPatchFlag(common, &pointer.configPatch, configPatchFlagName)
	addConfigPatchControlPlaneFlag(common, &pointer.configPatchControlPlane, configPatchControlPlaneFlagName)
	addConfigPatchWorkerFlag(common, &pointer.configPatchWorker, configPatchWorkerFlagName)

	addControlplaneCpusFlag(common, &pointer.controlplaneResources.cpu, controlPlaneCpusFlagName)
	addWorkersCpusFlag(common, &pointer.workerResources.cpu, workersCpusFlagName)
	addControlPlaneMemoryFlag(common, &pointer.controlplaneResources.memory, controlPlaneMemoryFlagName)
	addWorkersMemoryFlag(common, &pointer.workerResources.memory, workersMemoryFlagName)

	// The following flags are used in tests and development
	addNetworkMTUFlag(common, &pointer.networkMTU)
	cli.Should(common.MarkHidden(networkMTUFlagName))
	addRegistryMirrorFlag(common, &pointer.registryMirrors)
	cli.Should(common.MarkHidden(registryMirrorFlagName))

	return common
}

// Common flags

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
	flagset.StringVar(bind, flagName, *bind, "the share of CPUs as fraction for each control plane/VM")
}

func addWorkersCpusFlag(flagset *pflag.FlagSet, bind *string, flagName string) {
	flagset.StringVar(bind, flagName, *bind, "the share of CPUs as fraction for each worker/VM")
}

func addControlPlaneMemoryFlag(flagset *pflag.FlagSet, bind *bytesize.ByteSize, flagName string) {
	flagset.Var(bind, flagName, "the limit on memory usage for each control plane/VM")
}

func addWorkersMemoryFlag(flagset *pflag.FlagSet, bind *bytesize.ByteSize, flagName string) {
	flagset.Var(bind, flagName, "the limit on memory usage for each worker/VM")
}

func addConfigPatchFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
}

func addConfigPatchControlPlaneFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to 'controlplane' type)")
}

func addConfigPatchWorkerFlag(flagset *pflag.FlagSet, bind *[]string, flagName string) {
	flagset.StringArrayVar(bind, flagName, nil, "patch generated machineconfigs (applied to 'worker' type)")
}

func addWorkersFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, workersFlagName, *bind, "the number of workers to create")
}

func addControlplanesFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, controlplanesFlagName, *bind, "the number of controlplanes to create")
}

func addKubernetesVersionFlag(flagset *pflag.FlagSet, bind *string) {
	flagset.StringVar(bind, kubernetesVersionFlagName, *bind, "desired kubernetes version to run")
}

func addRegistryMirrorFlag(flagset *pflag.FlagSet, bind *[]string) {
	flagset.StringSliceVar(bind, registryMirrorFlagName, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
}

func addNetworkMTUFlag(flagset *pflag.FlagSet, bind *int) {
	flagset.IntVar(bind, networkMTUFlagName, *bind, "MTU of the cluster network")
}

func addTalosVersionFlag(flagset *pflag.FlagSet, bind *string, description string) {
	flagset.StringVar(bind, talosVersionFlagName, *bind, description)
}

// qemu flags

func addDisksFlag(flagset *pflag.FlagSet, bind *[]string, defaultVal []string) {
	flagset.StringSliceVar(bind, disksFlagName, defaultVal,
		`list of disks to create in format "<driver1>:<size1>" (disks after the first one are added only to worker machines)`)
}
