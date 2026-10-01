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

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const (
	alpineISOName     = "alpine.iso"
	alpineOutputLimit = 512 << 10
)

// TestAlpineSerialConsole boots the supplied stock Alpine virt ISO and
// exercises login, shell and networking exclusively through ConsoleStream.
func (suite *LibvirtSuite) TestAlpineSerialConsole() {
	suite.requireContentLibrarySupport()

	if suite.HypervisorAlpineISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.alpine-iso is not set")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping as the cluster is not provisioned by qemu")
	}

	extraDHCPRecords := suite.Cluster.Info().Network.ExtraDHCPRecords
	if len(extraDHCPRecords) == 0 {
		suite.T().Skip("skipping as the cluster has no spare DHCP reservations")
	}

	ourRecord := extraDHCPRecords[len(extraDHCPRecords)-1]
	hardwareAddr, err := net.ParseMAC(ourRecord.MAC)
	suite.Require().NoError(err)

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	if arch := suite.ReadMachineArch(nodeCtx); arch != "amd64" {
		suite.T().Skipf("Alpine virt ISO is x86_64-only, node architecture is %q", arch)
	}

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

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
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")
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
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{
			InterfaceName:         "nic0",
			InterfaceLink:         "net0",
			HardwareAddressConfig: nethelpers.HardwareAddr(hardwareAddr),
		},
	}

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
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
		},
	)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 1)

	endpoint, networkResponse, requestSeen := suite.startAlpineHTTPServer(ourRecord.Gateway.String(), ourRecord.IP.Addr().String())

	streamCtx, cancelStream := context.WithTimeout(nodeCtx, 3*time.Minute)
	defer cancelStream()

	stream, err := suite.Client.HypervisorClient.ConsoleStream(streamCtx)
	suite.Require().NoError(err)
	suite.Require().NoError(stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{
				Name: name,
			},
		},
	}))

	console := &alpineConsole{
		stream: stream,
	}

	suite.T().Cleanup(func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
		}
	})

	bootStarted := time.Now()
	start := console.mark()
	suite.Require().NoError(console.send("\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("login:")))
	start = console.mark()
	suite.Require().NoError(console.send("root\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("localhost:~#")))
	suite.T().Logf("Alpine serial login became interactive after %s", time.Since(bootStarted).Round(time.Millisecond))

	// Split markers on the wire: terminal echo cannot contain the expected
	// contiguous value, whether or not terminal echo is enabled.
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start = console.mark()
	suite.Require().NoError(console.send("printf '\\nALPINE_NONCE_%s\\n' '" + nonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_NONCE_"+nonce+"\r\n")))

	fileNonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start = console.mark()
	suite.Require().NoError(console.send("printf '%s\\n' console-file >/tmp/console-test && test \"$(cat /tmp/console-test)\" = console-file && printf '\\nALPINE_FILE_OK_%s\\n' '" + fileNonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_FILE_OK_"+fileNonce+"\r\n")))

	guestIP := ourRecord.IP.Addr().String()
	start = console.mark()
	suite.Require().NoError(console.send("ip link set eth0 up && udhcpc -i eth0 -q -n -t 5 && ip -4 addr show dev eth0\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("inet "+guestIP+"/")))

	pingNonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start = console.mark()
	suite.Require().NoError(console.send("ping -c 1 -W 2 " + ourRecord.Gateway.String() + " >/dev/null && printf '\\nALPINE_PING_OK_%s\\n' '" + pingNonce + "'\n"))
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

func (suite *LibvirtSuite) detachAlpineConsole(console *alpineConsole) {
	suite.T().Helper()
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

func (suite *LibvirtSuite) startAlpineHTTPServer(gateway, guestIP string) (string, string, <-chan struct{}) {
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
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			remoteIP, _, remoteErr := net.SplitHostPort(request.RemoteAddr)
			if remoteErr != nil || remoteIP != guestIP || request.Method != http.MethodGet || request.URL.Path != "/alpine-console" {
				http.NotFound(writer, request)

				return
			}

			select {
			case requestSeen <- struct{}{}:
			default:
			}

			writer.Header().Set("Content-Type", "text/plain")

			if _, writeErr := io.WriteString(writer, response); writeErr != nil {
				suite.T().Logf("write Alpine HTTP response: %s", writeErr)
			}
		}),
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
