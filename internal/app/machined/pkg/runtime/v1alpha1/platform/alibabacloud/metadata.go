// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package alibabacloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/siderolabs/gen/xslices"
)

// MetadataConfig represents a metadata Alibabacloud instance.
type MetadataConfig struct {
	Hostname     string
	InstanceID   string
	InstanceType string

	Region     string
	Zone       string
	DNSServers []string
	NTPServers []string
	Tags       map[string]string

	PublicIPv4       string
	PrimaryInterface *InterfaceConfig
}

// InterfaceConfig holds the IMDS metadata for a single network interface.
type InterfaceConfig struct {
	MAC          string
	PrivateIPv4s []string
	IPv6s        []string
}

//nolint:gocyclo
func (a *Alibabacloud) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	client := newMetadataClient()

	getMetadataKey := client.getMetadataKey

	var (
		metadata MetadataConfig
		err      error
	)

	if metadata.Hostname, err = getMetadataKey(ctx, "hostname"); err != nil {
		return nil, err
	}

	if metadata.Region, err = getMetadataKey(ctx, "region-id"); err != nil {
		return nil, err
	}

	if metadata.Zone, err = getMetadataKey(ctx, "zone-id"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey(ctx, "instance-id"); err != nil {
		return nil, err
	}

	if metadata.InstanceType, err = getMetadataKey(ctx, "instance/instance-type"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4, err = getMetadataKey(ctx, "public-ipv4"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4 == "" {
		if metadata.PublicIPv4, err = getMetadataKey(ctx, "eipv4"); err != nil {
			return nil, err
		}
	}

	dnsServers, err := getMetadataKey(ctx, "dns-conf/nameservers")
	if err != nil {
		return nil, err
	}

	metadata.DNSServers = strings.Fields(dnsServers)

	ntpServers, err := getMetadataKey(ctx, "ntp-conf/ntp-servers")
	if err != nil {
		return nil, err
	}

	metadata.NTPServers = xslices.CopyN(strings.Fields(ntpServers), 5)

	if tags, err := getMetadataKey(ctx, "tags/instance/"); err == nil {
		metadata.Tags = make(map[string]string)

		for key := range strings.FieldsSeq(tags) {
			key = strings.TrimSuffix(key, "/")
			if key == "" {
				continue
			}

			if value, err := getMetadataKey(ctx, "tags/instance/"+key); err == nil {
				metadata.Tags[key] = value
			}
		}
	}

	if metadata.PrimaryInterface, err = a.getPrimaryInterface(ctx, getMetadataKey); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (a *Alibabacloud) getPrimaryInterface(ctx context.Context, getMetadataKey func(context.Context, string) (string, error)) (*InterfaceConfig, error) {
	primaryMAC, err := getMetadataKey(ctx, "mac")
	if err != nil {
		return nil, err
	}

	primaryMAC = strings.TrimSpace(primaryMAC)
	if primaryMAC == "" {
		return nil, nil
	}

	iface := &InterfaceConfig{
		MAC: primaryMAC,
	}

	if iface.PrivateIPv4s, err = fetchAddressList(ctx, getMetadataKey, fmt.Sprintf("network/interfaces/macs/%s/private-ipv4s", primaryMAC)); err != nil {
		return nil, err
	}

	if iface.IPv6s, err = fetchAddressList(ctx, getMetadataKey, fmt.Sprintf("network/interfaces/macs/%s/ipv6s", primaryMAC)); err != nil {
		return nil, err
	}

	return iface, nil
}

// fetchAddressList reads a JSON array of addresses from IMDS.
func fetchAddressList(ctx context.Context, getMetadataKey func(context.Context, string) (string, error), path string) ([]string, error) {
	raw, err := getMetadataKey(ctx, path)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var addrs []string
	if err = json.Unmarshal([]byte(raw), &addrs); err != nil {
		return nil, fmt.Errorf("failed to parse addresses from %q: %w", path, err)
	}

	return addrs, nil
}
