// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package digitalocean

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// DigitalOceanExternalIPEndpoint displays all external addresses associated with the instance.
	DigitalOceanExternalIPEndpoint = "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address"
	// DigitalOceanHostnameEndpoint is the local endpoint for the hostname.
	DigitalOceanHostnameEndpoint = "http://169.254.169.254/metadata/v1/hostname"
	// DigitalOceanUserDataEndpoint is the local endpoint for the config.
	DigitalOceanUserDataEndpoint = "http://169.254.169.254/metadata/v1/user-data"
)

// DigitalOcean is the concrete type that implements the platform.Platform interface.
type DigitalOcean struct{}

// Name implements the platform.Platform interface.
func (d *DigitalOcean) Name() string {
	return "digital-ocean"
}

// Configuration implements the platform.Platform interface.
func (d *DigitalOcean) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", DigitalOceanUserDataEndpoint)

	return download.Download(ctx, DigitalOceanUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the platform.Platform interface.
func (d *DigitalOcean) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the platform.Platform interface.
func (d *DigitalOcean) Hostname(ctx context.Context) (hostname []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DigitalOceanHostnameEndpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return hostname, fmt.Errorf("failed to fetch hostname from metadata service: %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

// ExternalIPs implements the runtime.Platform interface.
func (d *DigitalOcean) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, "GET", DigitalOceanExternalIPEndpoint, nil); err != nil {
		return
	}

	client := &http.Client{}
	if resp, err = client.Do(req); err != nil {
		return
	}

	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return addrs, fmt.Errorf("failed to retrieve external addresses for instance")
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	addrs = append(addrs, net.ParseIP(string(body)))

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (d *DigitalOcean) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0").Append("tty1"),
	}
}
