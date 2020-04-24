// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/download"
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
	// AzureInterfacesEndpoint is the local endpoint to get external IPs.
	AzureInterfacesEndpoint = "http://169.254.169.254/metadata/instance/network/interface?api-version=2019-06-01"
)

// Azure is the concrete type that implements the platform.Platform interface.
type Azure struct{}

// Name implements the platform.Platform interface.
func (a *Azure) Name() string {
	return "azure"
}

// Configuration implements the platform.Platform interface.
func (a *Azure) Configuration() ([]byte, error) {
	if err := linuxAgent(); err != nil {
		return nil, err
	}

	return download.Download(AzureUserDataEndpoint, download.WithHeaders(map[string]string{"Metadata": "true"}), download.WithFormat("base64"))
}

// Hostname implements the platform.Platform interface.
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

// Mode implements the platform.Platform interface.
func (a *Azure) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// ExternalIPs implements the runtime.Platform interface.
func (a *Azure) ExternalIPs() (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequest("GET", AzureInterfacesEndpoint, nil); err != nil {
		return
	}

	req.Header.Add("Metadata", "true")

	client := &http.Client{}

	if resp, err = client.Do(req); err != nil {
		return
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return addrs, fmt.Errorf("failed to retrieve external addresses for instance")
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return addrs, err
	}

	type IPAddress struct {
		PrivateIPAddress string `json:"privateIpAddress"`
		PublicIPAddress  string `json:"publicIpAddress"`
	}

	type interfaces []struct {
		IPv4 struct {
			IPAddresses []IPAddress `json:"ipAddress"`
		} `json:"ipv4"`
		IPv6 struct {
			IPAddresses []IPAddress `json:"ipAddress"`
		} `json:"ipv6"`
	}

	interfaceAddresses := interfaces{}
	if err = json.Unmarshal(body, &interfaceAddresses); err != nil {
		return addrs, err
	}

	for _, iface := range interfaceAddresses {
		for _, ipv4addr := range iface.IPv4.IPAddresses {
			addrs = append(addrs, net.ParseIP(ipv4addr.PublicIPAddress))
		}

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			addrs = append(addrs, net.ParseIP(ipv6addr.PublicIPAddress))
		}
	}

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (a *Azure) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0,115200n8"),
		procfs.NewParameter("earlyprintk").Append("ttyS0,115200"),
		procfs.NewParameter("rootdelay").Append("300"),
	}
}
