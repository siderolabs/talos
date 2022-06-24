// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	stderrors "errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
	// AzureHostnameEndpoint is the local endpoint for the hostname.
	AzureHostnameEndpoint = "http://169.254.169.254/metadata/instance/compute/osProfile/computerName?api-version=2021-12-13&format=text"
	// AzureInterfacesEndpoint is the local endpoint to get external IPs.
	AzureInterfacesEndpoint = "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-12-13&format=json"
	// AzureLoadbalancerEndpoint is the local endpoint for load balancer config.
	AzureLoadbalancerEndpoint = "http://169.254.169.254/metadata/loadbalancer?api-version=2021-05-01&format=json"

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

// LoadBalancerMetadata represents load balancer metadata in IMDS.
type LoadBalancerMetadata struct {
	LoadBalancer struct {
		PublicIPAddresses []struct {
			FrontendIPAddress string `json:"frontendIpAddress,omitempty"`
			PrivateIPAddress  string `json:"privateIpAddress,omitempty"`
		} `json:"publicIpAddresses,omitempty"`
	} `json:"loadbalancer,omitempty"`
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

// ParseMetadata parses Azure network metadata into the platform network config.
//
//nolint:gocyclo
func (a *Azure) ParseMetadata(interfaceAddresses []NetworkConfig, host []byte) (*runtime.PlatformNetworkConfig, error) {
	var networkConfig runtime.PlatformNetworkConfig

	// hostname
	if len(host) > 0 {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(string(host)); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	// external IP
	for _, iface := range interfaceAddresses {
		for _, ipv4addr := range iface.IPv4.IPAddresses {
			if ip, err := netaddr.ParseIP(ipv4addr.PublicIPAddress); err == nil {
				networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
			}
		}

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			if ip, err := netaddr.ParseIP(ipv6addr.PublicIPAddress); err == nil {
				networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
			}
		}
	}

	// DHCP6 for enabled interfaces
	for idx, iface := range interfaceAddresses {
		ipv6 := false

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			ipv6 = ipv6addr.PublicIPAddress != "" || ipv6addr.PrivateIPAddress != ""
		}

		if ipv6 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  fmt.Sprintf("eth%d", idx),
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: 1024,
				},
			})
		}
	}

	return &networkConfig, nil
}

// ParseLoadBalancerIP parses Azure LoadBalancer metadata into the platform external ip list.
func (a *Azure) ParseLoadBalancerIP(lbConfig LoadBalancerMetadata, exIP []netaddr.IP) ([]netaddr.IP, error) {
	lbAddresses := exIP

	for _, addr := range lbConfig.LoadBalancer.PublicIPAddresses {
		ipaddr := addr.FrontendIPAddress

		if i := strings.IndexByte(ipaddr, ']'); i != -1 {
			ipaddr = strings.TrimPrefix(ipaddr[:i], "[")
		}

		if ip, err := netaddr.ParseIP(ipaddr); err == nil {
			lbAddresses = append(lbAddresses, ip)
		}
	}

	return lbAddresses, nil
}

// Configuration implements the platform.Platform interface.
func (a *Azure) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	defer func() {
		if err := linuxAgent(ctx); err != nil {
			log.Printf("failed to update instance status, err: %s", err.Error())
		}
	}()

	log.Printf("fetching machine config from ovf-env.xml")

	// Custom data is not available in IMDS, so trying to find it on CDROM.
	return a.configFromCD()
}

// Mode implements the platform.Platform interface.
func (a *Azure) Mode() runtime.Mode {
	return runtime.ModeCloud
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
//nolint:gocyclo
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

			if len(ovfEnvData.CustomData) > 0 {
				b64CustomData, err := base64.StdEncoding.DecodeString(ovfEnvData.CustomData)
				if err != nil {
					return nil, err
				}

				return b64CustomData, nil
			}

			return nil, errors.ErrNoConfigSource
		}
	}

	return nil, errors.ErrNoConfigSource
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (a *Azure) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching network config from %q", AzureInterfacesEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, AzureInterfacesEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}))
	if err != nil {
		return fmt.Errorf("failed to fetch network config from metadata service: %w", err)
	}

	var interfaceAddresses []NetworkConfig

	if err = json.Unmarshal(metadataNetworkConfig, &interfaceAddresses); err != nil {
		return err
	}

	log.Printf("fetching hostname from: %q", AzureHostnameEndpoint)

	host, err := download.Download(ctx, AzureHostnameEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}),
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return fmt.Errorf("failed to fetch hostname from metadata service: %w", err)
	}

	networkConfig, err := a.ParseMetadata(interfaceAddresses, host)
	if err != nil {
		return fmt.Errorf("failed to parse network metadata: %w", err)
	}

	log.Printf("fetching load balancer metadata from: %q", AzureLoadbalancerEndpoint)

	var loadBalancerAddresses LoadBalancerMetadata

	lbConfig, err := download.Download(ctx, AzureLoadbalancerEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}),
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil && !stderrors.Is(err, errors.ErrNoConfigSource) {
		log.Printf("failed to fetch load balancer config from metadata service: %s", err)

		lbConfig = nil
	}

	if len(lbConfig) > 0 {
		if err = json.Unmarshal(lbConfig, &loadBalancerAddresses); err != nil {
			return fmt.Errorf("failed to parse loadbalancer metadata: %w", err)
		}

		networkConfig.ExternalIPs, err = a.ParseLoadBalancerIP(loadBalancerAddresses, networkConfig.ExternalIPs)
		if err != nil {
			return fmt.Errorf("failed to define externalIPs: %w", err)
		}
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
