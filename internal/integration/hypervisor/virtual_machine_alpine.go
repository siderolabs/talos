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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/provision"
)

const (
	alpineISOName     = "alpine.iso"
	alpineOutputLimit = 512 << 10
)

// prepareAlpineGuest shares the stock-ISO provisioning used by console and history tests.
func (suite *LibvirtSuite) prepareAlpineGuest() (string, *hypervisorcfg.VirtualMachineConfigV1Alpha1, string, string) {
	suite.T().Helper()

	node, nodeCtx := suite.requireAlpineGuestNode()

	extraDHCPRecords := suite.Cluster.Info().Network.ExtraDHCPRecords
	if len(extraDHCPRecords) == 0 {
		suite.T().Skip("skipping as the cluster has no spare DHCP reservations")
	}

	ourRecord := extraDHCPRecords[len(extraDHCPRecords)-1]
	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, &ourRecord, 512<<20)

	return node, doc, ourRecord.Gateway.String(), ourRecord.IP.Addr().String()
}

// requireAlpineGuestNode checks stock Alpine support without requiring guest networking.
func (suite *LibvirtSuite) requireAlpineGuestNode() (string, context.Context) {
	suite.T().Helper()
	suite.requireContentLibrarySupport()

	if suite.HypervisorAlpineISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.alpine-iso is not set")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping as the cluster is not provisioned by qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	if arch := suite.ReadMachineArch(nodeCtx); arch != "amd64" {
		suite.T().Skipf("Alpine virt ISO is x86_64-only, node architecture is %q", arch)
	}

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	return node, nodeCtx
}

// prepareAlpineGuestOnNode provisions the stock ISO with the requested RAM and optional NIC.
// A nil DHCP record needs neither a reservation nor host macvtap support.
func (suite *LibvirtSuite) prepareAlpineGuestOnNode(node string, nodeCtx context.Context, ourRecord *provision.DHCPRecord, memoryBytes uint64) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	suite.T().Helper()

	original, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)
	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)
	// Registered before library and guest provisioning so restoration runs
	// last, with its own deadline, after their ordinary cleanup callbacks.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		_, applyErr := suite.Client.ApplyConfiguration(client.WithNode(ctx, node), &machine.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
		})
		suite.Assert().NoError(applyErr)
	})

	var interfaces []hypervisorcfg.VirtualMachineInterface

	if ourRecord != nil {
		hardwareAddr, err := net.ParseMAC(ourRecord.MAC)
		suite.Require().NoError(err)

		interfaces = []hypervisorcfg.VirtualMachineInterface{
			{
				InterfaceName:         "nic0",
				InterfaceLink:         "net0",
				HardwareAddressConfig: nethelpers.HardwareAddr(hardwareAddr),
			},
		}
	}

	iso, err := os.Open(suite.HypervisorAlpineISOPath)
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		suite.Assert().NoError(iso.Close())
	})

	info, err := iso.Stat()
	suite.Require().NoError(err)
	suite.Require().True(info.Mode().IsRegular(), "Alpine ISO must be a regular file")
	suite.Require().Positive(info.Size(), "Alpine ISO must not be empty")

	isoDigest, err := digest.FromReader(iso)
	suite.Require().NoError(err)

	_, err = iso.Seek(0, io.SeekStart)
	suite.Require().NoError(err)
	suite.T().Logf("uploading supplied stock Alpine ISO %s (%d bytes, %s)", suite.HypervisorAlpineISOPath, info.Size(), isoDigest)

	library, _ := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	started := time.Now()
	response, err := suite.Client.ContentLibraryUpload(
		nodeCtx,
		library,
		alpineISOName,
		false,
		isoDigest.String(),
		iso,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(alpineISOName, response.GetName())
	suite.Require().Equal(uint64(info.Size()), response.GetSize())
	suite.T().Logf("uploaded stock Alpine ISO (%s) in %s", isoDigest, time.Since(started).Round(time.Millisecond))

	name := "vm-alpine-" + uuid.NewString()
	for _, existing := range original.VirtualMachineConfigs() {
		suite.Require().NotEqual(name, existing.Name())
	}

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize(strconv.FormatUint(memoryBytes, 10))
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "alpine",
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: library,
					ImageFile:    alpineISOName,
					ImageDigest:  isoDigest.String(),
				},
			},
		},
	}
	doc.NetworkingConfig.InterfacesConfig = interfaces

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx := client.WithNode(ctx, node)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, hypervisorcfg.VirtualMachineConfigKind, name)
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](cleanupCtx, suite.T(), suite.Client.COSI, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	rtestutils.AssertResources(
		nodeCtx,
		suite.T(),
		suite.Client.COSI,
		[]string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
		},
	)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, len(interfaces))

	return doc
}

