// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/pkg/provision"
)

// CreateDHCPd creates a DHCP server on darwin.
// It waits for the interface to appear, shut's down the apple bootp DHCPd server created by qemu by default,
// starts the talos DHCP server and then starts the apple bootp server again, which is configured such
// that it detects existing dhcp servers on interfaces and doesn't interfare with them.
func (p *Provisioner) CreateDHCPd(ctx context.Context, state *State, clusterReq provision.ClusterRequest) error {
	err := waitForInterface(ctx, state.BridgeName)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "/bin/launchctl", "unload", "-w", "/System/Library/LaunchDaemons/bootps.plist")

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stop native dhcp server: %w", err)
	}

	err = p.startDHCPd(state, clusterReq)
	if err != nil {
		return err
	}

	err = waitForDHCPServerUp(ctx, state)
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	cmd = exec.CommandContext(ctx, "/bin/launchctl", "load", "-w", "/System/Library/LaunchDaemons/bootps.plist")

	err = cmd.Run()
	if err != nil {
		fmt.Printf("warning: failed to start native dhcp server after creating a talos dhcp server: %s", err)
	}

	return nil
}

// waitForInterface returns when interface is found or errors after a minute.
func waitForInterface(ctx context.Context, interfaceName string) error {
	return retry.Constant(1*time.Minute, retry.WithUnits(50*time.Millisecond)).RetryWithContext(ctx, func(_ context.Context) error {
		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		for _, iface := range ifaces {
			if iface.Name == interfaceName {
				return nil
			}
		}

		return retry.ExpectedError(fmt.Errorf("interface %s not found", interfaceName))
	})
}

func waitForDHCPServerUp(ctx context.Context, state *State) error {
	return retry.Constant(1*time.Minute, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(_ context.Context) error {
		logFileData, err := os.ReadFile(state.GetRelativePath(dhcpLog))
		if err != nil {
			return retry.ExpectedError(err)
		}

		if strings.Contains(string(logFileData), "Ready to handle requests") {
			return nil
		}

		return retry.ExpectedError(fmt.Errorf("failure: DHCPd server has not started"))
	})
}
