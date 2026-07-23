// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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

	if err := container.validateContainer(mode); err != nil {
		multiErr = multierror.Append(multiErr, err)
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

	return warnings, multiErr.ErrorOrNil()
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
//nolint:gocyclo,cyclop
func (container *Container) validateContainer(mode validation.RuntimeMode) error {
	var errs error

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

	// KubeSpan requires a cluster identity, provided either by the deprecated .cluster.id/.cluster.secret
	// or by a DiscoveryIdentityConfig document. The identity may live in a separate document, so this
	// cross-document check is done at the container level.
	if kubeSpanConfig := container.NetworkKubeSpanConfig(); kubeSpanConfig != nil && kubeSpanConfig.Enabled() {
		identity := container.DiscoveryIdentityConfig()

		if identity == nil || identity.ClusterID() == "" {
			errs = multierror.Append(errs, fmt.Errorf("cluster ID (.cluster.id or DiscoveryIdentityConfig) should be set when .machine.network.kubespan is enabled"))
		}

		if identity == nil || identity.ClusterSecret() == "" {
			errs = multierror.Append(errs, fmt.Errorf("cluster secret (.cluster.secret or DiscoveryIdentityConfig) should be set when .machine.network.kubespan is enabled"))
		}
	}

	// control plane specific checks
	if container.Machine() != nil && container.Machine().Type().IsControlPlane() {
		hasLegacyEtcdEncryptionConfig := container.Cluster() != nil && (container.Cluster().SecretboxEncryptionSecret() != "" || container.Cluster().AESCBCEncryptionSecret() != "")
		hasKubeEtcdEncryptionConfig := container.K8sEtcdEncryptionConfig() != nil

		if !hasLegacyEtcdEncryptionConfig && !hasKubeEtcdEncryptionConfig {
			errs = multierror.Append(errs, fmt.Errorf("etcd encryption config is required for control plane machines"))
		}
	}

	// machine type specific checks
	var machineType machine.Type

	if container.Machine() != nil {
		machineType = container.Machine().Type()
	}

	controlplaneDocs := findMatchingDocs[ControlplaneOnlyConfig](container.documents)

	if len(controlplaneDocs) > 0 && !machineType.IsControlPlane() {
		kinds := xslices.Map(controlplaneDocs, func(d ControlplaneOnlyConfig) string {
			return d.Kind()
		})
		slices.Sort(kinds)
		kinds = slices.Compact(kinds)

		errs = multierror.Append(errs,
			fmt.Errorf(
				"the following document kinds are only allowed on control plane machines: %v",
				kinds,
			),
		)
	}

	return errs
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
