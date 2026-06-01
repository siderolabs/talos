// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Value of a variable.
type Value interface {
	// Get the value.
	Get() string
	// RegisterWatch handles registering a watch for the variable.
	RegisterWatch(ctx context.Context, st state.State, ch chan<- state.Event) error
	// EventHandler is called for each watch event, returns when the variable value is ready.
	EventHandler(event state.Event) (bool, error)
}

type value struct {
	mu  sync.Mutex
	val string

	registerWatch func(ctx context.Context, st state.State, ch chan<- state.Event) error
	eventHandler  func(event state.Event) (string, error)
}

func (v *value) Get() string {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.val
}

func (v *value) RegisterWatch(ctx context.Context, st state.State, ch chan<- state.Event) error {
	return v.registerWatch(ctx, st, ch)
}

func (v *value) EventHandler(event state.Event) (bool, error) {
	val, err := v.eventHandler(event)
	if err != nil {
		return false, err
	}

	if val == "" {
		return false, nil
	}

	v.mu.Lock()
	v.val = val
	v.mu.Unlock()

	return true, nil
}

// UUIDValue is a value for UUID variable.
func UUIDValue() Value {
	return &value{
		registerWatch: func(ctx context.Context, st state.State, ch chan<- state.Event) error {
			return st.Watch(ctx, hardware.NewSystemInformation(hardware.SystemInformationID).Metadata(), ch)
		},
		eventHandler: func(event state.Event) (string, error) {
			sysInfo, ok := event.Resource.(*hardware.SystemInformation)
			if !ok {
				return "", nil
			}

			return sysInfo.TypedSpec().UUID, nil
		},
	}
}

// SerialNumberValue is a value for SerialNumber variable.
func SerialNumberValue() Value {
	return &value{
		registerWatch: func(ctx context.Context, st state.State, ch chan<- state.Event) error {
			return st.Watch(ctx, hardware.NewSystemInformation(hardware.SystemInformationID).Metadata(), ch)
		},
		eventHandler: func(event state.Event) (string, error) {
			sysInfo, ok := event.Resource.(*hardware.SystemInformation)
			if !ok {
				return "", nil
			}

			return sysInfo.TypedSpec().SerialNumber, nil
		},
	}
}

// MACValue is a value for MAC variable.
func MACValue() Value {
	return &value{
		registerWatch: func(ctx context.Context, st state.State, ch chan<- state.Event) error {
			return st.Watch(ctx, network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr).Metadata(), ch)
		},
		eventHandler: func(event state.Event) (string, error) {
			hwAddr, ok := event.Resource.(*network.HardwareAddr)
			if !ok {
				return "", nil
			}

			return hwAddr.TypedSpec().HardwareAddr.String(), nil
		},
	}
}

// HostnameValue is a value for Hostname variable.
func HostnameValue() Value {
	return &value{
		registerWatch: func(ctx context.Context, st state.State, ch chan<- state.Event) error {
			return st.Watch(ctx, network.NewHostnameStatus(network.NamespaceName, network.HostnameID).Metadata(), ch)
		},
		eventHandler: func(event state.Event) (string, error) {
			hostname, ok := event.Resource.(*network.HostnameStatus)
			if !ok {
				return "", nil
			}

			return hostname.TypedSpec().Hostname, nil
		},
	}
}

// CodeValue is a value for Code variable.
func CodeValue() Value {
	return &value{
		registerWatch: func(ctx context.Context, st state.State, ch chan<- state.Event) error {
			return st.Watch(ctx, runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.DownloadURLCode)).Metadata(), ch)
		},
		eventHandler: func(event state.Event) (string, error) {
			code, ok := event.Resource.(*runtime.MetaKey)
			if !ok {
				return "", nil
			}

			return code.TypedSpec().Value, nil
		},
	}
}
