// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
	testID := "testID"
	sysInfo := hardware.NewSystemInformation(testID)
	sysInfo.TypedSpec().UUID = mockUUID
	sysInfo.TypedSpec().SerialNumber = mockSerialNumber
	assert.NoError(t, createOrUpdate(ctx, st, sysInfo))

	hostnameSpec := network.NewHostnameSpec(network.NamespaceName, testID)
	hostnameSpec.TypedSpec().Hostname = mockHostname
	assert.NoError(t, createOrUpdate(ctx, st, hostnameSpec))

	linkStatusSpec := network.NewLinkStatus(network.NamespaceName, testID)
	parsedMockMAC, err := net.ParseMAC(mockMAC)
	assert.NoError(t, err)

	linkStatusSpec.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(parsedMockMAC)
	linkStatusSpec.TypedSpec().LinkState = true
	assert.NoError(t, createOrUpdate(ctx, st, linkStatusSpec))
}

func TestPopulateURLParameters(t *testing.T) {
	mockUUID := "40dcbd19-3b10-444e-bfff-aaee44a51fda"

	mockMAC := "52:2f:fd:df:fc:c0"

	mockSerialNumber := "0OCZJ19N65"

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
			name:        "multiple uuids in one query parameter",
			url:         "http://example.com/metadata?u=this-${uuid}-equals-${uuid}-exactly",
			expectedURL: fmt.Sprintf("http://example.com/metadata?u=this-%s-equals-%s-exactly", mockUUID, mockUUID),
		},
		{
			name:        "uuid and mac in one query parameter",
			url:         "http://example.com/metadata?u=this-${uuid}-and-${mac}-together",
			expectedURL: fmt.Sprintf("http://example.com/metadata?u=this-%s-and-%s-together", mockUUID, mockMAC),
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
			name:        "uuid, serial number, MAC and hostname; case-insensitive",
			url:         "http://example.com/metadata?h=${HOSTname}&m=${mAC}&s=${SERIAL}&u=${uUid}",
			expectedURL: fmt.Sprintf("http://example.com/metadata?h=%s&m=%s&s=%s&u=%s", mockHostname, mockMAC, mockSerialNumber, mockUUID),
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
			ctx := context.Background()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			setup(ctx, t, st, mockUUID, mockSerialNumber, mockHostname, mockMAC)

			output, err := metal.PopulateURLParameters(ctx, tt.url, st)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				u, err := url.Parse(tt.expectedURL)
				assert.NoError(t, err)
				u.RawQuery = u.Query().Encode()
				assert.Equal(t, u.String(), output)
			}
		})
	}
}

func TestRepopulateOnRetry(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
