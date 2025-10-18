// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package create provides way to create talos clusters
package create

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
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
	disksFlagName           = "disks"
	omniAPIEndpointFlagName = "omni-api-endpoint"
)

func getCommonUserFacingFlags(pointer *clusterops.Common) *pflag.FlagSet {
	common := pflag.NewFlagSet("common", pflag.PanicOnError)

	addWorkersFlag(common, &pointer.Workers)
	addKubernetesVersionFlag(common, &pointer.KubernetesVersion)
	addTalosconfigDestinationFlag(common, &pointer.TalosconfigDestination, talosconfigDestinationFlagName)
	addConfigPatchFlag(common, &pointer.ConfigPatch, configPatchFlagName)
	addConfigPatchControlPlaneFlag(common, &pointer.ConfigPatchControlPlane, configPatchControlPlaneFlagName)
	addConfigPatchWorkerFlag(common, &pointer.ConfigPatchWorker, configPatchWorkerFlagName)

	addControlplaneCpusFlag(common, &pointer.ControlplaneResources.CPU, controlPlaneCpusFlagName)
	addWorkersCpusFlag(common, &pointer.WorkerResources.CPU, workersCpusFlagName)
	addControlPlaneMemoryFlag(common, &pointer.ControlplaneResources.Memory, controlPlaneMemoryFlagName)
	addWorkersMemoryFlag(common, &pointer.WorkerResources.Memory, workersMemoryFlagName)

	// The following flags are used in tests and development
	addNetworkMTUFlag(common, &pointer.NetworkMTU)
	cli.Should(common.MarkHidden(networkMTUFlagName))
	addRegistryMirrorFlag(common, &pointer.RegistryMirrors)
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

func addDisksFlag(flagset *pflag.FlagSet, bind *flags.Disks) {
	flagset.Var(bind, disksFlagName,
		`list of disks to create in format "<driver1>:<size1>" (disks after the first one are added only to worker machines)`)
}

func addOmniJoinTokenFlag(cmd *cobra.Command, bindAPIEndpoint *string, cfgPatchAllFlagName, cfgPatchWorkersFlagName, cfgPatchCPsFlagName string) {
	cmd.Flags().StringVar(bindAPIEndpoint, omniAPIEndpointFlagName, *bindAPIEndpoint, "the Omni API endpoint (must include a scheme, a port and a join token)")

	cmd.MarkFlagsMutuallyExclusive(omniAPIEndpointFlagName, cfgPatchAllFlagName)
	cmd.MarkFlagsMutuallyExclusive(omniAPIEndpointFlagName, cfgPatchWorkersFlagName)
	cmd.MarkFlagsMutuallyExclusive(omniAPIEndpointFlagName, cfgPatchCPsFlagName)
}
