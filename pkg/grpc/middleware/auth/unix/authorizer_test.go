// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package unix_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/unix"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const (
	selfMountNamespace = "mnt:[4026531840]"
	selfExeDev         = uint64(42)
	selfExeIno         = uint64(1000)
)

func newUsermodeHelperAuthorizer(t *testing.T) *unix.Authorizer {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	return &unix.Authorizer{
		Resources:           st,
		SelfMountNamespace:  selfMountNamespace,
		SelfExeDev:          selfExeDev,
		SelfExeIno:          selfExeIno,
		UsermodeHelperRoles: role.MakeSet(role.Admin, role.Operator),
	}
}

func ctxWithCreds(parent context.Context, creds unix.PeerCredentials) context.Context {
	return peer.NewContext(parent, &peer.Peer{AuthInfo: creds})
}

func TestAuthorizerUsermodeHelper(t *testing.T) {
	t.Parallel()

	a := newUsermodeHelperAuthorizer(t)

	// the kernel usermode helper: this same binary (exe dev/ino), same mount namespace, UID 0.
	ctx := ctxWithCreds(context.Background(), unix.PeerCredentials{
		PID:            12345,
		UID:            0,
		MountNamespace: selfMountNamespace,
		ExeDev:         selfExeDev,
		ExeIno:         selfExeIno,
	})

	roles, err := a.Authorize(ctx)
	require.NoError(t, err)

	assert.True(t, roles.Includes(role.Admin))
	assert.True(t, roles.Includes(role.Operator))
}

func TestAuthorizerUsermodeHelperRejected(t *testing.T) {
	t.Parallel()

	for name, creds := range map[string]unix.PeerCredentials{
		"wrong exe inode": {
			PID: 12345, UID: 0, MountNamespace: selfMountNamespace, ExeDev: selfExeDev, ExeIno: selfExeIno + 1,
		},
		"wrong exe device": {
			PID: 12345, UID: 0, MountNamespace: selfMountNamespace, ExeDev: selfExeDev + 1, ExeIno: selfExeIno,
		},
		"wrong mount namespace": {
			PID: 12345, UID: 0, MountNamespace: "mnt:[4026531999]", ExeDev: selfExeDev, ExeIno: selfExeIno,
		},
		"empty mount namespace": {
			PID: 12345, UID: 0, MountNamespace: "", ExeDev: selfExeDev, ExeIno: selfExeIno,
		},
		"non-root uid": {
			PID: 12345, UID: 1000, MountNamespace: selfMountNamespace, ExeDev: selfExeDev, ExeIno: selfExeIno,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			a := newUsermodeHelperAuthorizer(t)

			// bound the fallback watch so the deny path returns promptly instead of waiting PIDTimeout.
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			_, err := a.Authorize(ctxWithCreds(ctx, creds))
			assert.ErrorIs(t, err, unix.ErrNotAuthorized)
		})
	}
}

func TestAuthorizerUsermodeHelperDisabled(t *testing.T) {
	t.Parallel()

	// without SelfExeIno configured (as in apid/trustd authorizers) the check is disabled,
	// even for otherwise-matching credentials.
	a := &unix.Authorizer{
		Resources:           state.WrapCore(namespaced.NewState(inmem.Build)),
		UsermodeHelperRoles: role.MakeSet(role.Admin, role.Operator),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := a.Authorize(ctxWithCreds(ctx, unix.PeerCredentials{
		PID:            12345,
		UID:            0,
		MountNamespace: selfMountNamespace,
		ExeDev:         selfExeDev,
		ExeIno:         selfExeIno,
	}))
	assert.ErrorIs(t, err, unix.ErrNotAuthorized)
}
