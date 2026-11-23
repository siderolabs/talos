// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ValidateRuntime checks the runtime, and that the rest of the document is compatible with it.
//
// Under runsc the container talks to the gVisor kernel, not the host one: privileges, devices and the
// host network namespace either do not carry over into the sandbox, or punch holes through it. Its
// sockets live in gVisor's own network stack too, out of reach of health checks, which connect from
// the host kernel's side of the container's network namespace.
func (c *ContainerConfigV1Alpha1) ValidateRuntime() error {
	switch c.Runtime() {
	case config.ContainerRuntimeRunc:
		return nil
	case config.ContainerRuntimeRunsc:
	default:
		return fmt.Errorf("unsupported runtime %q, expected runc or runsc", c.ContainerRuntimeName)
	}

	var validationErrors error

	security := c.Security()

	if security.Profile() == config.ContainerSecurityProfilePrivileged {
		validationErrors = errors.Join(validationErrors, errors.New("runtime runsc is not compatible with the privileged security profile"))
	}

	if security.MachinedAccess() {
		validationErrors = errors.Join(validationErrors, errors.New("runtime runsc is not compatible with security.machinedAccess"))
	}

	if len(c.DevicesConfig) > 0 {
		validationErrors = errors.Join(validationErrors, errors.New("runtime runsc is not compatible with devices"))
	}

	if c.Network().Mode() == config.ContainerNetworkModeHost {
		validationErrors = errors.Join(validationErrors, errors.New("runtime runsc is not compatible with the host network mode"))
	}

	if c.HealthCheck().IsPresent() {
		validationErrors = errors.Join(validationErrors, errors.New("runtime runsc is not compatible with lifecycle.healthCheck"))
	}

	return validationErrors
}

// RuntimeValidate implements config.RuntimeValidator interface.
//
// The extensions a node runs are baked into its boot image, so only the node itself knows whether
// the runtime a container asks for is installed.
//
// The same validation runs while the configuration is acquired at boot, possibly before the
// extensions are listed. Rejecting the configuration there would leave the node without one, so
// until the machine is past booting a missing extension is only a warning: the container waits for it.
func (c *ContainerConfigV1Alpha1) RuntimeValidate(ctx context.Context, st state.State, _ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	if c.Runtime() != config.ContainerRuntimeRunsc {
		return nil, nil
	}

	extensions, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("failed to list extensions: %w", err)
	}

	if runtime.ExtensionInstalled(extensions, constants.ContainerRuntimeRunscExtension) {
		return nil, nil
	}

	missing := fmt.Sprintf("container %q: runtime %s requires the %q system extension, which is not installed",
		c.MetaName, config.ContainerRuntimeRunsc, constants.ContainerRuntimeRunscExtension)

	booting, err := machineBooting(ctx, st)
	if err != nil {
		return nil, err
	}

	if booting {
		return []string{missing}, nil
	}

	return nil, errors.New(missing)
}

// machineBooting reports whether the machine may not have listed its extensions yet.
func machineBooting(ctx context.Context, st state.State) (bool, error) {
	machineStatus, err := safe.StateGetByID[*runtime.MachineStatus](ctx, st, runtime.MachineStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("failed to get machine status: %w", err)
	}

	stage := machineStatus.TypedSpec().Stage

	return stage == runtime.MachineStageUnknown || stage == runtime.MachineStageBooting, nil
}
