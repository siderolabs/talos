// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// executionHashCache keeps the execution hash of each ContainerSpec, so that it is computed once per
// version of the spec rather than on every pass.
//
// It is only used from the controller's own goroutine.
type executionHashCache map[resource.ID]executionHashEntry

type executionHashEntry struct {
	created time.Time
	version resource.Version
	hash    string
}

// get returns the execution hash of the spec.
func (c *executionHashCache) get(containerSpec *containers.ContainerSpec) string {
	md := containerSpec.Metadata()

	// a spec removed and created again starts over with its version, but not with its creation time
	if entry, ok := (*c)[md.ID()]; ok && entry.version.Equal(md.Version()) && entry.created.Equal(md.Created()) {
		return entry.hash
	}

	if *c == nil {
		*c = executionHashCache{}
	}

	hash := containerSpec.TypedSpec().ExecutionHash()

	(*c)[md.ID()] = executionHashEntry{created: md.Created(), version: md.Version(), hash: hash}

	return hash
}

// retain forgets the specs which are gone.
func (c executionHashCache) retain(ids map[string]struct{}) {
	for id := range c {
		if _, ok := ids[id]; !ok {
			delete(c, id)
		}
	}
}
