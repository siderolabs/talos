// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"context"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ValidateAsClient validates the config in the client context (outside of Talos).
//
// The method returns warnings and fatal errors (as multierror).
func (container *Container) ValidateAsClient(mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	return container.validate(mode, opt...)
}

// ValidateAtRuntime validates the config in the runtime context (inside Talos).
//
// This method performs full machine configuration validation, including validation which requires runtime context.
//
// The method returns warnings and fatal errors (as multierror).
func (container *Container) ValidateAtRuntime(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	warnings, err := container.validate(mode, opt...)

	extraWarnings, extraErr := container.runtimeValidate(ctx, st, mode, opt...)

	warnings = slices.Concat(warnings, extraWarnings)

	var multiErr *multierror.Error

	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	if extraErr != nil {
		multiErr = multierror.Append(multiErr, extraErr)
	}

	return warnings, multiErr.ErrorOrNil()
}

// validate checks configuration and returns warnings and fatal errors (as multierror).
//
// It performs validation which can be done client-side (outside of Talos) and is used by both ValidateAsClient and ValidateAtRuntime.
//
// The validation first validates each individual document, then it does conflict validation of new
// documents with v1alpha1.Config (if it exists).
// Finally, whole container is validated according to the mode.
//
//nolint:gocyclo
func (container *Container) validate(mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	var (
		warnings []string
		err      error
	)

	if container.v1alpha1Config != nil {
		warnings, err = container.v1alpha1Config.Validate(mode, opt...)
		if err != nil {
			err = fmt.Errorf("v1alpha1.Config: %w", err)
		}
	}

	var multiErr *multierror.Error

	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	for _, doc := range container.documents {
		if validatableDoc, ok := doc.(config.Validator); ok {
			docWarnings, docErr := validatableDoc.Validate(mode, opt...)
			if docErr != nil {
				docErr = fmt.Errorf("%s: %w", docID(doc), docErr)
			}

			warnings = append(warnings, docWarnings...)
			multiErr = multierror.Append(multiErr, docErr)
		}
	}

	// now cross-validate the config
	if container.v1alpha1Config != nil {
		for _, doc := range container.documents {
			if conflictValidator, ok := doc.(V1Alpha1ConflictValidator); ok {
				err := conflictValidator.V1Alpha1ConflictValidate(container.v1alpha1Config)
				if err != nil {
					multiErr = multierror.Append(multiErr, err)
				}
			}
		}
	}

	containerWarnings, containerErr := container.validateContainer(mode)

	warnings = append(warnings, containerWarnings...)

	if containerErr != nil {
		multiErr = multierror.Append(multiErr, containerErr)
	}

	return warnings, multiErr.ErrorOrNil()
}

// runtimeValidate validates the config in the runtime context.
//
// The method returns warnings and fatal errors (as multierror).
func (container *Container) runtimeValidate(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	var (
		warnings []string
		err      error
	)

	if container.v1alpha1Config != nil {
		warnings, err = container.v1alpha1Config.RuntimeValidate(ctx, st, mode, opt...)
		if err != nil {
			err = fmt.Errorf("v1alpha1.Config: %w", err)
		}
	}

	var multiErr *multierror.Error

	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	for _, doc := range container.documents {
		if validatableDoc, ok := doc.(config.RuntimeValidator); ok {
			docWarnings, docErr := validatableDoc.RuntimeValidate(ctx, st, mode, opt...)
			if docErr != nil {
				docErr = fmt.Errorf("%s: %w", docID(doc), docErr)
			}

			warnings = append(warnings, docWarnings...)
			multiErr = multierror.Append(multiErr, docErr)
		}
	}

	if err := container.runtimeValidateContainer(ctx, st); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	cpuWarnings, err := container.runtimeValidateCPUPartition(ctx, st, mode)
	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	return append(warnings, cpuWarnings...), multiErr.ErrorOrNil()
}

