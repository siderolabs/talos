// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

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
	// OpenstackExternalIPEndpoint is the local EC2 endpoint for the external IP.
	OpenstackExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"

	// OpenstackHostnameEndpoint is the local EC2 endpoint for the hostname.
	OpenstackHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"

	// OpenstackUserDataEndpoint is the local EC2 endpoint for the config.
	OpenstackUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// Openstack is the concrete type that implements the runtime.Platform interface.
type Openstack struct{}

// Name implements the runtime.Platform interface.
func (a *Openstack) Name() string {
	return "openstack"
}

// Configuration implements the runtime.Platform interface.
func (a *Openstack) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", OpenstackUserDataEndpoint)

	return download.Download(ctx, OpenstackUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (a *Openstack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (a *Openstack) Hostname(ctx context.Context) (hostname []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OpenstackHostnameEndpoint, nil)
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
func (a *Openstack) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, "GET", OpenstackExternalIPEndpoint, nil); err != nil {
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
func (a *Openstack) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
