// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package azure contains the Azure implementation of the [platform.Platform].
package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	stderrors "errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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
func (a *Azure) ParseMetadata(metadata *ComputeMetadata, interfaceAddresses []NetworkConfig, host []byte) (*runtime.PlatformNetworkConfig, error) {
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

	var publicIPs []string

	// external IP
	for _, iface := range interfaceAddresses {
		for _, ipv4addr := range iface.IPv4.IPAddresses {
			publicIPs = append(publicIPs, ipv4addr.PublicIPAddress)
		}

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			publicIPs = append(publicIPs, ipv6addr.PublicIPAddress)
		}
	}

	// DHCP6 for enabled interfaces
	for idx, iface := range interfaceAddresses {
		ipv6 := false

		for _, ipv6addr := range iface.IPv6.IPAddresses {
			ipv6 = ipv6addr.PublicIPAddress != "" || ipv6addr.PrivateIPAddress != ""
		}

		if ipv6 {
			ifname := fmt.Sprintf("eth%d", idx)

			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  ifname,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: 2 * network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})

			// If accept_ra is not set, use the default gateway.
			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     netip.MustParseAddr("fe80::1234:5678:9abc"),
				OutLinkName: ifname,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      nethelpers.FamilyInet6,
				Priority:    4 * network.DefaultRouteMetric,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	zone := metadata.FaultDomain
	if metadata.Zone != "" {
		zone = fmt.Sprintf("%s-%s", metadata.Location, metadata.Zone)
	}

	providerID, err := convertResourceGroupNameToLower(metadata.ResourceID)
	if err != nil {
		return nil, err
	}

	networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.Links = append(networkConfig.Links,
		network.LinkSpecSpec{
			Name:        "eth0",
			Up:          true,
			MTU:         1400,
			ConfigLayer: network.ConfigPlatform,
		},
	)

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.OSProfile.ComputerName,
		Region:       strings.ToLower(metadata.Location),
		Zone:         strings.ToLower(zone),
		InstanceType: metadata.VMSize,
		InstanceID:   metadata.ResourceID,
		ProviderID:   fmt.Sprintf("azure://%s", providerID),
		Spot:         metadata.EvictionPolicy != "",
	}

	return &networkConfig, nil
}

// ParseLoadBalancerIP parses Azure LoadBalancer metadata into the platform external ip list.
func (a *Azure) ParseLoadBalancerIP(lbConfig LoadBalancerMetadata, exIP []netip.Addr) ([]netip.Addr, error) {
	lbAddresses := exIP

	for _, addr := range lbConfig.LoadBalancer.PublicIPAddresses {
		ipaddr := addr.FrontendIPAddress

		if i := strings.IndexByte(ipaddr, ']'); i != -1 {
			ipaddr = strings.TrimPrefix(ipaddr[:i], "[")
		}

		if ip, err := netip.ParseAddr(ipaddr); err == nil {
			lbAddresses = append(lbAddresses, ip)
		}
	}

	return lbAddresses, nil
}

// Configuration implements the platform.Platform interface.
func (a *Azure) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	defer func() {
		if err := netutils.Wait(ctx, r); err != nil {
			log.Printf("failed to wait for network, err: %s", err)
		}

		if err := linuxAgent(ctx); err != nil {
			log.Printf("failed to update instance status, err: %s", err)
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
func (a *Azure) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0,115200n8"),
		procfs.NewParameter("earlyprintk").Append("ttyS0,115200"),
		procfs.NewParameter("rootdelay").Append("300"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
		// disable 'kexec' as Azure VMs sometimes are stuck on kexec, and normal soft reboot
		// doesn't take much longer on VMs
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// configFromCD handles looking for devices and trying to mount/fetch xml to get the custom data.
//
//nolint:gocyclo
func (a *Azure) configFromCD() ([]byte, error) {
	devList, err := os.ReadDir("/dev")
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

			ovfEnvFile, err := os.ReadFile(filepath.Join(mnt, "ovf-env.xml"))
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
func (a *Azure) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	metadata, apiVersion, err := a.getMetadata(ctx)
	if err != nil {
		return err
	}

	interfacesEndpoint := fmt.Sprintf(AzureInterfacesEndpoint, apiVersion)

	log.Printf("fetching network config from %q", interfacesEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, interfacesEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}))
	if err != nil {
		return fmt.Errorf("failed to fetch network config from metadata service: %w", err)
	}

	var interfaceAddresses []NetworkConfig

	if err = json.Unmarshal(metadataNetworkConfig, &interfaceAddresses); err != nil {
		return err
	}

	networkConfig, err := a.ParseMetadata(metadata, interfaceAddresses, []byte(metadata.OSProfile.ComputerName))
	if err != nil {
		return fmt.Errorf("failed to parse network metadata: %w", err)
	}

	loadbalancerEndpoint := fmt.Sprintf(AzureLoadbalancerEndpoint, apiVersion)

	log.Printf("fetching load balancer metadata from: %q", loadbalancerEndpoint)

	var loadBalancerAddresses LoadBalancerMetadata

	lbConfig, err := download.Download(ctx, loadbalancerEndpoint,
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

// convertResourceGroupNameToLower converts the resource group name in the resource ID to be lowered.
// https://github.com/kubernetes-sigs/cloud-provider-azure/blob/4192b264611aebef8070505dd56680a862acfbbf/pkg/provider/azure_wrap.go#L91
func convertResourceGroupNameToLower(resourceID string) (string, error) {
	// https://github.com/kubernetes-sigs/cloud-provider-azure/blob/4192b264611aebef8070505dd56680a862acfbbf/pkg/provider/azure_wrap.go#L37
	azureResourceGroupNameRE := regexp.MustCompile(`.*/subscriptions/(?:.*)/resourceGroups/(.+)/providers/(?:.*)`)

	matches := azureResourceGroupNameRE.FindStringSubmatch(resourceID)
	if len(matches) != 2 {
		return "", fmt.Errorf("%q isn't in Azure resource ID format %q", resourceID, azureResourceGroupNameRE.String())
	}

	resourceGroup := matches[1]

	return strings.Replace(resourceID, resourceGroup, strings.ToLower(resourceGroup), 1), nil
}
