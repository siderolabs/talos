// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"fmt"
	"io"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

// ForEachResource get resources from the controller runtime and run callback using each element.
//nolint:gocyclo
func ForEachResource(ctx context.Context, c *client.Client, callback func(ctx context.Context, msg client.ResourceResponse) error, namespace string, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("not enough arguments: at least 1 is expected")
	}

	resourceType := args[0]

	var resourceID string

	if len(args) > 1 {
		resourceID = args[1]
	}

	if resourceID != "" {
		resp, err := c.Resources.Get(ctx, namespace, resourceType, resourceID)
		if err != nil {
			return err
		}

		for _, msg := range resp {
			if err = callback(ctx, msg); err != nil {
				return err
			}
		}
	} else {
		listClient, err := c.Resources.List(ctx, namespace, resourceType)
		if err != nil {
			return err
		}

		for {
			msg, err := listClient.Recv()
			if err != nil {
				if err == io.EOF || status.Code(err) == codes.Canceled {
					return nil
				}

				return err
			}

			if msg.Metadata.GetError() != "" {
				fmt.Fprintf(os.Stderr, "%s: %s\n", msg.Metadata.GetHostname(), msg.Metadata.GetError())

				continue
			}

			if err = callback(ctx, msg); err != nil {
				return err
			}
		}
	}

	return nil
}
