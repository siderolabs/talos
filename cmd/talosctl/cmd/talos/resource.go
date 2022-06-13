// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"google.golang.org/grpc/codes"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

func getResourcesOfType[T typed.DeepCopyable[T], RD typed.ResourceDefinition[T]](
	namespace resource.Namespace,
	resourceType resource.Type,
	resources *[]typed.Resource[T, RD],
) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		listClient, err := c.Resources.List(ctx, namespace, resourceType)
		if err != nil {
			return err
		}

		for {
			msg, err := listClient.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) || client.StatusCode(err) == codes.Canceled {
					return nil
				}

				return err
			}

			if msg.Metadata.GetError() != "" {
				return fmt.Errorf("%s: %s", msg.Metadata.GetHostname(), msg.Metadata.GetError())
			}

			if msg.Resource == nil {
				continue
			}

			b, err := yaml.Marshal(msg.Resource.Spec())
			if err != nil {
				return err
			}

			var spec T

			if err = yaml.Unmarshal(b, &spec); err != nil {
				return err
			}

			res := typed.NewResource[T, RD](*msg.Resource.Metadata(), spec)
			*resources = append(*resources, *res)
		}
	}
}
