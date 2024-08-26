// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package networkutils provides utilities for controllers to interact with network resources.
package networkutils

import (
	"context"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/safchain/ethtool"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// WaitForNetworkReady waits for devices to be ready.
//
// It is a helper function for controllers.
func WaitForNetworkReady(ctx context.Context, r controller.Runtime, condition func(*network.StatusSpec) bool, nextInputs []controller.Input) error {
	// set inputs temporarily to a service only
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.EventCh():
		}

		status, err := safe.ReaderGetByID[*network.Status](ctx, r, network.StatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if condition(status.TypedSpec()) {
			// condition met
			break
		}
	}

	// restore inputs
	if err := r.UpdateInputs(nextInputs); err != nil {
		return err
	}

	// queue an update to reprocess with new inputs
	r.QueueReconcile()

	return nil
}

// FormatFeatures formats ethtool features.
func FormatFeatures(features map[string]ethtool.FeatureState) map[string]string {
	if features == nil {
		return nil
	}

	keys := maps.Keys(features)
	slices.Sort(keys)

	out := make(map[string]string, len(keys))

	for _, feature := range keys {
		state := features[feature]

		var value string

		if state.Active {
			value = "on"
		} else {
			value = "off"
		}

		if !state.Available || state.NeverChanged {
			value += " [fixed]"
		} else if state.Requested != state.Active {
			if state.Requested {
				value += " [requested on]"
			} else {
				value += " [requested off]"
			}
		}

		out[feature] = value
	}

	return out
}
