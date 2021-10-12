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
	"os"
	"path/filepath"
	"regexp"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
	// AzureHostnameEndpoint is the local endpoint for the hostname.
	AzureHostnameEndpoint = "http://169.254.169.254/metadata/instance/compute/name?api-version=2021-05-01&format=text"
	// AzureInterfacesEndpoint is the local endpoint to get external IPs.
	AzureInterfacesEndpoint = "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-05-01"

	mnt = "/mnt"
)

// NetworkConfig holds network interface meta config.
type NetworkConfig struct {
	IPv4 struct {
		IPAddresses []IPAddresses `json:"ipAddress"`
	} `json:"ipv4"`
	IPv6 struct {
		IPAddresses []IPAddresses `json:"ipAddress"`
	} `json:"ipv6"`
}

// IPAddresses holds public/private IPs.
type IPAddresses struct {
	PrivateIPAddress string `json:"privateIpAddress"`
	PublicIPAddress  string `json:"publicIpAddress"`
}

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
	log.Printf("fetching machine config from ovf-env.xml")

	// Custom data is not available in IMDS, so trying to find it on CDROM.
	config, err := a.configFromCD()
	if err != nil {
		log.Printf("fetching machine config from cdrom failed, err: %s", err.Error())
	}

	if err := linuxAgent(ctx); err != nil {
		log.Printf("failed to update instance status, err: %s", err.Error())

		return nil, errors.ErrNoConfigSource
	}

	if len(config) == 0 {
		return nil, errors.ErrNoConfigSource
	}

	return config, nil
}

// Mode implements the platform.Platform interface.
func (a *Azure) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the platform.Platform interface.
func (a *Azure) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", AzureHostnameEndpoint)

	host, err := download.Download(ctx, AzureHostnameEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}),
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	return host, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (a *Azure) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching externalIP from: %q", AzureInterfacesEndpoint)

	body, err := download.Download(ctx, AzureInterfacesEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}),
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return nil, err
	}

	addrs, err = a.GetPublicIPs(body)
	if err != nil {
		return nil, err
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (a *Azure) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0,115200n8"),
		procfs.NewParameter("earlyprintk").Append("ttyS0,115200"),
		procfs.NewParameter("rootdelay").Append("300"),
	}
}

// configFromCD handles looking for devices and trying to mount/fetch xml to get the custom data.
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

// GetPublicIPs parced metadata interface response.
func (a *Azure) GetPublicIPs(interfaces []byte) (addrs []net.IP, err error) {
	var interfaceAddresses []NetworkConfig

	if err = json.Unmarshal(interfaces, &interfaceAddresses); err != nil {
		return nil, errors.ErrNoExternalIPs
	}

	for _, iface := range interfaceAddresses {
		for _, ipv4addr := range iface.IPv4.IPAddresses {
			if ip := net.ParseIP(ipv4addr.PublicIPAddress); ip != nil {
				addrs = append(addrs, ip)
			}
		}

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			if ip := net.ParseIP(ipv6addr.PublicIPAddress); ip != nil {
				addrs = append(addrs, ip)
			}
		}
	}

	return addrs, nil
}
