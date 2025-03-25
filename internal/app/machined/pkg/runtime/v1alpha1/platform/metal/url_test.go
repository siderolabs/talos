// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func createOrUpdate(ctx context.Context, st state.State, r resource.Resource) error {
	oldRes, err := st.Get(ctx, r.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if oldRes == nil {
		err = st.Create(ctx, r)
		if err != nil {
			return err
		}
	} else {
		r.Metadata().SetVersion(oldRes.Metadata().Version())

		err = st.Update(ctx, r)
		if err != nil {
			return err
		}
	}

	return nil
}

func setup(ctx context.Context, t *testing.T, st state.State, mockUUID, mockSerialNumber, mockHostname, mockMAC string) {
	sysInfo := hardware.NewSystemInformation(hardware.SystemInformationID)
	sysInfo.TypedSpec().UUID = mockUUID
	sysInfo.TypedSpec().SerialNumber = mockSerialNumber
	assert.NoError(t, createOrUpdate(ctx, st, sysInfo))

	hostnameSpec := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameSpec.TypedSpec().Hostname = mockHostname
	assert.NoError(t, createOrUpdate(ctx, st, hostnameSpec))

	linkStatusSpec := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
	parsedMockMAC, err := net.ParseMAC(mockMAC)
	assert.NoError(t, err)

	linkStatusSpec.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(parsedMockMAC)
	assert.NoError(t, createOrUpdate(ctx, st, linkStatusSpec))

	netStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	netStatus.TypedSpec().AddressReady = true
	assert.NoError(t, createOrUpdate(ctx, st, netStatus))
}

func TestRepopulateOnRetry(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	nCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch nCalls {
		case 0:
			assert.Equal(t, "h=myTestHostname&m=52%3A2f%3Afd%3Adf%3Afc%3Ac0&s=0OCZJ19N65&u=40dcbd19-3b10-444e-bfff-aaee44a51fda", r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)

			// After the first call we change the resources that should be substituted in the next call.
			uuid2 := "9fba530f-767d-40f9-9410-bb1fed5d2134"
			mac2 := "aa:aa:bb:bb:cc:cc"
			serialNumber2 := "111AAA9N65"
			hostname2 := "anotherHostname"

			setup(ctx, t, st, uuid2, serialNumber2, hostname2, mac2)
		case 1:
			// Before the second call Configuration() should have resubstituted all the new parameters in the URL.
			assert.Equal(t, "h=anotherHostname&m=aa%3Aaa%3Abb%3Abb%3Acc%3Acc&s=111AAA9N65&u=9fba530f-767d-40f9-9410-bb1fed5d2134", r.URL.RawQuery)
			w.WriteHeader(http.StatusOK)
		}

		nCalls++
	}))
	defer server.Close()

	uuid1 := "40dcbd19-3b10-444e-bfff-aaee44a51fda"
	mac1 := "52:2f:fd:df:fc:c0"
	serialNumber1 := "0OCZJ19N65"
	hostname1 := "myTestHostname"

	setup(ctx, t, st, uuid1, serialNumber1, hostname1, mac1)

	downloadURL := server.URL + "/metadata?h=${hostname}&m=${mac}&s=${serial}&u=${uuid}"

	param := procfs.NewParameter(constants.KernelParamConfig)
	param.Append(downloadURL)

	procfs.ProcCmdline().Set(constants.KernelParamConfig, param)
	defer procfs.ProcCmdline().Set(constants.KernelParamConfig, nil)

	go func() {
		testObj := metal.Metal{}
		_, err := testObj.Configuration(ctx, st)
		assert.NoError(t, err)

		cancel()
	}()

	<-ctx.Done()
}