// TestAlpineSerialConsole boots the supplied stock Alpine virt ISO and
// exercises login, shell and networking exclusively through ConsoleStream.
func (suite *LibvirtSuite) TestAlpineSerialConsole() {
	node, doc, gateway, guestIP := suite.prepareAlpineGuest()
	name := doc.Name()
	nodeCtx := client.WithNode(suite.ctx, node)
	endpoint, networkResponse, requestSeen := suite.startAlpineHTTPServer(gateway, guestIP, nil)

	console := suite.loginAlpineConsole(nodeCtx, name)
	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
		}
	}()

	// Split markers on the wire: terminal echo cannot contain the expected
	// contiguous value, whether or not terminal echo is enabled.
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start := console.mark()
	suite.Require().NoError(console.send("printf '\\nALPINE_NONCE_%s\\n' '" + nonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_NONCE_"+nonce+"\r\n")))

	fileNonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start = console.mark()
	suite.Require().NoError(console.send("printf '%s\\n' console-file >/tmp/console-test && test \"$(cat /tmp/console-test)\" = console-file && printf '\\nALPINE_FILE_OK_%s\\n' '" + fileNonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_FILE_OK_"+fileNonce+"\r\n")))

	start = console.mark()
	suite.Require().NoError(console.send("ip link set eth0 up && udhcpc -i eth0 -q -n -t 5 && ip -4 addr show dev eth0\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("inet "+guestIP+"/")))

	pingNonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start = console.mark()
	suite.Require().NoError(console.send("ping -c 1 -W 2 " + gateway + " >/dev/null && printf '\\nALPINE_PING_OK_%s\\n' '" + pingNonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_PING_OK_"+pingNonce+"\r\n")))

	start = console.mark()
	suite.Require().NoError(console.send("wget -qO- '" + endpoint + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte(networkResponse)))

	select {
	case <-requestSeen:
	case <-time.After(5 * time.Second):
		suite.Require().FailNow("host HTTP server did not observe Alpine's wget request")
	}

	suite.detachAlpineConsole(console)
}

// TestAlpineGuestWorkloadMemoryLimitEnforced verifies that the kernel enforces the virtualMachines
// root limit on a running guest: once the guest touches more RAM than the root allows, the host memcg
// kills its QEMU. The cap is derived from the measured idle charge of the booted guest.
//
//nolint:gocyclo,cyclop
func (suite *LibvirtSuite) TestAlpineGuestWorkloadMemoryLimitEnforced() {
	// No interface: the proof is host-side, and the guest needs only its serial console.
	node, nodeCtx := suite.requireAlpineGuestNode()

	originalBytes := suite.RequireNoWorkloadResourceConfig(nodeCtx, node)

	suite.Require().Equal("max", suite.ReadFile(nodeCtx, filepath.Join(constants.CgroupMountPath, constants.CgroupVirtualMachines, "memory.max")))
	suite.RequireWorkloadRootIdle(nodeCtx, constants.CgroupVirtualMachines)

	// MiB-aligned so the kernel applies the cap exactly.
	const (
		alignment   = uint64(1 << 20)
		guestMemory = uint64(1 << 30)
		fillBytes   = uint64(512 << 20)
	)

	memTotal := suite.ReadMemTotal(nodeCtx)
	suite.Require().Less(guestMemory*2, memTotal, "node %s has %d bytes, too small to host the %d byte guest", node, memTotal, guestMemory)

	suite.RestoreMachineConfigOnCleanup(node, originalBytes, func(cleanupCtx context.Context) {
		suite.AssertCgroupFile(cleanupCtx, constants.CgroupVirtualMachines, "memory.max", "max")
	})

	// Boot uncapped and measure, so the cap is placed on observed numbers.
	guest := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, guestMemory)

	console := suite.loginAlpineConsole(nodeCtx, guest.Name())
	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
		}
	}()

	qemuPID, err := suite.alpineGuestQEMUPID(nodeCtx, guest.Name())
	suite.Require().NoError(err)
	suite.Require().NotZero(qemuPID, "no QEMU process for guest %q", guest.Name())

	cgroupPath := suite.ReadFile(nodeCtx, fmt.Sprintf("/proc/%d/cgroup", qemuPID))
	suite.Require().Contains(cgroupPath, "0::/"+constants.CgroupVirtualMachines+"/", "QEMU %d is not under the %s root: %s", qemuPID, constants.CgroupVirtualMachines, cgroupPath)

	idleCharge, err := suite.ReadCgroupUint(nodeCtx, constants.CgroupVirtualMachines, "memory.current")
	suite.Require().NoError(err)

	// Guest-side headroom is measured, not assumed: the write must fit in the guest's free RAM with
	// margin, so the guest's own OOM killer has no reason to act before the host cap does.
	freeNonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start := console.mark()
	suite.Require().NoError(console.send("printf '\\nALPINE_FREE_%s_%s_END\\n' '" + freeNonce + "' \"$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)\"\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("_END\r\n")))

	guestAvailable := parseAlpineMarker(suite.T(), console.transcript[start:], "\r\nALPINE_FREE_"+freeNonce+"_", "_END\r\n") * 1024
	suite.Require().Greater(guestAvailable, fillBytes+fillBytes/4, "guest has %d bytes available, not enough to write %d bytes without guest-side pressure", guestAvailable, fillBytes)

	// The cap sits halfway between the idle charge and the write, so the booted guest keeps running
	// and the write cannot complete under it.
	limit := (idleCharge + fillBytes/2) / alignment * alignment
	suite.Require().Greater(limit, idleCharge+64<<20, "idle charge %d leaves no room for a cap below the %d byte write", idleCharge, fillBytes)

	suite.T().Logf("guest %q QEMU PID %d in %s: idle charge %d, guest MemAvailable %d, cap %d, write %d",
		guest.Name(), qemuPID, strings.TrimSpace(cgroupPath), idleCharge, guestAvailable, limit, fillBytes)

	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.VirtualMachinesConfig = &runtimecfg.WorkloadResourceRoot{
		MemoryConfig: &runtimecfg.WorkloadMemoryResource{MemoryLimit: meta.MustByteSize(strconv.FormatUint(limit, 10))},
	}

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.AssertCgroupFile(nodeCtx, constants.CgroupVirtualMachines, "memory.max", strconv.FormatUint(limit, 10))

	// The guest is still alive under the cap before any pressure is applied.
	stillPID, err := suite.alpineGuestQEMUPID(nodeCtx, guest.Name())
	suite.Require().NoError(err)
	suite.Require().Equal(qemuPID, stillPID, "QEMU did not survive the cap being applied above its idle charge")

	unrelated := []string{constants.CgroupTalosContainersRoot, constants.CgroupSystem, constants.CgroupPodRuntimeRoot}
	before := suite.MemoryEventsSnapshot(nodeCtx, append([]string{constants.CgroupVirtualMachines}, unrelated...)...)
	started := time.Now()

	// Stock BusyBox: fill a tmpfs sized for the write. Every touched guest page is QEMU memory
	// charged under the root. The console is not drained from here on: the proof is the host-side
	// kill, not guest output.
	suite.Require().NoError(console.send(fmt.Sprintf(
		"mkdir -p /fill && mount -t tmpfs -o size=%dm tmpfs /fill && dd if=/dev/zero of=/fill/zero bs=1M count=%d\n",
		fillBytes>>20+1, fillBytes>>20,
	)))

	after := suite.AssertMemcgOOMKill(nodeCtx, constants.CgroupVirtualMachines, before[constants.CgroupVirtualMachines], 3*time.Minute)

	// The victim is the original QEMU: a successful process listing must no longer contain its PID.
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		pid, listErr := suite.alpineGuestQEMUPID(nodeCtx, guest.Name())
		if assert.NoError(collect, listErr) {
			assert.NotEqual(collect, qemuPID, pid, "original QEMU %d still alive", qemuPID)
		}
	}, time.Minute, time.Second)

	// Remove the VM before Talos restarts it into the same cap, then check the blast radius.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, guest.Name())
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](nodeCtx, suite.T(), suite.Client.COSI, guest.Name())

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		pid, listErr := suite.alpineGuestQEMUPID(nodeCtx, guest.Name())
		if assert.NoError(collect, listErr) {
			assert.Zero(collect, pid, "QEMU for removed guest %q still running", guest.Name())
		}
	}, time.Minute, time.Second)

	suite.AssertNoNewOOMKills(nodeCtx, before, unrelated...)
	suite.AssertNoUserspaceOOMSince(nodeCtx, started)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	suite.T().Logf("root %s OOM-killed guest %q %d time(s)", constants.CgroupVirtualMachines, guest.Name(),
		after.Hierarchical["oom_kill"]-before[constants.CgroupVirtualMachines].Hierarchical["oom_kill"])
}

// parseAlpineMarker extracts the integer between prefix and suffix in a console transcript.
func parseAlpineMarker(t *testing.T, transcript []byte, prefix, suffix string) uint64 {
	t.Helper()

	_, rest, found := bytes.Cut(transcript, []byte(prefix))
	require.True(t, found, "marker %q not found in console output %q", prefix, transcript)

	value, _, found := bytes.Cut(rest, []byte(suffix))
	require.True(t, found, "marker %q not terminated in console output %q", prefix, rest)

	parsed, err := strconv.ParseUint(string(bytes.TrimSpace(value)), 10, 64)
	require.NoError(t, err, "marker value %q", value)

	return parsed
}

// alpineGuestQEMUPID returns the PID of the QEMU process running the named guest, or zero when the
// process listing succeeded but contains none.
func (suite *LibvirtSuite) alpineGuestQEMUPID(nodeCtx context.Context, name string) (int32, error) {
	response, err := suite.Client.Processes(nodeCtx)
	if err != nil {
		return 0, err
	}

	for _, message := range response.Messages {
		for _, process := range message.Processes {
			if strings.Contains(process.Executable, "/qemu-system-") && strings.Contains(process.Args, "guest="+name+",") {
				return process.Pid, nil
			}
		}
	}

	return 0, nil
}

func (suite *LibvirtSuite) detachAlpineConsole(console *alpineConsole) {
	suite.T().Helper()

	defer console.cancel()

	suite.Require().NoError(console.stream.CloseSend())

	// Detach can leave queued stdout before EOF. Drain it under the existing
	// stream deadline and shared transcript limit, without accepting other errors.
	for {
		response, recvErr := console.stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}

		suite.Require().NoError(recvErr)

		output := response.GetStdoutData()
		suite.Require().LessOrEqual(len(console.transcript)+len(output), alpineOutputLimit, "console output exceeded limit while draining detach")
		console.transcript = append(console.transcript, output...)
	}
}

