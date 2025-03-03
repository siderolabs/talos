// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vip_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/vip"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestEquinixMetalHandler(t *testing.T) {
	// WARNING: this test requires interaction with Equinix Metal API with real device IDs and API token
	// it is skipped by default unless following variables are set:
	//   TALOS_EM_API_TOKEN
	//   TALOS_EM_PROJECT_ID
	//   TALOS_EM_DEVICE_ID_1
	//   TALOS_EM_DEVICE_ID_2
	//   TALOS_EM_VIP
	settings := map[string]string{}

	for _, variable := range []string{
		"TALOS_EM_API_TOKEN",
		"TALOS_EM_PROJECT_ID",
		"TALOS_EM_DEVICE_ID_1",
		"TALOS_EM_DEVICE_ID_2",
		"TALOS_EM_VIP",
	} {
		var ok bool

		settings[variable], ok = os.LookupEnv(variable)

		if !ok {
			t.Skip("skipping the test as the environment variable is not set", variable)
		}
	}

	logger := zaptest.NewLogger(t)

	handler1 := vip.NewEquinixMetalHandler(logger, settings["TALOS_EM_VIP"], network.VIPEquinixMetalSpec{
		ProjectID: settings["TALOS_EM_PROJECT_ID"],
		DeviceID:  settings["TALOS_EM_DEVICE_ID_1"],
		APIToken:  settings["TALOS_EM_API_TOKEN"],
	})

	handler2 := vip.NewEquinixMetalHandler(logger, settings["TALOS_EM_VIP"], network.VIPEquinixMetalSpec{
		ProjectID: settings["TALOS_EM_PROJECT_ID"],
		DeviceID:  settings["TALOS_EM_DEVICE_ID_2"],
		APIToken:  settings["TALOS_EM_API_TOKEN"],
	})

	// not graceful
	require.NoError(t, handler1.Acquire(t.Context()))
	require.NoError(t, handler2.Acquire(t.Context()))

	// graceful
	require.NoError(t, handler1.Acquire(t.Context()))
	require.NoError(t, handler1.Release(t.Context()))
	require.NoError(t, handler2.Acquire(t.Context()))
	require.NoError(t, handler2.Release(t.Context()))
}
