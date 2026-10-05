// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/talos/internal/app/hypervisord"
	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// configStore stands in for the machine configuration a power request rewrites.
type configStore struct {
	current coreconfig.Provider
	err     error

	// hold, when set, is called before the next patch and decides whether it fails.
	hold func() error
}

func (s *configStore) PatchConfiguration(_ context.Context, patch func(coreconfig.Container) (coreconfig.Provider, error)) error {
	if hold := s.hold; hold != nil {
		s.hold = nil

		if err := hold(); err != nil {
			return err
		}
	}

	if s.err != nil {
		return s.err
	}

	patched, err := patch(s.current)
	if err != nil {
		return err
	}

	s.current = patched

	return nil
}

func (s *configStore) powerState(t *testing.T, name string) hypervisorhelpers.PowerState {
	t.Helper()

	for _, vm := range s.current.VirtualMachineConfigs() {
		if vm.Name() == name {
			return vm.PowerState()
		}
	}

	t.Fatalf("virtual machine %q is no longer declared", name)

	return 0
}

func newConfigStore(t *testing.T, names ...string) *configStore {
	t.Helper()

	documents := make([]config.Document, 0, len(names))

	for _, name := range names {
		vm := &hypervisorcfg.VirtualMachineConfigV1Alpha1{
			MetaAPIVersion:   "v1alpha1",
			MetaKind:         hypervisorcfg.VirtualMachineConfigKind,
			MetaName:         name,
			PowerStateConfig: hypervisorhelpers.PowerStateRunning,
			CPUConfig:        hypervisorcfg.VirtualMachineCPU{CPUCount: 1},
			MemoryConfig:     hypervisorcfg.VirtualMachineMemory{MemorySize: meta.MustByteSize("1GiB")},
			FirmwareConfig: hypervisorcfg.VirtualMachineFirmware{
				FirmwareType: hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
			},
		}
		documents = append(documents, vm)
	}

	cfg, err := container.New(documents...)
	require.NoError(t, err)

	return &configStore{current: cfg}
}

// powerClient wires a service which can drive power as well as serve consoles.
func powerClient(t *testing.T, st state.State, cfg hypervisord.ConfigPatcher, domains func(context.Context) (domain.Client, error)) machine.HypervisorServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()

	open := func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
		return nil, errors.New("no console in this test")
	}

	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, open,
		hypervisord.WithDomainConnector(domains), hypervisord.WithConfigPatcher(cfg), hypervisord.WithPowerState(st)))

	done := make(chan error, 1)

	go func() { done <- server.Serve(listener) }()

	conn, err := grpc.NewClient("passthrough:///power", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		server.Stop()
		require.NoError(t, <-done)
	})

	return machine.NewHypervisorServiceClient(conn)
}

// The machine configuration carries the power state and nothing else; the way a stop is carried
// out is a parameter of that transition, and lives outside it.
func TestStopAndStartRewriteTheMachineConfiguration(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	cfg := newConfigStore(t, "vm1", "vm2")
	c := powerClient(t, st, cfg, nil)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)

	require.Equal(t, hypervisorhelpers.PowerStateStopped, cfg.powerState(t, "vm1"))
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"),
		"a stop asks the guest unless it is told to force")

	// Only the machine which was named moves.
	require.Equal(t, hypervisorhelpers.PowerStateRunning, cfg.powerState(t, "vm2"))

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
	require.NoError(t, err)

	require.Equal(t, hypervisorhelpers.StopModeForced.String(), stopMode(t, st, "vm1"),
		"forcing is recorded, not left to the absence of a record")

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)

	require.Equal(t, hypervisorhelpers.PowerStateRunning, cfg.powerState(t, "vm1"))
}

// A start ends whatever stop was asked for before it, so the record of a graceful stop the guest
// never obeyed cannot decide how a later stop is carried out.
func TestStartWithdrawsTheStopMode(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	c := powerClient(t, st, newConfigStore(t, "vm1"), nil)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"))

	// The record is owned by the controller which retires it, which is what lets it do so.
	res, err := safe.StateGetByID[*hypervisor.VirtualMachineStopMode](t.Context(), st, "vm1")
	require.NoError(t, err)
	require.Equal(t, hypervisor.VirtualMachineStopModeOwner, res.Metadata().Owner())

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Empty(t, stopMode(t, st, "vm1"))
}

// A stop which could not be driven leaves the record as it found it.
func TestFailedStopRestoresTheStopMode(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	cfg := newConfigStore(t, "vm1")
	c := powerClient(t, st, cfg, nil)

	cfg.err = status.Error(codes.FailedPrecondition, "another apply configuration is already in progress")

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Empty(t, stopMode(t, st, "vm1"), "a stop which never started records nothing")

	// A graceful stop in flight is not turned into a destroy by a forced one which failed.
	cfg.err = nil

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)

	cfg.err = status.Error(codes.FailedPrecondition, "another apply configuration is already in progress")

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"))
}

