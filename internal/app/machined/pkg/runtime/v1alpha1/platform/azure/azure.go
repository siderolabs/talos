// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// AzureUserDataEndpoint is the local endpoint for the config.
	// By specifying format=text and drilling down to the actual key we care about
	// we get a base64 encoded config response.
	AzureUserDataEndpoint = "http://169.254.169.254/metadata/instance/compute/customData?api-version=2019-06-01&format=text"
	// AzureHostnameEndpoint is the local endpoint for the hostname.
	AzureHostnameEndpoint = "http://169.254.169.254/metadata/instance/compute/name?api-version=2019-06-01&format=text"
	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
	// AzureInterfacesEndpoint is the local endpoint to get external IPs.
	AzureInterfacesEndpoint = "http://169.254.169.254/metadata/instance/network/interface?api-version=2019-06-01"

	mnt = "/mnt"
)

// Azure is the concrete type that implements the platform.Platform interface.
type Azure struct{}

// ovfXML is a simple struct to help us fish custom data out from the ovf-env.xml file.
type ovfXML struct {
	XMLName    xml.Name `xml:"Environment"`
	CustomData string   `xml:"ProvisioningSection>LinuxProvisioningConfigurationSet>CustomData"`
}

// Name implements the platform.Platform interface.
func (a *Azure) Name() string {
	return "azure"
}

// Configuration implements the platform.Platform interface.
func (a *Azure) Configuration(ctx context.Context) ([]byte, error) {
	// TODO: support ErrNoConfigSource, requires handling of both CD-ROM & user-data sources
	//       requires splitting `linuxAgent` into separate platform task which is called when node is up (or close to that)
	// attempt to download from metadata endpoint
	// disabled by default
	log.Printf("fetching machine config from: %q", AzureUserDataEndpoint)

	config, err := download.Download(ctx, AzureUserDataEndpoint, download.WithHeaders(map[string]string{"Metadata": "true"}), download.WithFormat("base64"))
	if err != nil {
		fmt.Printf("metadata download failed, falling back to ovf-env.xml file. err: %s", err.Error())
	}

	// fall back to cdrom read b/c we failed to pull userdata from metadata server
	if len(config) == 0 {
		log.Printf("fetching machine config from: ovf-env.xml")

		config, err = a.configFromCD()
		if err != nil {
			return nil, err
		}
	}

	if err := linuxAgent(ctx); err != nil {
		return nil, err
	}

	return config, nil
}

// Hostname implements the platform.Platform interface.
func (a *Azure) Hostname(ctx context.Context) (hostname []byte, err error) {
	var (
		req  *http.Request
		resp *http.Response
	)

	req, err = http.NewRequestWithContext(ctx, "GET", AzureHostnameEndpoint, nil)
	if err != nil {
		return
	}

	req.Header.Add("Metadata", "true")

	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil {
		return
	}

	//nolint:errcheck
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
func (a *Azure) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, "GET", AzureInterfacesEndpoint, nil); err != nil {
		return
	}

	req.Header.Add("Metadata", "true")

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

// configFromCD handles looking for devices and trying to mount/fetch xml to get the userdata.
func (a *Azure) configFromCD() ([]byte, error) {
	devList, err := ioutil.ReadDir("/dev")
	if err != nil {
		return nil, err
	}

	diskRegex := regexp.MustCompile("(sr[0-9]|hd[c-z]|cdrom[0-9]|cd[0-9])")

	for _, dev := range devList {
		if diskRegex.MatchString(dev.Name()) {
			fmt.Printf("found matching device. checking for ovf-env.xml: %s\n", dev.Name())

			// Mount and slurp xml from disk
			if err = unix.Mount(filepath.Join("/dev", dev.Name()), mnt, "udf", unix.MS_RDONLY, ""); err != nil {
				fmt.Printf("unable to mount %s, possibly not udf: %s", dev.Name(), err.Error())

				continue
			}

			ovfEnvFile, err := ioutil.ReadFile(filepath.Join(mnt, "ovf-env.xml"))
			if err != nil {
				// Device mount worked, but it wasn't the "CD" that contains the xml file
				if os.IsNotExist(err) {
					continue
				}

				return nil, fmt.Errorf("failed to read config: %w", err)
			}

			if err = unix.Unmount(mnt, 0); err != nil {
				return nil, fmt.Errorf("failed to unmount: %w", err)
			}

			// Unmarshall xml we slurped
			ovfEnvData := ovfXML{}

			err = xml.Unmarshal(ovfEnvFile, &ovfEnvData)
			if err != nil {
				return nil, err
			}

			b64CustomData, err := base64.StdEncoding.DecodeString(ovfEnvData.CustomData)
			if err != nil {
				return nil, err
			}

			return b64CustomData, nil
		}
	}

	return nil, fmt.Errorf("no devices seemed to contain ovf-env.xml for pulling machine config")
}