// runtimeValidateCPUPartition checks the bounded roots against the online CPUs, and warns when the
// policy differs from the one the node booted with: it only takes effect after a reboot.
func (container *Container) runtimeValidateCPUPartition(ctx context.Context, st state.State, mode validation.RuntimeMode) ([]string, error) {
	if mode.InContainer() {
		return nil, nil
	}

	declared := ProjectCPUPartition(container.CPUPartitionConfig())

	if err := validateCPUPartitionCPUs(ctx, st, declared.Roots); err != nil {
		return nil, err
	}

	boot, err := safe.StateGetByID[*runtime.CPUPartitionSpec](ctx, st, runtime.CPUPartitionSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	if sameCPUPartition(boot.TypedSpec(), &declared) {
		return nil, nil
	}

	warnings := []string{"CPUPartitionConfig: the CPU partition policy changes take effect only after a reboot"}

	// The kubelet keeps the policy of the boot, which validateCPUPartitionKubelet doesn't see once the
	// written policy leaves kubepods unbounded: a conflicting kubelet configuration isn't rendered at all.
	if _, bounded := boot.TypedSpec().Roots[constants.CgroupKubepods]; bounded && container.kubeletEligible() {
		kubeletConfig := container.K8sKubeletConfig()

		if err := kubelet.ValidateCPUReservationConfiguration(kubeletConfig.ExtraConfig(), kubeletConfig.ExtraArgs()); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"CPUPartitionConfig: the kubelet configuration conflicts with the CPU partition policy of the boot and is not updated until a reboot: %s", err))
		}
	}

	return warnings, nil
}

// validateCPUPartitionCPUs requires every bounded root to name online CPUs only, and an online CPU
// outside a bounded kubepods root: the kubelet static CPU manager policy refuses to start without a
// reserved CPU. Slices need no check, the document validation keeps them within their root and
// reports an invalid list.
func validateCPUPartitionCPUs(ctx context.Context, st state.State, roots map[string]string) error {
	if len(roots) == 0 {
		return nil
	}

	topology, err := safe.StateGetByID[*hardware.NUMATopology](ctx, st, hardware.NUMATopologyID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	online := cpuset.New(xslices.Map(topology.TypedSpec().OnlineCPUs, func(id uint32) int { return int(id) })...)

	var errs *multierror.Error

	for _, root := range slices.Sorted(maps.Keys(roots)) {
		bound, err := cpuset.Parse(roots[root])
		if err != nil {
			continue
		}

		if offline := bound.Difference(online); !offline.IsEmpty() {
			errs = multierror.Append(errs, fmt.Errorf("CPUPartitionConfig: root %s names offline CPUs %q", root, offline))
		}

		if root == constants.CgroupKubepods && online.Difference(bound).IsEmpty() {
			errs = multierror.Append(errs, fmt.Errorf("CPUPartitionConfig: kubepods.cpus %q leaves no online CPU outside kubepods, "+
				"which the kubelet static CPU manager policy requires", roots[root]))
		}
	}

	return errs.ErrorOrNil()
}

// ProjectCPUPartition is the CPUPartitionSpec a CPU partition configuration declares; nil declares no policy.
func ProjectCPUPartition(partition configconfig.CPUPartitionConfig) runtime.CPUPartitionSpecSpec {
	if partition == nil {
		return runtime.CPUPartitionSpecSpec{}
	}

	spec := runtime.CPUPartitionSpecSpec{Roots: map[string]string{}}

	for root, cpus := range partition.Roots() {
		spec.Roots[string(root)] = cpus
	}

	for _, slice := range partition.Slices() {
		spec.Slices = append(spec.Slices, runtime.CPUPartitionSliceSpec{Name: slice.Name(), CPUs: slice.CPUs(), Exclusive: slice.Exclusive()})
	}

	return spec
}

// sameCPUPartition compares CPU sets, not their spelling; slices are named and disjoint, so their order is irrelevant.
func sameCPUPartition(a, b *runtime.CPUPartitionSpecSpec) bool {
	byName := func(x, y runtime.CPUPartitionSliceSpec) int { return strings.Compare(x.Name, y.Name) }

	sameCPUs := func(x, y string) bool {
		xs, errX := cpuset.Parse(x)
		ys, errY := cpuset.Parse(y)

		if errX != nil || errY != nil {
			return x == y
		}

		return xs.Equals(ys)
	}

	sameSlice := func(x, y runtime.CPUPartitionSliceSpec) bool {
		return x.Name == y.Name && x.Exclusive == y.Exclusive && sameCPUs(x.CPUs, y.CPUs)
	}

	return maps.EqualFunc(a.Roots, b.Roots, sameCPUs) &&
		slices.EqualFunc(slices.SortedFunc(slices.Values(a.Slices), byName), slices.SortedFunc(slices.Values(b.Slices), byName), sameSlice)
}

// runtimeValidateContainer validates the full configuration container in the runtime context.
//
// This is the runtime-context counterpart to validateContainer: it performs validation which only
// makes sense for the whole configuration (vs. individual documents) and which requires runtime
// state. In particular, it detects a promotable system volume (ETCD, CRI, KUBELET, LOG) whose backing
// VolumeConfig document has been removed while the volume is still backed by a live dedicated
// partition. The per-document VolumeConfig.RuntimeValidate cannot catch this because a removed
// document is no longer part of the container.
func (container *Container) runtimeValidateContainer(ctx context.Context, st state.State) error {
	var errs *multierror.Error

	volumes := container.Volumes()

	for _, name := range configconfig.PromotableSystemVolumeNames {
		if _, present := volumes.ByName(name); present {
			// the document is still present: VolumeConfig.RuntimeValidate handles any backing conflict.
			continue
		}

		volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, st, name)
		if err != nil {
			if state.IsNotFoundError(err) {
				// the volume was never established (cluster creation / boot): nothing to conflict with.
				continue
			}

			return err
		}

		// only compare against a settled volume; an in-flight volume is not yet established.
		if volumeStatus.TypedSpec().Phase != block.VolumePhaseReady {
			continue
		}

		if volumeStatus.TypedSpec().Type == block.VolumeTypePartition {
			errs = multierror.Append(errs, fmt.Errorf(
				"the %q system volume is backed by a dedicated partition and its VolumeConfig cannot be removed; "+
					"migrating a system volume off a dedicated partition is not supported",
				name,
			))
		}
	}

	return errs.ErrorOrNil()
}

