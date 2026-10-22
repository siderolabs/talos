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
func powerClient(t *testing.T, st state.State, domains func(context.Context) (domain.Client, error)) machine.HypervisorServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()

	open := func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
		return nil, errors.New("no console in this test")
	}

	machine.RegisterHypervisorServiceServer(server, hypervisord.NewService(st, open,
		hypervisord.WithDomainConnector(domains), hypervisord.WithPowerState(st)))

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
	c := powerClient(t, st, nil)

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
	addVM(t, st, false, "stopped")
	declare(t, st, hypervisorhelpers.PowerStateStopped, "vm1")
	c := powerClient(t, st, nil)

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

// Power operations use the producer's VM spec even when no machine configuration exists.
func TestPowerOperationsOnExternallyProducedVM(t *testing.T) {
	t.Parallel()

	st := setup(t)
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	spec.TypedSpec().PowerState = "stopped"
	require.NoError(t, st.Create(t.Context(), spec))

	c := powerClient(t, st, func(context.Context) (domain.Client, error) { return &rebootClient{}, nil })
	_, err := c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, "running", overridePower(t, st, "vm1"), "effective running state does not replace stopped baseline")

	createReadyRebootStatus(t, st)
	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
	require.NoError(t, err)

	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, "stopped", overridePower(t, st, "vm1"))
	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "pending stop fences reboot before projection")
}

func TestStartRemovesOverrideForExternalRunningBaseline(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	c := powerClient(t, st, nil)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, "stopped", overridePower(t, st, "vm1"))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Empty(t, overridePower(t, st, "vm1"))
}

func TestPowerOperationsRejectMissingOrUnknownBaseline(t *testing.T) {
	t.Parallel()

	for _, baseline := range []string{"", "invalid"} {
		t.Run(baseline, func(t *testing.T) {
			t.Parallel()

			st := setup(t)
			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
			spec.TypedSpec().PowerState = baseline
			require.NoError(t, st.Create(t.Context(), spec))
			c := powerClient(t, st, func(context.Context) (domain.Client, error) {
				t.Error("invalid baseline must be rejected before connecting to libvirt")

				return nil, errors.New("unreachable")
			})
			_, err := c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			createReadyRebootStatus(t, st)
			_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Empty(t, overridePower(t, st, "vm1"))
		})
	}
}

func TestPowerOperationsRejectTearingDownSpec(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](t.Context(), st, "vm1")
	require.NoError(t, err)
	require.NoError(t, st.AddFinalizer(t.Context(), spec.Metadata(), "test.power"))
	_, err = st.Teardown(t.Context(), spec.Metadata())
	require.NoError(t, err)

	c := powerClient(t, st, func(context.Context) (domain.Client, error) {
		t.Error("tearing-down spec must not reach libvirt")

		return nil, errors.New("unreachable")
	})
	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Empty(t, overridePower(t, st, "vm1"))
}

// Asking again for the stop in effect keeps its request, so the guest is not asked anew; forcing a
// graceful stop is a change.
func TestStopAlreadyInEffectKeepsItsRequest(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	c := powerClient(t, st, nil)

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
	c := powerClient(t, hooked, nil)

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
	c := powerClient(t, st, nil)

	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "other"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	// A stop which was refused leaves no record behind.
	require.Empty(t, stopMode(t, st, "other"))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "other"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = c.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: ""})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRebootRejectsStaleReadyStatusAfterRunningOverride(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "stopped")
	createReadyRebootStatus(t, st)
	client := powerClient(t, st, func(context.Context) (domain.Client, error) {
		t.Error("old readiness must not authorize reboot")

		return &rebootClient{}, nil
	})
	_, err := client.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
	require.NoError(t, err)
	_, err = client.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRebootRejectsReadinessFromEarlierDomain(t *testing.T) {
	t.Parallel()

	for _, recreate := range []bool{false, true} {
		t.Run(map[bool]string{false: "override removal", true: "domain recreation"}[recreate], func(t *testing.T) {
			t.Parallel()

			st := setup(t)
			addVM(t, st, false, "running")
			createReadyRebootStatus(t, st)

			connections := make(chan struct{}, 2)
			client := powerClient(t, st, func(context.Context) (domain.Client, error) {
				connections <- struct{}{}

				return &rebootClient{}, nil
			})
			definition, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](t.Context(), st, "vm1")
			require.NoError(t, err)

			if recreate {
				require.NoError(t, st.Destroy(t.Context(), definition.Metadata()))

				replacement := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm1")
				*replacement.TypedSpec() = *definition.TypedSpec()
				require.NoError(t, st.Create(t.Context(), replacement))
			} else {
				_, err = client.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
				require.NoError(t, err)
				_, err = safe.StateUpdateWithConflicts(t.Context(), st, definition.Metadata(), func(current *hypervisor.VirtualMachineDomainSpec) error {
					current.TypedSpec().PowerState = "stopped"

					return nil
				})
				require.NoError(t, err)
				_, err = client.Start(t.Context(), &machine.VirtualMachineStartRequest{Name: "vm1"})
				require.NoError(t, err)
				_, err = safe.StateUpdateWithConflicts(t.Context(), st, definition.Metadata(), func(current *hypervisor.VirtualMachineDomainSpec) error {
					current.TypedSpec().PowerState = "running"

					return nil
				})
				require.NoError(t, err)
			}

			_, err = client.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Empty(t, connections, "stale readiness must be rejected before connecting")

			definition, err = safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](t.Context(), st, "vm1")
			require.NoError(t, err)
			_, err = safe.StateUpdateWithConflicts(t.Context(), st,
				hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1").Metadata(),
				func(current *hypervisor.VirtualMachineStatus) error {
					current.TypedSpec().ObservedGeneration = hypervisor.VirtualMachineStatusGeneration(definition.TypedSpec().ObservedGeneration, definition)

					return nil
				})
			require.NoError(t, err)
			_, err = client.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
			require.NoError(t, err)
			require.Len(t, connections, 1)
		})
	}
}

