/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package azure

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

const (
	// AzureUserDataEndpoint is the local endpoint for the config.
	// By specifying format=text and drilling down to the actual key we care about
	// we get a base64 encoded config response
	AzureUserDataEndpoint = "http://169.254.169.254/metadata/instance/compute/customData?api-version=2019-06-01&format=text"
	// AzureHostnameEndpoint is the local endpoint for the hostname.
	AzureHostnameEndpoint = "http://169.254.169.254/metadata/instance/compute/name?api-version=2019-06-01&format=text"
	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
)

// Azure is the concrete type that implements the platform.Platform interface.
type Azure struct{}

// Name implements the platform.Platform interface.
func (a *Azure) Name() string {
	return "Azure"
}

// Configuration implements the platform.Platform interface.
func (a *Azure) Configuration() ([]byte, error) {
	if err := linuxAgent(); err != nil {
		return nil, err
	}

	return config.Download(AzureUserDataEndpoint, config.WithHeaders(map[string]string{"Metadata": "true"}), config.WithFormat("base64"))
}

// Mode implements the platform.Platform interface.
func (a *Azure) Mode() runtime.Mode {
	return runtime.Cloud
}

// Hostname gets the hostname from the Azure metadata endpoint.
func (a *Azure) Hostname() (hostname []byte, err error) {
	var (
		req  *http.Request
		resp *http.Response
	)

	req, err = http.NewRequest("GET", AzureHostnameEndpoint, nil)
	if err != nil {
		return
	}

	req.Header.Add("Metadata", "true")

	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil {
		return
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return hostname, fmt.Errorf("failed to fetch hostname from metadata service: %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

// ExternalIPs provides any external addresses assigned to the instance
func (a *Azure) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}
