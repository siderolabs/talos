// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// ReadyCondition implements condition which waits for the network to be ready.
type ReadyCondition struct {
	state  state.State
	checks []StatusCheck
}

// NewReadyCondition builds a condition which waits for the network to be ready.
func NewReadyCondition(state state.State, checks ...StatusCheck) *ReadyCondition {
	return &ReadyCondition{
		state:  state,
		checks: checks,
	}
}

func (condition *ReadyCondition) String() string {
	return "network"
}

// Wait implements condition interface.
func (condition *ReadyCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(NamespaceName, StatusType, StatusID, resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			status := r.(*Status).TypedSpec()

			for _, check := range condition.checks {
				if !check(status) {
					return false, nil
				}
			}

			return true, nil
		}),
	)

	return err
}

// StatusCheck asserts specific part of Status to be true.
type StatusCheck func(*StatusSpec) bool

// AddressReady checks if address is ready.
func AddressReady(spec *StatusSpec) bool {
	return spec.AddressReady
}

// ConnectivityReady checks if connectivity is ready.
func ConnectivityReady(spec *StatusSpec) bool {
	return spec.ConnectivityReady
}

// HostnameReady checks if hostname is ready.
func HostnameReady(spec *StatusSpec) bool {
	return spec.HostnameReady
}

// EtcFilesReady checks if etc files are ready.
func EtcFilesReady(spec *StatusSpec) bool {
	return spec.EtcFilesReady
}

// StatusChecksFromStatuses converts nethelpers.Status list into list of checks.
func StatusChecksFromStatuses(statuses ...nethelpers.Status) []StatusCheck {
	checks := make([]StatusCheck, 0, len(statuses))

	for _, st := range statuses {
		switch st {
		case nethelpers.StatusAddresses:
			checks = append(checks, AddressReady)
		case nethelpers.StatusConnectivity:
			checks = append(checks, ConnectivityReady)
		case nethelpers.StatusHostname:
			checks = append(checks, HostnameReady)
		case nethelpers.StatusEtcFiles:
			checks = append(checks, EtcFilesReady)
		}
	}

	return checks
}
