// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/go-kmsg"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// KmsgLogStorageController presents kernel message log as a 'kernel' log.
type KmsgLogStorageController struct {
	V1Alpha1Logging runtime.LoggingManager
	V1Alpha1Mode    runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Name() string {
	return "runtime.KmsgLogStorageController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KmsgLogStorageController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	var err error

	logWriter, err := ctrl.V1Alpha1Logging.ServiceLog("kernel").Writer()
	if err != nil {
		return fmt.Errorf("error opening logger: %w", err)
	}
	defer logWriter.Close() //nolint:errcheck

	// initilalize kmsg reader early, so that we don't lose position on config changes
	reader, err := kmsg.NewReader(kmsg.Follow())
	if err != nil {
		return fmt.Errorf("error reading kernel messages: %w", err)
	}

	defer reader.Close() //nolint:errcheck

	kmsgCh := reader.Scan(ctx)

	// wait for the initial event to start processing messages
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	for {
		var msg kmsg.Packet

		select {
		case <-ctx.Done():
			return nil
		case msg = <-kmsgCh:
		}

		if msg.Err != nil {
			return fmt.Errorf("error receiving kernel logs: %w", msg.Err)
		}

		if _, err = logWriter.Write(
			fmt.Appendf(nil, "%s: %7s: [%s]: %s", msg.Message.Facility, msg.Message.Priority, msg.Message.Timestamp.Format(time.RFC3339Nano), msg.Message.Message),
		); err != nil {
			return err
		}
	}
}
