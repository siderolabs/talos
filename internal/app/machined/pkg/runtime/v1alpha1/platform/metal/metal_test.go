// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func generateMAC() string {
	const macLen = 6

	bytes := make([]byte, macLen)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	macWithoutDashes := hex.EncodeToString(bytes)

	mac := ""

	for i := 0; i < macLen; i++ {
		mac += macWithoutDashes[i:i+2] + "-"
	}

	return mac[:len(mac)-1]
}

func generateSerialNumber() string {
	const serialNumberLetters = "01234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	const serialLen = 10

	serialNumber := []byte{}

	for i := 0; i < serialLen; i++ {
		serialNumber = append(serialNumber, serialNumberLetters[rand.Int()%len(serialNumberLetters)])
	}

	return string(serialNumber)
}

func TestPopulateURLParameters(t *testing.T) {
	mockUUID := uuid.New().String()

	mockMAC := generateMAC()

	mockSerialNumber := generateSerialNumber()

	mockHostname := "myTestHostname"

	for _, tt := range []struct {
		name          string
		url           string
		expectedURL   string
		expectedError string
	}{
		{
			name:        "no uuid",
			url:         "http://example.com/metadata",
			expectedURL: "http://example.com/metadata",
		},
		{
			name:        "empty uuid",
			url:         "http://example.com/metadata?uuid=",
			expectedURL: fmt.Sprintf("http://example.com/metadata?uuid=%s", mockUUID),
		},
		{
			name:        "uuid present",
			url:         "http://example.com/metadata?uuid=xyz",
			expectedURL: "http://example.com/metadata?uuid=xyz",
		},
		{
			name:        "other parameters",
			url:         "http://example.com/metadata?foo=a",
			expectedURL: "http://example.com/metadata?foo=a",
		},
		{
			name:        "multiple uuids",
			url:         "http://example.com/metadata?uuid=xyz&uuid=foo",
			expectedURL: fmt.Sprintf("http://example.com/metadata?uuid=%s", mockUUID),
		},
		{
			name:        "single serial number",
			url:         "http://example.com/metadata?serial=${serial}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?serial=%s", mockSerialNumber),
		},
		{
			name:        "single MAC",
			url:         "http://example.com/metadata?mac=${mac}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?mac=%s", mockMAC),
		},
		{
			name:        "single hostname",
			url:         "http://example.com/metadata?host=${hostname}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?host=%s", mockHostname),
		},
		{
			name:        "serial number, MAC and hostname",
			url:         "http://example.com/metadata?h=${hostname}&m=${mac}&s=${serial}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?h=%s&m=%s&s=%s", mockHostname, mockMAC, mockSerialNumber),
		},
		{
			name:        "MAC and UUID without variable",
			url:         "http://example.com/metadata?macaddr=${mac}&uuid=",
			expectedURL: fmt.Sprintf("http://example.com/metadata?macaddr=%s&uuid=%s", mockMAC, mockUUID),
		},
		{
			name:        "serial number and UUID without variable, order is not preserved",
			url:         "http://example.com/metadata?uuid=&ser=${serial}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?ser=%s&uuid=%s", mockSerialNumber, mockUUID),
		},
		{
			name:        "UUID variable",
			url:         "http://example.com/metadata?uuid=${uuid}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?uuid=%s", mockUUID),
		},
		{
			name:        "serial number and UUID with variable, order is not preserved",
			url:         "http://example.com/metadata?uuid=${uuid}&ser=${serial}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?ser=%s&uuid=%s", mockSerialNumber, mockUUID),
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			getSystemUUIDAndSerialNumber := func() (string, string, error) {
				return mockUUID, mockSerialNumber, nil
			}

			getMACAddress := func(context.Context, state.State) (string, error) {
				return mockMAC, nil
			}

			getHostname := func(context.Context, state.State) (string, error) {
				return mockHostname, nil
			}

			output, err := metal.PopulateURLParameters(context.Background(), tt.url, nil, getSystemUUIDAndSerialNumber, getMACAddress, getHostname)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.Equal(t, tt.expectedURL, output)
			}
		})
	}
}

type mockState struct {
	listMacAddress  *nethelpers.HardwareAddr
	watchMacAddress *nethelpers.HardwareAddr
	listHostname    string
	watchHostname   string
}

func (mock *mockState) Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error) {
	return nil, nil
}