// stopMode reads how the next stop of a virtual machine has been asked to be carried out, or an
// empty string when nothing asked for anything.
func stopMode(t *testing.T, st state.State, name string) string {
	t.Helper()

	res, err := safe.StateGetByID[*hypervisor.VirtualMachineStopMode](t.Context(), st, name)
	if state.IsNotFoundError(err) {
		return ""
	}

	require.NoError(t, err)

	return res.TypedSpec().Mode
}

// A stop which fails restores what was recorded before it, which must never be the record of a stop
// which overtook it and succeeded.
func TestFailedStopDoesNotUndoAConcurrentStop(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	cfg := newConfigStore(t, "vm1")
	c := powerClient(t, st, cfg, nil)

	entered := make(chan struct{})
	release := make(chan struct{})

	cfg.hold = func() error {
		close(entered)
		<-release

		return status.Error(codes.FailedPrecondition, "another apply configuration is already in progress")
	}

	forced := make(chan error, 1)

	go func() {
		_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
		forced <- err
	}()

	<-entered

	graceful := make(chan error, 1)

	go func() {
		_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
		graceful <- err
	}()

	// Unserialized, the graceful stop finishes in this window, and the forced one then restores the
	// absence of a record over it. Serialized, it waits for the forced one to give up.
	<-time.After(100 * time.Millisecond)

	close(release)

	require.Equal(t, codes.FailedPrecondition, status.Code(<-forced))
	require.NoError(t, <-graceful)

	require.Equal(t, hypervisorhelpers.PowerStateStopped, cfg.powerState(t, "vm1"))
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"),
		"the stop which succeeded is carried out the way it asked for")
}

func TestPowerRequestsRefuseUndeclaredMachines(t *testing.T) {
	t.Parallel()

	st := setup(t)
	cfg := newConfigStore(t, "vm1")
	c := powerClient(t, st, cfg, nil)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "other"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	// A stop which was refused leaves no record behind.
	require.Empty(t, stopMode(t, st, "other"))

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "other", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: ""})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPowerRequestsSurfaceAConfigurationWriteRefusal(t *testing.T) {
	t.Parallel()

	st := setup(t)
	cfg := newConfigStore(t, "vm1")
	cfg.err = status.Error(codes.FailedPrecondition, "another apply configuration is already in progress")
	c := powerClient(t, st, cfg, nil)

	_, err := c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRebootRequiresAReadyMachine(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		stage   hypervisor.VirtualMachineStage
		power   string
		managed bool
		code    codes.Code
	}{
		"unmanaged":  {managed: false, code: codes.NotFound},
		"not driven": {managed: true, power: "stopped", stage: hypervisor.VirtualMachineStageReady, code: codes.FailedPrecondition},
		"not ready":  {managed: true, power: "running", stage: hypervisor.VirtualMachineStagePending, code: codes.FailedPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := setup(t)

			if test.managed {
				addVM(t, st, false, test.power)

				vmStatus := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
				vmStatus.TypedSpec().Stage = test.stage
				require.NoError(t, st.Create(t.Context(), vmStatus))
			}

			c := powerClient(t, st, newConfigStore(t, "vm1"), func(context.Context) (domain.Client, error) {
				t.Error("libvirt must not be reached for a machine which cannot be rebooted")

				return nil, errors.New("unreachable")
			})

			_, err := c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
			require.Equal(t, test.code, status.Code(err))
		})
	}
}

// A reboot leaves the power state the machine is driven towards alone.
func TestRebootDoesNotTouchTheMachineConfiguration(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")

	vmStatus := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
	vmStatus.TypedSpec().Stage = hypervisor.VirtualMachineStageReady
	require.NoError(t, st.Create(t.Context(), vmStatus))

	cfg := newConfigStore(t, "vm1")
	fake := &rebootClient{}
	c := powerClient(t, st, cfg, func(context.Context) (domain.Client, error) { return fake, nil })

	_, err := c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, 1, fake.reboots, "a reboot asks the guest unless it is told to force")
	require.Zero(t, fake.removes)

	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.NoError(t, err)
	require.Equal(t, 1, fake.removes, "a forced reboot destroys the domain and lets it be defined again")

	require.Equal(t, hypervisorhelpers.PowerStateRunning, cfg.powerState(t, "vm1"))
	require.True(t, fake.closed, "the libvirt session is not left open")
}

type rebootClient struct {
	domain.Client

	reboots int
	removes int
	closed  bool
}

func (c *rebootClient) Reboot(domain.Domain) error {
	c.reboots++

	return nil
}

func (c *rebootClient) Remove(domain.Domain) error {
	c.removes++

	return nil
}

func (c *rebootClient) Close() { c.closed = true }
