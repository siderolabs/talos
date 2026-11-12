// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	// bootstrapLoggerGracePeriod is the time given to the machine to join the etcd cluster before the first reminder.
	//
	// The machine which joins an existing cluster does that in a matter of seconds, so there is no need for a reminder.
	bootstrapLoggerGracePeriod = 10 * time.Second
	// bootstrapLoggerInterval is the interval between the reminders.
	bootstrapLoggerInterval = 30 * time.Second
)

// BootstrapLoggerController reminds that the etcd cluster should be bootstrapped
// while the machine is waiting to join the etcd cluster.
type BootstrapLoggerController struct {
	// GracePeriod is the time to wait before the first reminder, defaults to 10 seconds.
	GracePeriod time.Duration
	// Interval is the interval between the reminders, defaults to 30 seconds.
	Interval time.Duration
}

// Name implements controller.Controller interface.
func (ctrl *BootstrapLoggerController) Name() string {
	return "etcd.BootstrapLoggerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BootstrapLoggerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.MembershipStatusType,
			ID:        optional.Some(etcd.MembershipStatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BootstrapLoggerController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *BootstrapLoggerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.GracePeriod == 0 {
		ctrl.GracePeriod = bootstrapLoggerGracePeriod
	}

	if ctrl.Interval == 0 {
		ctrl.Interval = bootstrapLoggerInterval
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	// the moment the next reminder is due, zero if the machine is not waiting to join the cluster
	var nextReminder time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		status, err := safe.ReaderGetByID[*etcd.MembershipStatus](ctx, r, etcd.MembershipStatusID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting etcd membership status: %w", err)
		}

		timer.Stop()

		select {
		case <-timer.C:
		default:
		}

		if status == nil || status.TypedSpec().Phase != etcd.MembershipPhaseJoining {
			nextReminder = time.Time{}

			r.ResetRestartBackoff()

			continue
		}

		if nextReminder.IsZero() {
			nextReminder = time.Now().Add(ctrl.GracePeriod)
		}

		if !time.Now().Before(nextReminder) {
			var ips []string

			// the addresses are only used for the message, so they are optional
			addresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting node addresses: %w", err)
			}

			if addresses != nil {
				for _, ip := range addresses.TypedSpec().IPs() {
					ips = append(ips, ip.String())
				}
			}

			logger.Info(
				"etcd is waiting to join the cluster, if this node is the first node in the cluster, please run `talosctl bootstrap` against one of the following IPs",
				zap.Strings("ips", ips),
			)

			nextReminder = time.Now().Add(ctrl.Interval)
		}

		timer.Reset(time.Until(nextReminder))

		r.ResetRestartBackoff()
	}
}