func (mock *mockState) List(ctx context.Context, k resource.Kind, opts ...state.ListOption) (resource.List, error) {
	switch k.Type() {
	case network.LinkStatusType:
		link := network.NewLinkStatus(network.ConfigNamespaceName, "")

		if mock.listMacAddress != nil {
			link.TypedSpec().HardwareAddr = *mock.listMacAddress
			link.TypedSpec().LinkState = true
		}

		return resource.List{Items: []resource.Resource{
			link,
		}}, nil
	case network.HostnameSpecType:
		if mock.listHostname == "" {
			break
		}

		hostnameSpec := network.NewHostnameSpec(network.NamespaceName, "")
		hostnameSpec.TypedSpec().Hostname = mock.listHostname

		return resource.List{Items: []resource.Resource{
			hostnameSpec,
		}}, nil
	}

	return resource.List{Items: []resource.Resource{}}, nil
}

func (mock *mockState) Create(context.Context, resource.Resource, ...state.CreateOption) error {
	return nil
}

func (mock *mockState) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...state.UpdateOption) error {
	return nil
}

func (mock *mockState) Destroy(context.Context, resource.Pointer, ...state.DestroyOption) error {
	return nil
}

func (mock *mockState) Watch(context.Context, resource.Pointer, chan<- state.Event, ...state.WatchOption) error {
	return nil
}

func (mock *mockState) WatchKind(ctx context.Context, k resource.Kind, eventCh chan<- state.Event, opts ...state.WatchKindOption) error {
	send := func(res resource.Resource) {
		go func() {
			eventCh <- state.Event{
				Resource: res,
				Type:     state.Created,
			}
		}()
	}

	switch k.Type() {
	case network.LinkStatusType:
		link := network.NewLinkStatus(network.ConfigNamespaceName, "")

		if mock.watchMacAddress != nil {
			link.TypedSpec().HardwareAddr = *mock.watchMacAddress
			link.TypedSpec().LinkState = true
		}

		send(link)
	case network.HostnameSpecType:
		if mock.watchHostname != "" {
			hostnameSpec := network.NewHostnameSpec(network.NamespaceName, "")

			hostnameSpec.TypedSpec().Hostname = mock.watchHostname

			send(hostnameSpec)
		} else {
			send(nil)
		}
	}

	return nil
}

func (mock *mockState) UpdateWithConflicts(context.Context, resource.Pointer, state.UpdaterFunc, ...state.UpdateOption) (resource.Resource, error) {
	return nil, nil
}

func (mock *mockState) WatchFor(context.Context, resource.Pointer, ...state.WatchForConditionFunc) (resource.Resource, error) {
	return nil, nil
}

func (mock *mockState) Teardown(context.Context, resource.Pointer, ...state.TeardownOption) (bool, error) {
	return false, nil
}

func (mock *mockState) AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error {
	return nil
}

func (mock *mockState) RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error {
	return nil
}

func TestGetMACAddressAvailable(t *testing.T) {
	// given
	expectedHWAddr := nethelpers.HardwareAddr("123456")
	mock := &mockState{
		listMacAddress:  &expectedHWAddr,
		watchMacAddress: nil,
		listHostname:    "",
		watchHostname:   "",
	}

	// when
	mac, err := metal.GetMacAddress(context.Background(), mock)

	// then
	assert.NoError(t, err)
	assert.Equal(t, expectedHWAddr.String(), mac)
}

func TestGetMACAddressAvailableLater(t *testing.T) {
	// given
	expectedHWAddr := nethelpers.HardwareAddr("asdfgh")
	mock := &mockState{
		listMacAddress:  nil,
		watchMacAddress: &expectedHWAddr,
		listHostname:    "",
		watchHostname:   "",
	}

	// when
	mac, err := metal.GetMacAddress(context.Background(), mock)

	// then
	assert.NoError(t, err)
	assert.Equal(t, expectedHWAddr.String(), mac)
}

func TestGetHostnameAvailable(t *testing.T) {
	// given
	expectedHostname := "talos"
	mock := &mockState{
		listMacAddress:  nil,
		watchMacAddress: nil,
		listHostname:    expectedHostname,
		watchHostname:   "",
	}

	// when
	actualSpec, err := metal.GetHostname(context.Background(), mock)

	// then
	assert.NoError(t, err)
	assert.Equal(t, expectedHostname, actualSpec)
}

func TestGetHostnameAvailableLater(t *testing.T) {
	// given
	expectedHostname := "myhost"
	mock := &mockState{
		listMacAddress:  nil,
		watchMacAddress: nil,
		listHostname:    "",
		watchHostname:   expectedHostname,
	}

	// when
	actualHostname, err := metal.GetHostname(context.Background(), mock)

	// then
	assert.NoError(t, err)
	assert.Equal(t, expectedHostname, actualHostname)
}
