// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

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
	// AWSExternalIPEndpoint displays all external addresses associated with the instance.
	AWSExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// AWSHostnameEndpoint is the local EC2 endpoint for the hostname.
	AWSHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"
	// AWSUserDataEndpoint is the local EC2 endpoint for the config.
	AWSUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// AWS is the concrete type that implements the runtime.Platform interface.
type AWS struct{}

// Name implements the runtime.Platform interface.
func (a *AWS) Name() string {
	return "aws"
}

// Configuration implements the runtime.Platform interface.
func (a *AWS) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", AWSUserDataEndpoint)

	return download.Download(ctx, AWSUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (a *AWS) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (a *AWS) Hostname(ctx context.Context) (hostname []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AWSHostnameEndpoint, nil)
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
func (a *AWS) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, "GET", AWSExternalIPEndpoint, nil); err != nil {
		return
	}

	client := &http.Client{}
	if resp, err = client.Do(req); err != nil {
		return
	}

	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return addrs, fmt.Errorf("failed to retrieve external addresses for instance: %d", resp.StatusCode)
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	if addr := net.ParseIP(string(body)); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (a *AWS) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
