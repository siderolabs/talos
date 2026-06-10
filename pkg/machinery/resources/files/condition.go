// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// EtcFileCondition waits for the given controller-managed /etc files to be ready.
type EtcFileCondition struct {
	state state.State
	ids   []resource.ID
}

// NewEtcFileCondition builds a condition that waits until each named /etc file has been written
// by its controller (its EtcFileStatus exists).
//
// Services that bind or read controller-managed /etc files must gate on this. /etc is a
// read-only overlay over a managed tmpfs; an early lookup of a not-yet-written path caches a
// negative dentry that the later out-of-band write cannot clear.
func NewEtcFileCondition(st state.State, ids ...resource.ID) *EtcFileCondition {
	return &EtcFileCondition{state: st, ids: ids}
}

// String implements conditions.Condition.
func (c *EtcFileCondition) String() string {
	return fmt.Sprintf("/etc files %q to be ready", c.ids)
}

// Wait implements conditions.Condition.
func (c *EtcFileCondition) Wait(ctx context.Context) error {
	for _, id := range c.ids {
		if _, err := c.state.WatchFor(
			ctx,
			resource.NewMetadata(NamespaceName, EtcFileStatusType, id, resource.VersionUndefined),
			state.WithCondition(func(r resource.Resource) (bool, error) {
				return !resource.IsTombstone(r), nil
			}),
		); err != nil {
			return err
		}
	}

	return nil
}