func TestRebootRequiresAReadyMachine(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		stage   hypervisorhelpers.VirtualMachineStage
		power   string
		managed bool
		code    codes.Code
	}{
		"unmanaged":  {managed: false, code: codes.NotFound},
		"not driven": {managed: true, power: "stopped", stage: hypervisorhelpers.VirtualMachineStageReady, code: codes.FailedPrecondition},
		"not ready":  {managed: true, power: "running", stage: hypervisorhelpers.VirtualMachineStagePending, code: codes.FailedPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := setup(t)

			if test.managed {
				addVM(t, st, false, test.power)

				spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](t.Context(), st, "vm1")
				require.NoError(t, err)

				vmStatus := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
				vmStatus.TypedSpec().ObservedGeneration = hypervisor.VirtualMachineSpecGeneration(spec)
				vmStatus.TypedSpec().Stage = test.stage
				require.NoError(t, st.Create(t.Context(), vmStatus))
			}

			c := powerClient(t, st, func(context.Context) (domain.Client, error) {
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

	createReadyRebootStatus(t, st)

	fake := &rebootClient{}
	c := powerClient(t, st, func(context.Context) (domain.Client, error) { return fake, nil })

	_, err := c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, 1, fake.reboots, "a reboot asks the guest unless it is told to force")
	require.Zero(t, fake.removes)

	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.NoError(t, err)
	require.Equal(t, 1, fake.removes, "a forced reboot destroys the domain and lets it be defined again")

	require.Empty(t, overridePower(t, st, "vm1"))
	require.True(t, fake.closed, "the libvirt session is not left open")
}

func createReadyRebootStatus(t *testing.T, st state.State) {
	t.Helper()

	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](t.Context(), st, "vm1")
	require.NoError(t, err)

	override, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](t.Context(), st, "vm1")
	if !state.IsNotFoundError(err) {
		require.NoError(t, err)
	}

	generation := hypervisor.VirtualMachineEffectiveGeneration(spec, override)

	domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm1")
	domainSpec.TypedSpec().PowerState = "running"
	domainSpec.TypedSpec().DomainXML = "<domain/>"
	domainSpec.TypedSpec().ObservedGeneration = generation
	require.NoError(t, st.Create(t.Context(), domainSpec))

	vmStatus := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
	vmStatus.TypedSpec().ObservedGeneration = hypervisor.VirtualMachineStatusGeneration(generation, domainSpec)
	vmStatus.TypedSpec().Stage = hypervisorhelpers.VirtualMachineStageReady
	vmStatus.TypedSpec().PowerState = hypervisorhelpers.VirtualMachinePowerStateRunning
	require.NoError(t, st.Create(t.Context(), vmStatus))
}

func TestRebootRefusesStaleReadiness(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)
	_, err := safe.StateUpdateWithConflicts(t.Context(), st,
		hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1").Metadata(),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().Memory.Size++

			return nil
		})
	require.NoError(t, err)

	c := powerClient(t, st, func(context.Context) (domain.Client, error) {
		t.Error("stale readiness must be refused before connecting to libvirt")

		return &rebootClient{}, nil
	})
	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRebootRefusesReadinessFromPreviousVMLifetime(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)

	previous, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](t.Context(), st, "vm1")
	require.NoError(t, err)
	require.NoError(t, st.Destroy(t.Context(), previous.Metadata()))

	current := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	current.TypedSpec().PowerState = "running"
	current.TypedSpec().Firmware.Type = "unsupported"
	require.NoError(t, st.Create(t.Context(), current))
	require.Equal(t, previous.Metadata().Version(), current.Metadata().Version(), "recreation must reuse the numeric version")

	fake := &rebootClient{}
	c := powerClient(t, st, func(context.Context) (domain.Client, error) {
		return fake, nil
	})

	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Zero(t, fake.removes, "readiness from a previous spec lifetime must never authorize destruction")
}

func TestRebootRechecksReadinessAfterConnecting(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)

	fake := &rebootClient{}
	c := powerClient(t, st, func(context.Context) (domain.Client, error) {
		_, err := safe.StateUpdateWithConflicts(t.Context(), st,
			hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1").Metadata(),
			func(spec *hypervisor.VirtualMachineSpec) error {
				spec.TypedSpec().PowerState = "stopped"

				return nil
			})
		if err != nil {
			return nil, err
		}

		return fake, nil
	})

	_, err := c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Zero(t, fake.removes, "a superseding stop must not turn into a destructive reboot")
	require.True(t, fake.closed)
}

