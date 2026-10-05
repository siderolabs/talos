// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// TestAlpineSerialHistory exercises retained guest output through the real Logs RPC.
func (suite *LibvirtSuite) TestAlpineSerialHistory() {
	node, doc, gateway, guestIP := suite.prepareAlpineGuest()
	ctx := client.WithNode(suite.ctx, node)
	console := suite.loginAlpineConsole(ctx, doc.Name())
	start := console.mark()
	suite.Require().NoError(console.send("ip link set eth0 up && udhcpc -i eth0 -q -n -t 5 && ip -4 addr show dev eth0\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("inet "+guestIP+"/")))

	// The response is unknown to the guest and cannot occur in terminal echo.
	// Acknowledging the command and observing wget blocked on the server precede
	// detach. Only after detach reaches EOF can the server release guest output.
	release := make(chan struct{})
	endpoint, detachedMarker, requested := suite.startAlpineHTTPServer(gateway, guestIP, release)
	ready := serialNonce("READY")
	start = console.mark()
	suite.Require().NoError(console.send("(wget -T 40 -qO- '" + endpoint + "' >/dev/ttyS0) & " + serialPrintf(ready)))
	suite.Require().NoError(console.expectAfter(start, []byte(ready)))
	suite.awaitSerialSignal(requested, "guest wget readiness")
	suite.detachAlpineConsole(console)
	close(release)

	suite.Require().Eventually(func() bool {
		return bytes.Contains(suite.vmLogSnapshot(ctx, doc.Name(), -1), []byte(detachedMarker))
	}, 20*time.Second, 250*time.Millisecond, "guest output emitted after detach was not captured")

	// Keep a follower open while attaching and exchanging interactive data.
	// This proves history readers do not own the exclusive interactive console.
	probe := serialNonce("FOLLOW_READY")
	fresh := serialNonce("FOLLOW_FRESH")
	follower := suite.followVMLogs(ctx, doc.Name(), []string{probe, fresh, detachedMarker})
	console = suite.attachAlpineConsole(ctx, doc.Name())
	suite.synchronizeVMFollower(console, probe, follower.seen[0])

	select {
	case <-follower.seen[2]:
		suite.Require().FailNow("tail=0 replayed existing history")
	default:
	}

	start = console.mark()
	suite.Require().NoError(console.send(serialPrintf(fresh)))
	suite.Require().NoError(console.expectAfter(start, []byte(fresh)))
	suite.awaitSerialSignal(follower.seen[1], "fresh guest output through tail=0 follow")

	// Produce two exact final lines, with no prompt or echo after them. The
	// foreground shell is stopped before it can print the next prompt.
	first := serialNonce("TAIL_FIRST")
	last := serialNonce("TAIL_LAST")
	start = console.mark()
	suite.Require().NoError(console.send("stty -echo; printf '\\n%s\\n%s\\n' '" + first + "' '" + last + "'; kill -STOP $$\n"))
	suite.Require().NoError(console.expectAfter(start, []byte(last+"\r\n")))
	suite.Require().Eventually(func() bool {
		return bytes.Equal(suite.vmLogSnapshot(ctx, doc.Name(), 2), []byte(first+"\r\n"+last+"\r\n"))
	}, 10*time.Second, 100*time.Millisecond, "initial tail must contain exactly the two final guest lines")

	cliSuite := &base.CLISuite{
		TalosSuite: suite.TalosSuite,
	}
	cliSuite.SetT(suite.T())
	cliSuite.RunCLI([]string{"logs", "--kind", "vm", "--nodes", node, "--tail", "2", doc.Name()},
		base.StdoutMatchFunc(func(stdout string) error {
			// CLI text is terminal-safe; the RPC assertion above checks raw CRLF.
			if stdout != safeout.String(first+"\r\n"+last+"\r\n") {
				return fmt.Errorf("CLI VM tail did not return the two guest markers")
			}

			return nil
		}))

	follower.cancel()

	select {
	case err := <-follower.result:
		suite.Require().Equal(codes.Canceled, client.StatusCode(err), "follow cancellation returned %v", err)
	case <-time.After(5 * time.Second):
		suite.Require().FailNow("follow did not end promptly after cancellation")
	}

	suite.detachAlpineConsole(console)

	// Change desired power state, not the guest's reboot command. The existing
	// assertions verify the domain actually disappears before a new one starts.
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(ctx, doc)
	suite.assertStoppedDomain(ctx, node, doc.Name())
	suite.Require().Contains(string(suite.vmLogSnapshot(ctx, doc.Name(), -1)), detachedMarker)
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.PatchMachineConfig(ctx, doc)
	suite.assertRunningTransientDomainWithDevices(node, doc.Name(), 1, 1, 1)
	console = suite.loginAlpineConsole(ctx, doc.Name())
	restarted := serialNonce("RESTARTED")
	start = console.mark()
	suite.Require().NoError(console.send(serialPrintf(restarted)))
	suite.Require().NoError(console.expectAfter(start, []byte(restarted)))
	suite.Require().Eventually(func() bool {
		body := suite.vmLogSnapshot(ctx, doc.Name(), -1)

		return bytes.Contains(body, []byte(detachedMarker)) && bytes.Contains(body, []byte(restarted))
	}, 10*time.Second, 100*time.Millisecond, "full stop/start must append rather than truncate serial history")
	suite.detachAlpineConsole(console)
}

// TestAlpineSerialHistoryRotation forces actual virtlogd rollover with guest output.
func (suite *LibvirtSuite) TestAlpineSerialHistoryRotation() {
	if testing.Short() {
		suite.T().Skip("skipping bounded 12 MiB virtlogd rotation workload in short mode")
	}

	node, doc, gateway, guestIP := suite.prepareAlpineGuest()
	ctx := client.WithNode(suite.ctx, node)
	console := suite.loginAlpineConsole(ctx, doc.Name())
	start := console.mark()
	suite.Require().NoError(console.send("ip link set eth0 up && udhcpc -i eth0 -q -n -t 5 && ip -4 addr show dev eth0\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("inet "+guestIP+"/")))

	var domain libvirtxml.Domain

	suite.Require().NoError(domain.Unmarshal(suite.runVirsh(node, "dumpxml", doc.Name())))
	suite.Require().NotNil(domain.Devices)
	suite.Require().NotEmpty(domain.Devices.Serials)
	suite.Require().NotNil(domain.Devices.Serials[0].Log)
	path := domain.Devices.Serials[0].Log.File
	suite.Require().Regexp(`^/var/log/vm-[0-9a-f-]+-serial0\.log$`, path)

	old := serialNonce("BEFORE_ROTATION")
	start = console.mark()
	suite.Require().NoError(console.send(serialPrintf(old)))
	suite.Require().NoError(console.expectAfter(start, []byte(old)))
	suite.Require().Eventually(func() bool {
		return bytes.Contains(suite.vmLogSnapshot(ctx, doc.Name(), -1), []byte(old))
	}, 10*time.Second, 100*time.Millisecond)

	probe := serialNonce("ROTATION_FOLLOW_READY")
	fresh := serialNonce("AFTER_ROTATION")
	follower := suite.followVMLogs(ctx, doc.Name(), []string{probe, fresh})
	suite.synchronizeVMFollower(console, probe, follower.seen[0])

	// Twelve MiB exceeds the pinned virtlogd default of 2 MiB and three
	// backups. Newline-delimited 1 KiB records keep both tail scans and guest
	// memory bounded. Do not change daemon settings or rename files ourselves.
	release := make(chan struct{})
	endpoint, _, requested := suite.startAlpineHTTPServer(gateway, guestIP, release)
	ready := serialNonce("ROTATION_COMMAND_READY")
	// Re-emit old after 9 MiB so it remains in a backup at the end. Its
	// absence from the initial RPC read then proves active-file-only semantics,
	// rather than merely proving that retention discarded the first marker.
	flood := "awk 'BEGIN { for (i=0;i<9216;i++) printf \"%01023d\\n\", 0 }'; " +
		strings.TrimSuffix(serialPrintf(old), "\n") + "; " +
		"awk 'BEGIN { for (i=0;i<3072;i++) printf \"%01023d\\n\", 0 }'; " +
		strings.TrimSuffix(serialPrintf(fresh), "\n")
	start = console.mark()
	suite.Require().NoError(console.send("stty -echo; (wget -T 40 -qO- '" + endpoint + "' >/dev/null && { " + flood + "; }) >/dev/ttyS0 & " + serialPrintf(ready)))
	suite.Require().NoError(console.expectAfter(start, []byte(ready)))
	suite.awaitSerialSignal(requested, "rotation workload readiness")
	suite.detachAlpineConsole(console)
	close(release)

	select {
	case <-follower.seen[1]:
	case <-time.After(10 * time.Minute):
		suite.Require().FailNow("guest rotation workload did not finish within ten minutes")
	}

	busybox := "/nix/var/nix/profiles/default/bin/busybox"
	// Verify the actual capture files, not an inferred count from RPC bytes.
	// Unexpected overrides of the pinned defaults fail visibly, never skip.
	check := `set -eu
p="$1"; b="$2"; count=0
for f in "$p" "$p".*; do
  test -f "$f" || continue
  size=$("$b" stat -c %s "$f")
  test "$size" -le 2097152 || exit 2
  count=$((count + 1))
done
test "$count" -eq 4 || exit 3
test -f "$p.0" && test -f "$p.1" && test -f "$p.2" || exit 4
"$b" grep -F -q "$3" "$p.0" "$p.1" "$p.2" || exit 5
printf 'files=%s\n' "$count"
`

	suite.Require().Eventually(func() bool {
		_, code := suite.RunDebugContainer(ctx, node, busybox, "sh", "-c", check, "check-retention", path, busybox, old)

		return code == 0
	}, 20*time.Second, 250*time.Millisecond, "virtlogd must retain one active file and three bounded backups")
	body := suite.vmLogSnapshot(ctx, doc.Name(), -1)
	suite.Require().Contains(string(body), fresh)
	suite.Require().NotContains(string(body), old, "initial history read must not concatenate rotated backups")
}

func serialNonce(label string) string {
	return "SERIAL_" + label + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func serialPrintf(marker string) string {
	// Split the expected marker on the wire even when terminal echo is on.
	return "printf '\\nSERIAL_%s\\n' '" + strings.TrimPrefix(marker, "SERIAL_") + "'\n"
}

func (suite *LibvirtSuite) attachAlpineConsole(ctx context.Context, name string) *alpineConsole {
	suite.T().Helper()

	streamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	stream, err := suite.Client.HypervisorClient.ConsoleStream(streamCtx)
	suite.Require().NoError(err)
	suite.T().Cleanup(cancel)
	suite.Require().NoError(stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{
				Name: name,
			},
		},
	}))

	return &alpineConsole{
		stream: stream,
		cancel: cancel,
	}
}

func (suite *LibvirtSuite) loginAlpineConsole(ctx context.Context, name string) *alpineConsole {
	suite.T().Helper()
	console := suite.attachAlpineConsole(ctx, name)
	suite.Require().NoError(console.send("\n"))
	suite.Require().NoError(console.expectAfter(0, []byte("login:")))
	start := console.mark()
	suite.Require().NoError(console.send("root\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("localhost:~#")))

	return console
}

func (suite *LibvirtSuite) vmLogSnapshot(ctx context.Context, name string, tail int32) []byte {
	suite.T().Helper()

	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream, err := suite.Client.LogsWithKind(readCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, name, false, tail, machine.LogKind_LOG_KIND_VM)
	suite.Require().NoError(err)
	reader, err := client.ReadStream(stream)

	suite.Require().NoError(err)
	defer func() { suite.Assert().NoError(reader.Close()) }()

	body, err := io.ReadAll(io.LimitReader(reader, (3<<20)+1))
	suite.Require().NoError(err)
	suite.Require().LessOrEqual(len(body), 3<<20, "active VM log snapshot exceeded pinned virtlogd bound")

	return body
}

type vmLogFollower struct {
	seen   []chan struct{}
	done   chan struct{}
	result chan error
	cancel context.CancelFunc
}

func (suite *LibvirtSuite) followVMLogs(ctx context.Context, name string, markers []string) *vmLogFollower {
	suite.T().Helper()

	followCtx, cancel := context.WithTimeout(ctx, 11*time.Minute)
	stream, err := suite.Client.LogsWithKind(followCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, name, true, 0, machine.LogKind_LOG_KIND_VM)
	suite.Require().NoError(err)

	follower := &vmLogFollower{
		seen:   make([]chan struct{}, len(markers)),
		done:   make(chan struct{}),
		result: make(chan error, 1),
		cancel: cancel,
	}
	for i := range markers {
		follower.seen[i] = make(chan struct{})
	}

	go func() {
		defer close(follower.done)

		reader, readErr := client.ReadStream(stream)
		if readErr != nil {
			follower.result <- readErr

			return
		}

		readErr = scanVMLogMarkers(reader, markers, follower.seen)
		follower.result <- errors.Join(readErr, reader.Close())
	}()

	suite.T().Cleanup(func() {
		cancel()

		select {
		case <-follower.done:
		case <-time.After(5 * time.Second):
			suite.Assert().Fail("VM log receiver did not join after cancellation")
		}
	})

	return follower
}

func scanVMLogMarkers(reader io.Reader, markers []string, seen []chan struct{}) error {
	buf := make([]byte, 32<<10)
	window := make([]byte, 0, 40<<10)
	matched := make([]bool, len(markers))
	total := 0

	for {
		n, err := reader.Read(buf)

		total += n
		if total > 16<<20 {
			return fmt.Errorf("VM log follow exceeded 16 MiB workload limit")
		}

		window = append(window, buf[:n]...)
		for i, marker := range markers {
			if !matched[i] && bytes.Contains(window, []byte(marker)) {
				matched[i] = true
				close(seen[i])
			}
		}

		if len(window) > 8192 {
			copy(window, window[len(window)-8192:])
			window = window[:8192]
		}

		if err != nil {
			return err
		}
	}
}

func (suite *LibvirtSuite) synchronizeVMFollower(console *alpineConsole, probe string, seen <-chan struct{}) {
	suite.T().Helper()

	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()

	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		suite.Require().NoError(console.send(serialPrintf(probe)))

		select {
		case <-seen:
			return
		case <-deadline.C:
			suite.Require().FailNow("VM follower did not observe guest readiness marker")
		case <-tick.C:
		}
	}
}

func (suite *LibvirtSuite) awaitSerialSignal(seen <-chan struct{}, operation string) {
	suite.T().Helper()

	select {
	case <-seen:
	case <-time.After(45 * time.Second):
		suite.Require().FailNow("serial history deadline", operation)
	}
}
