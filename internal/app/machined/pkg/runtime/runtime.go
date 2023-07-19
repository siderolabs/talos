// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// Runtime defines the runtime parameters.
type Runtime interface { //nolint:interfacebloat
	Config() config.Config
	ConfigContainer() config.Container
	RollbackToConfigAfter(time.Duration) error
	CancelConfigRollbackTimeout()
	SetConfig(config.Provider) error
	CanApplyImmediate(config.Provider) error
	State() State
	Events() EventStream
	Logging() LoggingManager
	NodeName() (string, error)
	IsBootstrapAllowed() bool
	GetSystemInformation(ctx context.Context) (*hardware.SystemInformation, error)
}
