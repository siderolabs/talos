// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	siderolinkctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

type StatusSuite struct {
	ctest.DefaultSuite
}

func TestStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
		},
	})
}

func (suite *StatusSuite) TestStatus() {
	wgClient := &mockWgClient{
		device: &wgtypes.Device{
			Peers: []wgtypes.Peer{
				{
					LastHandshakeTime: time.Now().Add(-time.Minute),
				},
			},
		},
	}

	suite.Require().NoError(suite.Runtime().RegisterController(&siderolinkctrl.StatusController{
		WGClientFunc: func() (siderolinkctrl.WireguardClient, error) {
			return wgClient, nil
		},
		Interval: 100 * time.Millisecond,
	}))

	rtestutils.AssertNoResource[*siderolink.Status](suite.Ctx(), suite.T(), suite.State(), siderolink.StatusID)

	siderolinkConfig := siderolink.NewConfig(config.NamespaceName, siderolink.ConfigID)

	siderolinkConfig.TypedSpec().APIEndpoint = "https://siderolink.example.org:1234?jointoken=supersecret&foo=bar#some=fragment"
	siderolinkConfig.TypedSpec().Host = "siderolink.example.org:1234"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), siderolinkConfig))

	suite.assertStatus("siderolink.example.org", true)

	// disconnect the peer

	wgClient.setDevice(&wgtypes.Device{
		Peers: []wgtypes.Peer{
			{LastHandshakeTime: time.Now().Add(-time.Hour)},
		},
	})

	// no device
	wgClient.setDevice(nil)
	suite.assertStatus("siderolink.example.org", false)

	// reconnect the peer
	wgClient.setDevice(&wgtypes.Device{
		Peers: []wgtypes.Peer{
			{LastHandshakeTime: time.Now().Add(-5 * time.Second)},
		},
	})

	suite.assertStatus("siderolink.example.org", true)

	// update API endpoint

	siderolinkConfig.TypedSpec().APIEndpoint = "https://new.example.org?jointoken=supersecret"
	siderolinkConfig.TypedSpec().Host = "new.example.org"

	suite.Require().NoError(suite.State().Update(suite.Ctx(), siderolinkConfig))
	suite.assertStatus("new.example.org", true)

	// no config

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), siderolinkConfig.Metadata()))
	rtestutils.AssertNoResource[*siderolink.Status](suite.Ctx(), suite.T(), suite.State(), siderolink.StatusID)
}

func (suite *StatusSuite) assertStatus(endpoint string, connected bool) {
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{siderolink.StatusID},
		func(c *siderolink.Status, assert *assert.Assertions) {
			assert.Equal(endpoint, c.TypedSpec().Host)
			assert.Equal(connected, c.TypedSpec().Connected)
		})
}

type mockWgClient struct {
	mu     sync.Mutex
	device *wgtypes.Device
}

func (m *mockWgClient) setDevice(device *wgtypes.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.device = device
}

func (m *mockWgClient) Device(string) (*wgtypes.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.device == nil {
		return nil, os.ErrNotExist
	}

	return m.device, nil
}

func (m *mockWgClient) Close() error {
	return nil
}
