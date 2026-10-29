// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ContainerRestart configures how the container is restarted after it exits.
type ContainerRestart struct {
	//   description: |
	//     When the container is restarted after it exits.
	//
	//     `always` restarts it after every exit, `on-failure` after it failed (a non-zero exit code
	//     or a failure to start), and `never` runs it once.
	//     A container which is not restarted keeps its outcome until its configuration changes or it
	//     is removed, and runs again after a reboot or a restart of machined.
	//     `talosctl restart` only signals the container, so it doesn't run such a container again.
	//   values:
	//     - always
	//     - on-failure
	//     - never
	//   schema:
	//     type: string
	RestartPolicy config.ContainerRestartPolicy `yaml:"policy,omitempty"`
	//   description: |
	//     How many times a failed container is restarted at most, with the `on-failure` policy.
	//
	//     Unset restarts it until it succeeds.
	RestartMaxRetries *int `yaml:"maxRetries,omitempty"`
	//   description: |
	//     Time to wait after the container exited before it is restarted.
	//     Defaults to 5s.
	//   examples:
	//    - value: >
	//       5 * time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	RestartDelay *time.Duration `yaml:"delay,omitempty"`
	//   description: |
	//     Longest time to wait before a restart.
	//
	//     The delay doubles with each restart of a container which ran for a shorter time than this,
	//     up to this. Defaults to the delay, so the delay doesn't grow.
	//   examples:
	//    - value: >
	//       time.Minute
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	RestartMaxDelay *time.Duration `yaml:"maxDelay,omitempty"`
}

var _ config.ContainerRestartConfig = &ContainerRestart{}

// Policy implements config.ContainerRestartConfig interface.
func (r *ContainerRestart) Policy() config.ContainerRestartPolicy {
	if r.RestartPolicy == "" {
		return config.ContainerRestartPolicyAlways
	}

	return r.RestartPolicy
}

// MaxRetries implements config.ContainerRestartConfig interface.
func (r *ContainerRestart) MaxRetries() optional.Optional[int] {
	if r.RestartMaxRetries == nil {
		return optional.None[int]()
	}

	return optional.Some(*r.RestartMaxRetries)
}

// Delay implements config.ContainerRestartConfig interface.
func (r *ContainerRestart) Delay() time.Duration {
	if r.RestartDelay == nil {
		return constants.ContainerDefaultRestartDelay
	}

	return *r.RestartDelay
}

// MaxDelay implements config.ContainerRestartConfig interface.
func (r *ContainerRestart) MaxDelay() time.Duration {
	if r.RestartMaxDelay == nil {
		return r.Delay()
	}

	return *r.RestartMaxDelay
}

// Validate checks the restart settings.
func (r *ContainerRestart) Validate() error {
	var validationErrors error

	switch r.Policy() {
	case config.ContainerRestartPolicyAlways, config.ContainerRestartPolicyOnFailure, config.ContainerRestartPolicyNever:
	default:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("unsupported restart policy %q, expected always, on-failure or never", r.RestartPolicy))
	}

	if r.RestartMaxRetries != nil {
		if *r.RestartMaxRetries < 0 || *r.RestartMaxRetries > math.MaxUint32 {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("restart.maxRetries must be between 0 and %d, got %d", uint32(math.MaxUint32), *r.RestartMaxRetries))
		}

		if r.Policy() != config.ContainerRestartPolicyOnFailure {
			validationErrors = errors.Join(validationErrors, errors.New("restart.maxRetries is only supported with the on-failure policy"))
		}
	}

	return errors.Join(validationErrors, r.validateDelays())
}

func (r *ContainerRestart) validateDelays() error {
	var validationErrors error

	if r.RestartDelay != nil && *r.RestartDelay <= 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("restart.delay must be positive, got %s", *r.RestartDelay))
	}

	if r.RestartMaxDelay != nil && *r.RestartMaxDelay <= 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("restart.maxDelay must be positive, got %s", *r.RestartMaxDelay))
	}

	if r.RestartMaxDelay != nil && *r.RestartMaxDelay < r.Delay() {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("restart.maxDelay %s must not be shorter than restart.delay %s", *r.RestartMaxDelay, r.Delay()))
	}

	return validationErrors
}
