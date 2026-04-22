// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// Validate checks configuration and returns warnings and fatal errors (as multierror).
//
// The validation first validates each individual document, then it does conflict validation of new
// documents with v1alpha1.Config (if it exists).
// Finally, whole container is validated according to the mode.
//
//nolint:gocyclo
func (container *Container) Validate(mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
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

// RuntimeValidate validates the config in the runtime context.
func (container *Container) RuntimeValidate(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
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

	return warnings, multiErr.ErrorOrNil()
}

// validateContainer validates the full configuration container.
//
// This validation is used to do validation which only makes sense for the full configuration (vs. individual documents).
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

	return errs
}
