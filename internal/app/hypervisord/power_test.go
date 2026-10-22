// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
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
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// declare publishes an active machine configuration declaring each named virtual machine with
// powerState.
func declare(t *testing.T, st state.State, powerState hypervisorhelpers.PowerState, names ...string) {
	t.Helper()

	documents := make([]config.Document, 0, len(names))

	for _, name := range names {
		documents = append(documents, &hypervisorcfg.VirtualMachineConfigV1Alpha1{
			MetaAPIVersion:   "v1alpha1",
			MetaKind:         hypervisorcfg.VirtualMachineConfigKind,
			MetaName:         name,
			PowerStateConfig: powerState,
			CPUConfig:        hypervisorcfg.VirtualMachineCPU{CPUCount: 1},
			MemoryConfig:     hypervisorcfg.VirtualMachineMemory{MemorySize: meta.MustByteSize("1GiB")},
			FirmwareConfig: hypervisorcfg.VirtualMachineFirmware{
				FirmwareType: hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
			},
		})
	}

	cfg, err := container.New(documents...)
	require.NoError(t, err)

	active := configres.NewMachineConfig(cfg)

	if err = st.Destroy(t.Context(), active.Metadata()); err != nil && !state.IsNotFoundError(err) {
		require.NoError(t, err)
	}

	require.NoError(t, st.Create(t.Context(), active))
}

// declaredPowerState reads the power state the active machine configuration declares.
func declaredPowerState(t *testing.T, st state.State, name string) hypervisorhelpers.PowerState {
	t.Helper()

	active, err := safe.StateGetByID[*configres.MachineConfig](t.Context(), st, configres.ActiveID)
	require.NoError(t, err)

	for _, vm := range active.Config().VirtualMachineConfigs() {
		if vm.Name() == name {
			return vm.PowerState()
		}
	}

	t.Fatalf("virtual machine %q is no longer declared", name)

	return 0
}

// hookedState runs beforeWrite ahead of every override write, and fails the write when it does.
type hookedState struct {
	state.State

	beforeWrite func() error
}

func (s *hookedState) Modify(ctx context.Context, res resource.Resource, update func(resource.Resource) error, options ...state.UpdateOption) error {
	if err := s.beforeWrite(); err != nil {
		return err
	}

	return s.State.Modify(ctx, res, update, options...)
}

// powerClient wires a service which can drive power as well as serve consoles.
func powerClient(t *testing.T, st state.State) machine.HypervisorServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()

	open := func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
		return nil, errors.New("no console in this test")
	}

	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, open,
		hypervisord.WithPowerState(st)))

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

// Start and Stop override the power state for this boot and leave the machine configuration
// alone; the way a stop is carried out rides on the override.
func TestStopAndStartOverrideThePowerState(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1", "vm2")
	c := powerClient(t, st)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)

	require.Equal(t, "stopped", overridePower(t, st, "vm1"))
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"),
		"a stop asks the guest unless it is told to force")
	require.Equal(t, hypervisorhelpers.PowerStateRunning, declaredPowerState(t, st, "vm1"), "the configuration is not touched")

	// Only the machine which was named moves.
	require.Empty(t, overridePower(t, st, "vm2"))

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
	require.NoError(t, err)

	require.Equal(t, hypervisorhelpers.StopModeForced.String(), stopMode(t, st, "vm1"),
		"forcing is recorded, not left to the absence of a record")

	// Starting a machine configured to run leaves nothing to override.
	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Empty(t, overridePower(t, st, "vm1"))
}

// A machine configured to be stopped is started by an override, which a stop then replaces.
func TestStartOverridesAStoppedConfiguration(t *testing.T) {
	t.Parallel()

	st := setup(t)
	declare(t, st, hypervisorhelpers.PowerStateStopped, "vm1")
	c := powerClient(t, st)

	_, err := c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, "running", overridePower(t, st, "vm1"))
	require.Empty(t, stopMode(t, st, "vm1"))

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, "stopped", overridePower(t, st, "vm1"))
	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"))
	require.Equal(t, hypervisorhelpers.PowerStateStopped, declaredPowerState(t, st, "vm1"))
}

// Asking again for the stop in effect keeps its request, so the guest is not asked anew; forcing a
// graceful stop is a change.
func TestStopAlreadyInEffectKeepsItsRequest(t *testing.T) {
	t.Parallel()

	st := setup(t)
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	c := powerClient(t, st)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)

	request := stopRequest(t, st, "vm1")

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, request, stopRequest(t, st, "vm1"), "the stop in effect is not replaced")

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
	require.NoError(t, err)
	require.NotEqual(t, request, stopRequest(t, st, "vm1"))
	require.Equal(t, hypervisorhelpers.StopModeForced.String(), stopMode(t, st, "vm1"))
}

// overridePower reads the power state a virtual machine is overridden to, or an empty string when
// it is not overridden.
func overridePower(t *testing.T, st state.State, name string) string {
	t.Helper()

	override, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](t.Context(), st, name)
	if state.IsNotFoundError(err) {
		return ""
	}

	require.NoError(t, err)

	return override.TypedSpec().PowerState
}

// stopRequest reads the stop request of a virtual machine's power override.
func stopRequest(t *testing.T, st state.State, name string) string {
	t.Helper()

	override, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](t.Context(), st, name)
	if state.IsNotFoundError(err) {
		return ""
	}

	require.NoError(t, err)

	return override.TypedSpec().StopRequest
}

// stopMode reads how the next stop of a virtual machine has been asked to be carried out, or an
// empty string when nothing asked for anything.
func stopMode(t *testing.T, st state.State, name string) string {
	t.Helper()

	request := stopRequest(t, st, name)
	if request == "" {
		return ""
	}

	return hypervisorhelpers.VirtualMachineStopRequestMode(request).String()
}

// Concurrent stops are serialized, so a stop which fails cannot interleave with one which succeeds.
func TestFailedStopDoesNotUndoAConcurrentStop(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")

	entered := make(chan struct{})
	release := make(chan struct{})

	var once sync.Once

	hooked := &hookedState{State: st, beforeWrite: func() error {
		failed := false

		once.Do(func() {
			close(entered)
			<-release

			failed = true
		})

		if failed {
			return errors.New("injected write failure")
		}

		return nil
	}}
	c := powerClient(t, hooked)

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

	// Unserialized, the graceful stop finishes in this window. Serialized, it waits for the forced
	// one to give up.
	<-time.After(100 * time.Millisecond)

	require.Empty(t, overridePower(t, st, "vm1"))

	close(release)

	require.Error(t, <-forced)
	require.NoError(t, <-graceful)

	require.Equal(t, hypervisorhelpers.StopModeGraceful.String(), stopMode(t, st, "vm1"),
		"the stop which succeeded is carried out the way it asked for")
}

func TestPowerRequestsRefuseUndeclaredMachines(t *testing.T) {
	t.Parallel()

	st := setup(t)
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	c := powerClient(t, st)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "other"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	// A stop which was refused leaves no record behind.
	require.Empty(t, stopMode(t, st, "other"))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "other"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: ""})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
