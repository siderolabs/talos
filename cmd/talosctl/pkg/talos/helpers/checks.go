// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// ClientFactory is the interface for the client factory.
type ClientFactory interface {
	BuildClient(ctx context.Context, node string) (context.Context, *client.Client, error)
	Nodes() []string
}

// GetTalosVersionBounds returns the lowest and the highest Talos version running across all nodes.
func GetTalosVersionBounds(ctx context.Context, clientFactory ClientFactory) (lowest, highest semver.Version, err error) {
	found := false

	if err = forEachServerVersion(ctx, clientFactory, func(_ string, serverVersion semver.Version) {
		if !found || serverVersion.LT(lowest) {
			lowest = serverVersion
		}

		if !found || serverVersion.GT(highest) {
			highest = serverVersion
		}

		found = true
	}); err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	if !found {
		return semver.Version{}, semver.Version{}, errors.New("no server versions found")
	}

	return lowest, highest, nil
}

// ClientVersionCheck verifies that client is not outdated vs. Talos version.
func ClientVersionCheck(ctx context.Context, clientFactory ClientFactory) error {
	clientVersion, err := semver.ParseTolerant(version.NewVersion().Tag)
	if err != nil {
		return fmt.Errorf("error parsing client version: %w", err)
	}

	var warnings []string

	errs := forEachServerVersion(ctx, clientFactory, func(node string, serverVersion semver.Version) {
		if serverVersion.Compare(clientVersion) < 0 {
			warnings = append(warnings, fmt.Sprintf("%s: server version %s is older than client version %s", node, serverVersion, clientVersion))
		}
	})

	if warnings != nil {
		fmt.Fprintf(safeout.Stderr(), "WARNING: %s\n", strings.Join(warnings, ", "))
	}

	return errs
}

// forEachServerVersion fetches the Talos version from all nodes and calls fn for each successfully parsed version.
//
// Errors fetching or parsing versions are collected and returned after all nodes are processed.
func forEachServerVersion(ctx context.Context, clientFactory ClientFactory, fn func(node string, serverVersion semver.Version)) error {
	respCh := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.VersionResponse, error) {
			return c.Version(ctx)
		},
	)

	var errs error

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: error getting server version: %w", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.GetMessages() {
			serverVersion, err := semver.ParseTolerant(msg.GetVersion().Tag)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("%s: error parsing server version: %w", resp.Node, err))

				continue
			}

			fn(resp.Node, serverVersion)
		}
	}

	return errs
}
