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
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
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

	ourRecord, ok := suite.LeaseDHCPRecord(suite.T())
	if !ok {
		suite.T().Skip("skipping as the cluster has no free DHCP reservations")
	}

	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, &ourRecord, 512<<20, "")

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

	if arch := suite.ReadMachineArch(nodeCtx); arch != constants.ArchAmd64 {
		suite.T().Skipf("Alpine virt ISO is x86_64-only, node architecture is %q", arch)
	}

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	return node, nodeCtx
}

// prepareAlpineGuestOnNode provisions the stock ISO with the requested RAM, profile and optional NIC.
// A nil DHCP record needs neither a reservation nor host macvtap support.
func (suite *LibvirtSuite) prepareAlpineGuestOnNode(
	node string,
	nodeCtx context.Context,
	ourRecord *provision.DHCPRecord,
	memoryBytes uint64,
	profile string,
) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
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
	pool, _ := suite.provisionContentLibraryPool(nodeCtx, node, library)

	started := time.Now()
	response, err := suite.Client.ContentLibraryUpload(
		nodeCtx,
		library,
		alpineISOName,
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

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](nodeCtx, suite.T(), suite.Client.State(), name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.State(), name)

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name

	if profile != "" {
		doc.ProfileConfig = &profile
	}

	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize(strconv.FormatUint(memoryBytes, 10))
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "alpine",
			DiskPool:      pool,
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
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](cleanupCtx, suite.T(), suite.Client.State(), name)
		suite.assertNoDomainWithContext(cleanupCtx, node, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	rtestutils.AssertResources(
		nodeCtx,
		suite.T(),
		suite.Client.State(),
		[]string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
		},
	)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, len(interfaces))

	return doc
}

// expectedAlpineClock is the clock a rendered domain carries; values are libvirt attribute values.
func expectedAlpineClock(rtcTickPolicy, pitTickPolicy, hpetPresent string) *libvirtxml.DomainClock {
	return &libvirtxml.DomainClock{
		Offset: "utc",
		Timer: []libvirtxml.DomainTimer{
			{Name: "rtc", TickPolicy: rtcTickPolicy},
			{Name: "pit", TickPolicy: pitTickPolicy},
			{Name: "hpet", Present: hpetPresent},
		},
	}
}

// defaultAlpineClock is the clock of a profile without timer settings. The HPET is spelled out
// rather than derived: hiding it by default is the behavior under test.
func defaultAlpineClock() *libvirtxml.DomainClock {
	return expectedAlpineClock(
		hypervisorhelpers.DefaultVirtualMachineProfileRTCTickPolicy.String(),
		hypervisorhelpers.DefaultVirtualMachineProfilePITTickPolicy.String(),
		"no",
	)
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
	stream, err := suite.Client.HypervisorService().ConsoleStream(streamCtx)
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

	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("Alpine login serial transcript (bounded): %q", console.transcript)
		}
	}()

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

// qemuPID reports the process the named virtual machine is running in, or zero when it is not.
func (suite *LibvirtSuite) qemuPID(ctx context.Context, name string) int32 {
	suite.T().Helper()

	response, err := suite.Client.Processes(ctx)
	if err != nil {
		if base.IgnoreGRPCUnavailable(err) == nil {
			return 0
		}

		suite.Require().NoError(err)
	}

	for _, message := range response.Messages {
		for _, process := range message.Processes {
			if strings.Contains(process.Executable, "/qemu-system-") && strings.Contains(process.Args, fmt.Sprintf("guest=%s,", name)) {
				return process.Pid
			}
		}
	}

	return 0
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