func TestRebootRefusesCommittedStopBeforeProjection(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)
	c := powerClient(t, st, func(context.Context) (domain.Client, error) {
		t.Error("a committed stop must not be rebooted while projections catch up")

		return &rebootClient{}, nil
	})
	_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
	require.NoError(t, err)
	_, err = c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestStopCommitsWhileRebootIsConnecting(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)

	connected := make(chan struct{})
	release := make(chan struct{})

	releaseConnect := sync.OnceFunc(func() { close(release) })
	defer releaseConnect()

	fake := &rebootClient{}
	client := powerClient(t, st, func(context.Context) (domain.Client, error) {
		close(connected)
		<-release

		return fake, nil
	})
	rebootDone := make(chan error, 1)

	go func() {
		_, err := client.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
		rebootDone <- err
	}()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("reboot did not reach the connector")
	}

	stopDone := make(chan error, 1)

	go func() {
		_, err := client.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1", Force: true})
		stopDone <- err
	}()

	blocked := false

	var stopErr error
	select {
	case stopErr = <-stopDone:
	case <-time.After(time.Second):
		blocked = true
	}

	releaseConnect()

	if blocked {
		stopErr = <-stopDone
	}

	rebootErr := <-rebootDone

	require.False(t, blocked, "Stop must commit while the libvirt connector is blocked")
	require.NoError(t, stopErr)
	require.Equal(t, codes.FailedPrecondition, status.Code(rebootErr))
	require.Zero(t, fake.removes)
	require.Zero(t, fake.reboots)
}

func TestRebootRejectsProjectedCanonicalStopWithoutAPIRequest(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)

	fake := &rebootClient{}
	client := powerClient(t, st, func(context.Context) (domain.Client, error) {
		return fake, nil
	})

	// A plain config edit is not itself a power-operation fence: wait for its VM spec
	// projection before the service can observe the new original intent.
	declare(t, st, hypervisorhelpers.PowerStateStopped, "vm1")
	_, err := safe.StateUpdateWithConflicts(t.Context(), st,
		hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1").Metadata(),
		func(spec *hypervisor.VirtualMachineSpec) error {
			spec.TypedSpec().PowerState = "stopped"

			return nil
		})
	require.NoError(t, err)

	_, err = client.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Zero(t, fake.removes)
	require.Zero(t, fake.reboots)
}

func TestRebootWaitsForInFlightStop(t *testing.T) {
	t.Parallel()

	st := setup(t)
	addVM(t, st, false, "running")
	declare(t, st, hypervisorhelpers.PowerStateRunning, "vm1")
	createReadyRebootStatus(t, st)

	patchEntered := make(chan struct{})
	releasePatch := make(chan struct{})

	releaseStop := sync.OnceFunc(func() { close(releasePatch) })
	defer releaseStop()

	var once sync.Once

	hooked := &hookedState{State: st, beforeWrite: func() error {
		var err error

		once.Do(func() {
			close(patchEntered)
			<-releasePatch

			_, err = safe.StateUpdateWithConflicts(t.Context(), st,
				hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1").Metadata(),
				func(spec *hypervisor.VirtualMachineSpec) error {
					spec.TypedSpec().PowerState = "stopped"

					return nil
				})
		})

		return err
	}}
	connected := make(chan struct{}, 1)
	fake := &rebootClient{actions: make(chan struct{}, 1)}
	c := powerClient(t, hooked, func(context.Context) (domain.Client, error) {
		connected <- struct{}{}

		return fake, nil
	})
	stopDone := make(chan error, 1)

	go func() {
		_, err := c.Stop(t.Context(), &machine.VirtualMachineStopRequest{Name: "vm1"})
		stopDone <- err
	}()

	select {
	case <-patchEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not reach its override write")
	}

	rebootDone := make(chan error, 1)

	go func() {
		_, err := c.Reboot(t.Context(), &machine.VirtualMachineRebootRequest{Name: "vm1", Force: true})
		rebootDone <- err
	}()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("reboot did not complete preflight")
	}

	select {
	case <-fake.actions:
		t.Error("reboot changed the domain while Stop was committing")
	case <-time.After(100 * time.Millisecond):
	}

	releaseStop()

	require.NoError(t, <-stopDone)
	require.Equal(t, codes.FailedPrecondition, status.Code(<-rebootDone))
}

type rebootClient struct {
	domain.Client

	reboots int
	removes int
	closed  bool
	actions chan struct{}
}

func (c *rebootClient) Reboot(domain.Domain) error {
	c.reboots++
	if c.actions != nil {
		c.actions <- struct{}{}
	}

	return nil
}

func (c *rebootClient) Remove(domain.Domain) error {
	c.removes++
	if c.actions != nil {
		c.actions <- struct{}{}
	}

	return nil
}

func (c *rebootClient) Close() { c.closed = true }