type alpineConsole struct {
	stream     machine.HypervisorService_ConsoleStreamClient
	cancel     context.CancelFunc
	transcript []byte
}

func (console *alpineConsole) mark() int {
	return len(console.transcript)
}

func (console *alpineConsole) send(data string) error {
	return console.stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_StdinData{
			StdinData: []byte(data),
		},
	})
}

func (console *alpineConsole) expectAfter(start int, expected []byte) error {
	for !bytes.Contains(console.transcript[start:], expected) {
		response, err := console.stream.Recv()
		if err != nil {
			return fmt.Errorf("receive console output while waiting for %q: %w", expected, err)
		}

		if len(console.transcript)+len(response.GetStdoutData()) > alpineOutputLimit {
			return fmt.Errorf("console output exceeded %d-byte limit while waiting for %q", alpineOutputLimit, expected)
		}

		console.transcript = append(console.transcript, response.GetStdoutData()...)
	}

	return nil
}

func (suite *LibvirtSuite) startAlpineHTTPServer(gateway, guestIP string, release <-chan struct{}) (string, string, <-chan struct{}) {
	suite.T().Helper()

	listener, err := (&net.ListenConfig{}).Listen(suite.ctx, "tcp4", net.JoinHostPort(gateway, "0"))
	suite.Require().NoError(err)

	response := "ALPINE_WGET_OK_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	requestSeen := make(chan struct{}, 1)
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    4096,
		Handler:           suite.alpineHTTPHandler(guestIP, response, requestSeen, release),
	}

	if release != nil {
		server.WriteTimeout = 45 * time.Second
	}

	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdownErr := server.Shutdown(ctx)
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, server.Close())
		}

		suite.Assert().NoError(shutdownErr)

		select {
		case serveErr := <-serveResult:
			suite.Require().ErrorIs(serveErr, http.ErrServerClosed)
		case <-ctx.Done():
			suite.Require().Fail("host HTTP server did not stop")
		}
	})

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	return "http://" + net.JoinHostPort(gateway, port) + "/alpine-console", response, requestSeen
}

func (suite *LibvirtSuite) alpineHTTPHandler(guestIP, response string, requestSeen chan<- struct{}, release <-chan struct{}) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		remoteIP, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil || remoteIP != guestIP || request.Method != http.MethodGet || request.URL.Path != "/alpine-console" {
			http.NotFound(writer, request)

			return
		}

		select {
		case requestSeen <- struct{}{}:
		default:
		}

		if !waitAlpineRelease(request.Context(), suite.ctx, release) {
			return
		}

		writer.Header().Set("Content-Type", "text/plain")

		if _, writeErr := io.WriteString(writer, response); writeErr != nil {
			suite.T().Logf("write Alpine HTTP response: %s", writeErr)
		}
	}
}

func waitAlpineRelease(requestCtx, suiteCtx context.Context, release <-chan struct{}) bool {
	// Ordinary networking tests have no gate and retain their previous behavior.
	if release == nil {
		return true
	}

	select {
	case <-release:
		return true
	case <-requestCtx.Done():
		return false
	case <-suiteCtx.Done():
		return false
	}
}
