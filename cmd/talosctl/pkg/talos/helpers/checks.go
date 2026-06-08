// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xerrors"

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

// VersionOutsideRangeError is returned when a node is running a Talos version that is outside the desired range.
type VersionOutsideRangeError struct{}

// TalosVersionCheck verifies that all nodes are running the desired Talos version.
func TalosVersionCheck(ctx context.Context, clientFactory ClientFactory, desired semver.Range) error {
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

			if !desired(serverVersion) {
				errs = errors.Join(errs, xerrors.NewTaggedf[VersionOutsideRangeError]("%s: server version %s is outside the desired range", resp.Node, serverVersion))
			}
		}
	}

	return errs
}

// ClientVersionCheck verifies that client is not outdated vs. Talos version.
func ClientVersionCheck(ctx context.Context, clientFactory ClientFactory) error {
	respCh := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.VersionResponse, error) {
			return c.Version(ctx)
		},
	)

	clientVersion, err := semver.ParseTolerant(version.NewVersion().Tag)
	if err != nil {
		return fmt.Errorf("error parsing client version: %w", err)
	}

	var (
		warnings []string
		errs     error
	)

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

			if serverVersion.Compare(clientVersion) < 0 {
				warnings = append(warnings, fmt.Sprintf("%s: server version %s is older than client version %s", resp.Node, serverVersion, clientVersion))
			}
		}
	}

	if warnings != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", strings.Join(warnings, ", "))
	}

	return errs
}
