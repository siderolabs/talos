// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package helpers defines encryption handlers.
package helpers

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// SystemInformationGetter defines the closure which can be used in key handlers to get the node UUID.
type SystemInformationGetter func(context.Context) (*hardware.SystemInformation, error)

// TPMLockFunc is a function that ensures that the TPM is locked and PCR state is as expected.
type TPMLockFunc func(context.Context, func() error) error
