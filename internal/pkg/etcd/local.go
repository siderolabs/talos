// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// GetLocalMemberID gets the etcd member id of the local node via resources.
func GetLocalMemberID(ctx context.Context, s state.State) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	member, err := safe.StateWatchFor[*etcd.Member](
		ctx,
		s,
		etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata(),
		state.WithEventTypes(state.Created),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get local etcd member ID: %w", err)
	}

	return etcd.ParseMemberID(member.TypedSpec().MemberID)
}
