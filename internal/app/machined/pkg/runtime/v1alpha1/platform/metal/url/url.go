// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package url handles expansion of the download URL for the config.
package url

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
)

// Populate populates the config download URL with values replacing variables.
func Populate(ctx context.Context, downloadURL string, st state.State) (string, error) {
	return PopulateVariables(ctx, downloadURL, st, maps.Values(AllVariables()))
}

// PopulateVariables populates the config download URL with values replacing variables.
//
//nolint:gocyclo
func PopulateVariables(ctx context.Context, downloadURL string, st state.State, variables []*Variable) (string, error) {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	query := u.Query()

	var activeVariables []*Variable

	for _, variable := range variables {
		if variable.Matches(query) {
			activeVariables = append(activeVariables, variable)
		}
	}

	// happy path: no variables
	if len(activeVariables) == 0 {
		return downloadURL, nil
	}

	// setup watches
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	for _, variable := range activeVariables {
		if err = variable.Value.RegisterWatch(ctx, st, watchCh); err != nil {
			return "", fmt.Errorf("error watching variable %q: %w", variable.Key, err)
		}
	}

	pendingVariables := xslices.ToSet(activeVariables)

	// wait for all variables to be populated
	for len(pendingVariables) > 0 {
		log.Printf("waiting for URL variables: %v", xslices.Map(maps.Keys(pendingVariables), func(v *Variable) string { return v.Key }))

		var ev state.Event

		select {
		case <-ctx.Done():
			// context was canceled, return the URL as is
			u.RawQuery = query.Encode()

			return u.String(), ctx.Err()
		case ev = <-watchCh:
		}

		switch ev.Type {
		case state.Errored:
			return "", fmt.Errorf("error watching variables: %w", ev.Error)
		case state.Bootstrapped:
			// ignored
		case state.Created, state.Updated, state.Destroyed:
			anyHandled := false

			for _, variable := range activeVariables {
				handled, err := variable.Value.EventHandler(ev)
				if err != nil {
					return "", fmt.Errorf("error handling variable %q: %w", variable.Key, err)
				}

				if handled {
					delete(pendingVariables, variable)

					anyHandled = true
				}
			}

			if !anyHandled {
				continue
			}

			// perform another round of replacing
			query = u.Query()

			for _, variable := range activeVariables {
				if _, pending := pendingVariables[variable]; pending {
					continue
				}

				variable.Replace(query)
			}
		}
	}

	u.RawQuery = query.Encode()

	return u.String(), nil
}
