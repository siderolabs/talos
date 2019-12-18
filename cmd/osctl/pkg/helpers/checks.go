// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
)

// FailIfMultiNodes checks if ctx contains multi-node request metadata.
func FailIfMultiNodes(ctx context.Context, command string) error {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return nil
	}

	if len(md.Get("nodes")) <= 1 {
		return nil
	}

	return fmt.Errorf("command %q is not supported with multiple nodes", command)
}
