// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package cli_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/cli"
)

func TestClaimSignals(t *testing.T) {
	_, err := cli.WithContextC(t.Context(), func(ctx context.Context) (*cobra.Command, error) {
		sigC := make(chan os.Signal, 2)

		release := cli.ClaimSignals(ctx, sigC, syscall.SIGHUP)

		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))

		var received []os.Signal

		for range 2 {
			select {
			case sig := <-sigC:
				received = append(received, sig)
			case <-time.After(10 * time.Second):
				t.Fatalf("signals were not delivered, got %v", received)
			}
		}

		assert.ElementsMatch(t, []os.Signal{os.Interrupt, syscall.SIGHUP}, received)

		// A claimed signal does not cancel the command.
		require.NoError(t, ctx.Err())

		release()

		// Once released, SIGINT cancels the command again.
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))

		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
			t.Fatal("SIGINT did not cancel the command after the signals were released")
		}

		return nil, nil
	})
	require.NoError(t, err)
}
