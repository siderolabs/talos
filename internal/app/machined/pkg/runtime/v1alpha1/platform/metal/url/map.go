// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url

import (
	"context"
	"fmt"
	"log"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
)

// MapValues maps variable names to values.
//
//nolint:gocyclo
func MapValues(ctx context.Context, st state.State, variableNames []string) (map[string]string, error) {
	// happy case
	if len(variableNames) == 0 {
		return nil, nil
	}

	availableVariables := AllVariables()
	activeVariables := make(map[string]*Variable, len(variableNames))

	for _, variableName := range variableNames {
		if v, ok := availableVariables[variableName]; ok {
			activeVariables[variableName] = v
		} else {
			return nil, fmt.Errorf("unsupported variable name: %q", variableName)
		}
	}

	// setup watches
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	for _, variable := range activeVariables {
		if err := variable.Value.RegisterWatch(ctx, st, watchCh); err != nil {
			return nil, fmt.Errorf("error watching variable %q: %w", variable.Key, err)
		}
	}

	pendingVariables := xslices.ToSet(maps.Values(activeVariables))

	// wait for all variables to be populated
waitLoop:
	for len(pendingVariables) > 0 {
		log.Printf("waiting for variables: %v", xslices.Map(maps.Keys(pendingVariables), func(v *Variable) string { return v.Key }))

		var ev state.Event

		select {
		case <-ctx.Done():
			// context was canceled, return what we have
			break waitLoop
		case ev = <-watchCh:
		}

		switch ev.Type {
		case state.Errored:
			return nil, fmt.Errorf("error watching variables: %w", ev.Error)
		case state.Bootstrapped:
			// ignored
		case state.Created, state.Updated, state.Destroyed:
			for _, variable := range activeVariables {
				handled, err := variable.Value.EventHandler(ev)
				if err != nil {
					return nil, fmt.Errorf("error handling variable %q: %w", variable.Key, err)
				}

				if handled {
					delete(pendingVariables, variable)
				}
			}
		}
	}

	return maps.Map(activeVariables, func(k string, v *Variable) (string, string) { return k, v.Value.Get() }), nil
}
