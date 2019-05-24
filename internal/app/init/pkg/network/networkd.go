/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"context"
	"io"
	"log"
	"os"
	"sync"

	"github.com/talos-systems/talos/pkg/userdata"
)

// Service is a wrapper for 'networkd'.
//
// It's not a standalone service, but it runs as a goroutine in init for now.
type Service struct {
	logger *log.Logger
}

// NewService create backwards compatible entry logging to stderr
func NewService() *Service {
	return &Service{
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

// Main is an entrypoint into the service
func (svc *Service) Main(ctx context.Context, data *userdata.UserData, logWriter io.Writer) error {
	svc.logger = log.New(logWriter, "networkd ", log.LstdFlags)

	var wg sync.WaitGroup

	// Launch dhclient
	if data == nil || data.Networking == nil || data.Networking.OS == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.DHCPd(ctx, DefaultInterface)
		}()
	} else {
		for _, netconf := range data.Networking.OS.Devices {
			wg.Add(1)
			go func(netconf userdata.Device) {
				defer wg.Done()
				if netconf.DHCP {
					svc.DHCPd(ctx, netconf.Interface)
				}
			}(netconf)
		}
	}

	wg.Wait()

	return nil
}
