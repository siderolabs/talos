// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

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

			testID := "testID"
			sysInfo := hardware.NewSystemInformation(testID)
			sysInfo.TypedSpec().UUID = mockUUID
			sysInfo.TypedSpec().SerialNumber = mockSerialNumber
			assert.NoError(t, st.Create(ctx, sysInfo))

			hostnameSpec := network.NewHostnameSpec(network.NamespaceName, testID)
			hostnameSpec.TypedSpec().Hostname = mockHostname
			assert.NoError(t, st.Create(ctx, hostnameSpec))

			linkStatusSpec := network.NewLinkStatus(network.NamespaceName, testID)
			parsedMockMAC, err := net.ParseMAC(mockMAC)
			assert.NoError(t, err)

			linkStatusSpec.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(parsedMockMAC)
			linkStatusSpec.TypedSpec().LinkState = true
			assert.NoError(t, st.Create(ctx, linkStatusSpec))

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