// validateContainer validates the full configuration container.
//
// This validation is used to do validation which only makes sense for the full configuration (vs. individual documents).
//
// The method returns warnings and fatal errors (as multierror).
//
//nolint:gocyclo,cyclop
func (container *Container) validateContainer(mode validation.RuntimeMode) ([]string, error) {
	var (
		warnings []string
		errs     error
	)

	// A cpufreq attribute set both by CPUScalingConfig and through sysfs has two controllers writing
	// it, each reverting the other on every reconcile. Reject that rather than let the machine flap.
	if len(container.CPUScalingConfigs()) > 0 {
		for key := range container.SysfsConfig() {
			if strings.HasPrefix(key, "devices.system.cpu.") && strings.Contains(key, ".cpufreq.") {
				errs = multierror.Append(errs, fmt.Errorf(
					"sysfs key %q conflicts with CPUScalingConfig: configure CPU frequency scaling through one of them, not both",
					key,
				))
			}
		}
	}

	if mode.InContainer() {
		// in container mode, HostDNS must be enabled and forward KubeDNS to host must be enabled as well
		hostDNSConfig := container.NetworkHostDNSConfig()

		if hostDNSConfig == nil {
			errs = multierror.Append(errs, fmt.Errorf("hostDNS config is required in container mode"))
		} else {
			if !hostDNSConfig.HostDNSEnabled() {
				errs = multierror.Append(errs, fmt.Errorf("hostDNS must be enabled in container mode"))
			}

			if !hostDNSConfig.ForwardKubeDNSToHost() {
				errs = multierror.Append(errs, fmt.Errorf("forwardKubeDNSToHost must be enabled in container mode"))
			}
		}
	}

	// DNS protocols besides plain UDP/TCP can't be used without HostDNS
	if dnsConfig := container.NetworkResolverConfig(); dnsConfig != nil {
		hasNonDefaultDNS := false

		for _, ns := range dnsConfig.Resolvers() {
			if ns.Protocol != nethelpers.DNSProtocolDefault {
				hasNonDefaultDNS = true
			}
		}

		if hasNonDefaultDNS {
			hostDNSConfig := container.NetworkHostDNSConfig()
			if hostDNSConfig == nil || !hostDNSConfig.HostDNSEnabled() {
				errs = multierror.Append(errs, fmt.Errorf("hostDNS must be enabled when using non-default DNS protocols"))
			}
		}
	}

	// Discovery requires a cluster identity
	if discoveryConfigs := container.DiscoveryServiceConfigs(); len(discoveryConfigs) > 0 {
		identity := container.DiscoveryIdentityConfig()

		if identity == nil || identity.ClusterID() == "" {
			errs = multierror.Append(errs, fmt.Errorf("cluster ID (.cluster.id or DiscoveryIdentityConfig) should be set when cluster discovery (DiscoveryServiceConfig) is enabled"))
		}

		if identity == nil || identity.ClusterSecret() == "" {
			errs = multierror.Append(errs, fmt.Errorf("cluster secret (.cluster.secret or DiscoveryIdentityConfig) should be set when cluster discovery (DiscoveryServiceConfig) is enabled"))
		}
	}

	// The kernel keeps a single route per table, family, destination and metric, so routes sharing that key
	// (e.g. default routes on two links without distinct metrics) can't be installed together.
	// A per-document Validate() cannot see the other documents, so this is a container-level check.
	warnings = append(warnings, container.validateRouteKeys()...)

	// Container dependencies must form a DAG, and must only reference containers that exist.
	// A per-document Validate() cannot see the other documents, so this is a container-level check.
	if err := validateContainerDependencies(container.ContainerConfigs()); err != nil {
		errs = multierror.Append(errs, err)
	}

	// Container volume mounts must name a declared volume, and mount a read-only one read-only.
	// A per-document Validate() cannot see the other documents, so this is a container-level check.
	if err := validateContainerVolumeMounts(container); err != nil {
		errs = multierror.Append(errs, err)
	}

	// Container localhost seccomp profiles must name a declared seccomp profile.
	// A per-document Validate() cannot see the other documents, so this is a container-level check.
	if err := validateContainerSeccompProfiles(container); err != nil {
		errs = multierror.Append(errs, err)
	}

	// Container raw volume devices must name a declared raw volume, handed to a single container.
	// A per-document Validate() cannot see the other documents, so this is a container-level check.
	if err := validateContainerRawVolumeDevices(container); err != nil {
		errs = multierror.Append(errs, err)
	}

	// A volume backs a single content library, and must not be read-only. A per-document Validate()
	// cannot see the other documents, so this is a container-level check.
	if err := validateContentLibraryBackingVolumes(container); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := container.validateStoragePools(); err != nil {
		errs = multierror.Append(errs, err)
	}

	// Talos owns the container and VM roots even without workload limits; kubelet must not write their memory.max.
	if err := container.validateWorkloadResourceKubelet(mode); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := container.validateCPUPartitionKubelet(mode); err != nil {
		errs = multierror.Append(errs, err)
	}

	// Pools remain runtime dependencies. Libraries are only provisioning sources:
	// copy controllers report missing sources or adopt existing copies without them.
	if err := validateVirtualMachinePoolReferences(container); err != nil {
		errs = multierror.Append(errs, err)
	}

	if !mode.InContainer() {
		if err := validateVirtualMachineCPUSlices(container); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	// KubeSpan requires a cluster identity, provided either by the deprecated .cluster.id/.cluster.secret
	// or by a DiscoveryIdentityConfig document. The identity may live in a separate document, so this
	// cross-document check is done at the container level.
	if kubeSpanConfig := container.NetworkKubeSpanConfig(); kubeSpanConfig != nil && kubeSpanConfig.Enabled() {
		discoveryEnabled := len(container.DiscoveryServiceConfigs()) > 0

		if !discoveryEnabled {
			errs = multierror.Append(errs, fmt.Errorf("KubeSpan requires cluster discovery to be enabled"))
		}
	}

	// machine type specific checks
	var machineType machine.Type

	if container.Machine() != nil {
		machineType = container.Machine().Type()
	}

	// control plane specific checks
	if machineType.IsControlPlane() {
		// switching here on api-server CA config instead of api-server config,
		// as due to backwards compatibility reasons, the empty legacy `.cluster.apiServer` config
		// resolves to "default api-server".
		if container.K8sAPIServerCAConfig() != nil {
			hasLegacyEtcdEncryptionConfig := container.Cluster() != nil && (container.Cluster().SecretboxEncryptionSecret() != "" || container.Cluster().AESCBCEncryptionSecret() != "")
			hasKubeEtcdEncryptionConfig := container.K8sEtcdEncryptionConfig() != nil

			if !hasLegacyEtcdEncryptionConfig && !hasKubeEtcdEncryptionConfig {
				errs = multierror.Append(errs, fmt.Errorf("etcd encryption config is required for control plane machines running kube-apiserver"))
			}
		}

		// the legacy .machine.nodeLabels/.machine.nodeTaints are amended with the control plane role
		// label and taint for control plane machines, while the KubeNodeConfig document is used as-is,
		// so warn if the control plane role label is missing after the migration
		//
		// the taint is not checked, as skipping it is a valid way to allow scheduling on control planes
		if nodeConfigs := findMatchingDocs[configconfig.K8sNodeConfig](container.documents); len(nodeConfigs) > 0 {
			nodeConfig := nodeConfigs[0]

			_, hasControlPlaneLabel := nodeConfig.Labels()[constants.LabelNodeRoleControlPlane]

			// if the node is not registered in Kubernetes, the labels and taints are not used at all
			if !hasControlPlaneLabel && !nodeConfig.SkipNodeRegistration() {
				warnings = append(warnings, fmt.Sprintf(
					"KubeNodeConfig document should set the %q node label on control plane machines "+
						"(and, unless scheduling on control planes is allowed, the %q taint): "+
						"unlike .machine.nodeLabels/.machine.nodeTaints, the document contents are used as-is",
					constants.LabelNodeRoleControlPlane,
					constants.LabelNodeRoleControlPlane+": "+constants.TaintEffectNoSchedule,
				))
			}
		}
	}

	// if Kubernetes is configured for this machine (there is a Kubernetes CA), the cluster name and
	// endpoint are required: they might be provided either by the legacy .cluster.controlPlane.endpoint
	// or by the KubeClusterConfig document, so this is a cross-document check
	//
	// configs generated with Kubernetes disabled have neither, so they are not affected
	if container.K8sAPIServerCAConfig() != nil && container.K8sClusterConfig() == nil {
		errs = multierror.Append(errs, fmt.Errorf("cluster name and endpoint are required when Kubernetes is configured: "+
			"either .cluster.clusterName/.cluster.controlPlane.endpoint or the KubeClusterConfig document"))
	}

	controlplaneDocs := findMatchingDocs[ControlplaneOnlyConfig](container.documents)

	if len(controlplaneDocs) > 0 && !machineType.IsControlPlane() {
		kinds := xslices.Map(controlplaneDocs, func(d ControlplaneOnlyConfig) string {
			return d.Kind()
		})
		slices.Sort(kinds)
		kinds = slices.Compact(kinds)

		errs = multierror.Append(
			errs,
			fmt.Errorf(
				"the following document kinds are only allowed on control plane machines: %v",
				kinds,
			),
		)
	}

	return warnings, errs
}

// validateWorkloadResourceKubelet protects Talos-managed roots and active workload limits from competing kubelet writers.
func (container *Container) validateWorkloadResourceKubelet(mode validation.RuntimeMode) error {
	if mode.InContainer() || !container.kubeletEligible() {
		return nil
	}

	kubeletConfig := container.K8sKubeletConfig()
	workloadResources := container.WorkloadResourceConfig()

	if workloadResources == nil ||
		(workloadResources.KubepodsMemoryLimit() == 0 && workloadResources.TalosContainersMemoryLimit() == 0 && workloadResources.VirtualMachinesMemoryLimit() == 0) {
		return kubelet.ValidateWorkloadRootReservations(kubeletConfig.ExtraConfig(), kubeletConfig.ExtraArgs())
	}

	kubepodsLimit := workloadResources.KubepodsMemoryLimit()

	var multiErr *multierror.Error

	if kubepodsLimit > 0 {
		multiErr = multierror.Append(multiErr, kubelet.ValidateMemoryLimitConfiguration(kubeletConfig.ExtraConfig(), kubeletConfig.ExtraArgs()))
	}

	multiErr = multierror.Append(multiErr, kubelet.ValidateWorkloadRootConfiguration(kubeletConfig.ExtraConfig(), kubeletConfig.ExtraArgs()))

	return multiErr.ErrorOrNil()
}

// validateCPUPartitionKubelet rejects user reserved CPUs while the CPU partition policy bounds kubepods,
// which derives them. It checks the configuration as written; the running kubelet follows the
// policy captured at boot until a reboot.
func (container *Container) validateCPUPartitionKubelet(mode validation.RuntimeMode) error {
	if mode.InContainer() || !container.kubeletEligible() {
		return nil
	}

	partition := container.CPUPartitionConfig()
	if partition == nil {
		return nil
	}

	if _, bounded := partition.Roots()[constants.CgroupKubepods]; !bounded {
		return nil
	}

	kubeletConfig := container.K8sKubeletConfig()

	return kubelet.ValidateCPUReservationConfiguration(kubeletConfig.ExtraConfig(), kubeletConfig.ExtraArgs())
}

// kubeletEligible mirrors the inputs the kubelet configuration controller requires before it renders anything.
func (container *Container) kubeletEligible() bool {
	return container.Cluster() != nil && container.Machine() != nil &&
		container.K8sNetworkConfig() != nil && container.K8sNodeConfig() != nil && container.K8sKubeletConfig() != nil
}

// Validate is the legacy validation method.
//
// Deprecated: use ValidateAsClient instead for client-side validation (outside of Talos).
func (container *Container) Validate(mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	return container.validate(mode, opt...)
}

// RuntimeValidate is the legacy runtime validation method.
//
// Deprecated: use ValidateAtRuntime instead for runtime validation (inside Talos).
func (container *Container) RuntimeValidate(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	return container.runtimeValidate(ctx, st, mode, opt...)
}

// validateRouteKeys warns about the routes which resolve to the same kernel route.
//
// The kernel keeps a single route per table, family, destination and metric: when several routes in the
// configuration share that key, only one of them is going to be installed, so the configuration is ambiguous.
//
//nolint:gocyclo
func (container *Container) validateRouteKeys() []string {
	var warnings []string

	owners := map[string]string{}

	check := func(table nethelpers.RoutingTable, family nethelpers.Family, destination netip.Prefix, metric uint32, owner string) {
		key := network.RouteID(table, family, destination, metric)

		destinationStr := "default"

		if destination.Bits() > 0 {
			destinationStr = destination.Masked().String()
		}

		if previous, ok := owners[key]; ok {
			warnings = append(warnings, fmt.Sprintf(
				"%s and %s both configure the %s route to %s with metric %d in table %s, only one of them is going to be installed",
				previous, owner, family, destinationStr, metric, table,
			))

			return
		}

		owners[key] = owner
	}

	for _, linkConfig := range container.NetworkCommonLinkConfigs() {
		for i, route := range linkConfig.Routes() {
			destination := route.Destination().ValueOrZero()
			gateway := route.Gateway().ValueOrZero()

			family := nethelpers.FamilyInet4

			if (gateway.IsValid() && gateway.Is6()) || (destination.IsValid() && destination.Addr().Is6()) {
				family = nethelpers.FamilyInet6
			}

			check(
				route.Table().ValueOr(nethelpers.TableMain),
				family,
				destination,
				route.Metric().ValueOr(network.DefaultRouteMetric),
				fmt.Sprintf("link %q route %d", linkConfig.Name(), i),
			)
		}
	}

	for _, blackholeRouteConfig := range container.NetworkBlackholeRouteConfigs() {
		destination, err := netip.ParsePrefix(blackholeRouteConfig.Name())
		if err != nil {
			// validated by the document itself
			continue
		}

		family := nethelpers.FamilyInet4

		if destination.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		check(
			nethelpers.TableMain,
			family,
			destination,
			blackholeRouteConfig.Metric().ValueOr(network.DefaultRouteMetric),
			fmt.Sprintf("blackhole route %q", blackholeRouteConfig.Name()),
		)
	}

	return warnings
}
