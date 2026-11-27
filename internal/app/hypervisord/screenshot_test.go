// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"context"
	"fmt"
	"testing"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/hypervisord"
	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestScreenshot(t *testing.T) {
	st := setup(t)

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Console.VNC = true
	require.NoError(t, st.Create(t.Context(), vm))

	image := []byte("\x89PNG\r\n\x1a\nimage")

	var got domain.Domain

	c := client(t, st, nil, hypervisord.WithScreenshotConnector(func(_ context.Context, d domain.Domain) ([]byte, string, error) {
		got = d

		return image, "image/png", nil
	}))

	resp, err := c.Screenshot(t.Context(), &machine.VirtualMachineScreenshotRequest{Name: "vm1"})
	require.NoError(t, err)
	require.Equal(t, image, resp.GetImage())
	require.Equal(t, "image/png", resp.GetMimeType())
	require.Equal(t, "vm1", got.Name)
	require.NotZero(t, got.UUID)
}

func TestScreenshotPreconditions(t *testing.T) {
	for _, tt := range []struct {
		name, vmName, power        string
		exists, enabled, connector bool
		code                       codes.Code
	}{
		{"unconfigured", "vm1", "running", true, true, false, codes.Unimplemented},
		{"empty name", "", "running", true, true, true, codes.InvalidArgument},
		{"missing", "vm1", "running", false, true, true, codes.NotFound},
		{"VNC disabled", "vm1", "running", true, false, true, codes.FailedPrecondition},
		{"stopped", "vm1", "stopped", true, true, true, codes.FailedPrecondition},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := setup(t)

			if tt.exists {
				vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
				vm.TypedSpec().PowerState = tt.power
				vm.TypedSpec().Console.VNC = tt.enabled
				vm.TypedSpec().Console.Serial = true
				require.NoError(t, st.Create(t.Context(), vm))
			}

			var options []hypervisord.ServiceOption
			if tt.connector {
				options = append(options, hypervisord.WithScreenshotConnector(func(context.Context, domain.Domain) ([]byte, string, error) {
					t.Error("unexpected connector call")

					return nil, "", nil
				}))
			}

			_, err := client(t, st, nil, options...).Screenshot(t.Context(), &machine.VirtualMachineScreenshotRequest{Name: tt.vmName})
			require.Equal(t, tt.code, status.Code(err))
		})
	}
}

func TestScreenshotConnectorErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
	}{
		{"too large", fmt.Errorf("capture: %w", domain.ErrScreenshotTooLarge), codes.ResourceExhausted},
		{"canceled", context.Canceled, codes.Canceled},
		{"no domain", libvirt.Error{Code: uint32(libvirt.ErrNoDomain)}, codes.NotFound},
		{"transport", libvirt.ErrInterrupted, codes.Unavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := setup(t)

			vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
			vm.TypedSpec().PowerState = "running"
			vm.TypedSpec().Console.VNC = true
			require.NoError(t, st.Create(t.Context(), vm))

			c := client(t, st, nil, hypervisord.WithScreenshotConnector(func(context.Context, domain.Domain) ([]byte, string, error) {
				return nil, "", tt.err
			}))

			_, err := c.Screenshot(t.Context(), &machine.VirtualMachineScreenshotRequest{Name: "vm1"})
			require.Equal(t, tt.code, status.Code(err), "%v", err)
		})
	}
}
