// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// MetadataConfig represents a metadata AWS instance.
type MetadataConfig struct {
	Hostname          string            `json:"hostname,omitempty"`
	InstanceID        string            `json:"instance-id,omitempty"`
	InstanceType      string            `json:"instance-type,omitempty"`
	InstanceLifeCycle string            `json:"instance-life-cycle,omitempty"`
	PublicIPv4        string            `json:"public-ipv4,omitempty"`
	InternalDNS       string            `json:"local-hostname,omitempty"`
	ExternalDNS       string            `json:"public-hostname,omitempty"`
	Region            string            `json:"region,omitempty"`
	Zone              string            `json:"zone,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`

	// PrimaryInterface holds metadata for the primary network interface.
	//
	// Talos only supports a single NIC on AWS, so secondary interfaces are ignored.
	PrimaryInterface *InterfaceConfig `json:"primary-interface,omitempty"`
}

// InterfaceConfig holds the IMDS metadata for a single network interface.
type InterfaceConfig struct {
	MAC          string   `json:"mac,omitempty"`
	DeviceNumber string   `json:"device-number,omitempty"`
	LocalIPv4s   []string `json:"local-ipv4s,omitempty"`
	IPv6s        []string `json:"ipv6s,omitempty"`
}

//nolint:gocyclo
func (a *AWS) getMetadata(ctx context.Context, client *imds.Client) (*MetadataConfig, error) {
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
	getMetadataKey := func(key string) (string, error) {
		resp, err := client.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: key,
		})
		if err != nil {
			if isNotFoundError(err) {
				return "", nil
			}

			return "", fmt.Errorf("failed to fetch %q from IMDS: %w", key, err)
		}

		defer resp.Content.Close() //nolint:errcheck

		v, err := io.ReadAll(resp.Content)

		return string(v), err
	}

	var (
		metadata MetadataConfig
		err      error
	)
	if metadata.Hostname, err = getMetadataKey("hostname"); err != nil {
		return nil, err
	}

	if metadata.InstanceType, err = getMetadataKey("instance-type"); err != nil {
		return nil, err
	}

	if metadata.InstanceLifeCycle, err = getMetadataKey("instance-life-cycle"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey("instance-id"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4, err = getMetadataKey("public-ipv4"); err != nil {
		return nil, err
	}

	if metadata.InternalDNS, err = getMetadataKey("local-hostname"); err != nil {
		return nil, err
	}

	if metadata.ExternalDNS, err = getMetadataKey("public-hostname"); err != nil {
		return nil, err
	}

	if metadata.Region, err = getMetadataKey("placement/region"); err != nil {
		return nil, err
	}

	if metadata.Zone, err = getMetadataKey("placement/availability-zone"); err != nil {
		return nil, err
	}

	if tags, err := getMetadataKey("tags/instance"); err == nil {
		metadata.Tags = make(map[string]string)

		for key := range strings.FieldsSeq(tags) {
			if value, err := getMetadataKey("tags/instance/" + key); err == nil {
				metadata.Tags[key] = value
			}
		}
	}

	if metadata.PrimaryInterface, err = a.getPrimaryInterface(getMetadataKey); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// getPrimaryInterface returns metadata for the primary network interface.
//
// IMDS lists every NIC under network/interfaces/macs/, but Talos only supports
// the primary NIC. Pick the entry with device-number=0 — AWS guarantees this is
// the primary — and fall back to the first listed MAC if the field is missing.
//
//nolint:gocyclo
func (a *AWS) getPrimaryInterface(getMetadataKey func(string) (string, error)) (*InterfaceConfig, error) {
	macsList, err := getMetadataKey("network/interfaces/macs/")
	if err != nil {
		return nil, err
	}

	var macs []string

	for line := range strings.Lines(macsList) {
		mac := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if mac == "" {
			continue
		}

		macs = append(macs, mac)
	}

	if len(macs) == 0 {
		return nil, nil
	}

	primaryMAC := macs[0]

	for _, mac := range macs {
		deviceNumber, err := getMetadataKey(fmt.Sprintf("network/interfaces/macs/%s/device-number", mac))
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(deviceNumber) == "0" {
			primaryMAC = mac

			break
		}
	}

	iface := &InterfaceConfig{
		MAC: primaryMAC,
	}

	if iface.DeviceNumber, err = getMetadataKey(fmt.Sprintf("network/interfaces/macs/%s/device-number", primaryMAC)); err != nil {
		return nil, err
	}

	iface.DeviceNumber = strings.TrimSpace(iface.DeviceNumber)

	if iface.LocalIPv4s, err = fetchAddressList(getMetadataKey, fmt.Sprintf("network/interfaces/macs/%s/local-ipv4s", primaryMAC)); err != nil {
		return nil, err
	}

	if iface.IPv6s, err = fetchAddressList(getMetadataKey, fmt.Sprintf("network/interfaces/macs/%s/ipv6s", primaryMAC)); err != nil {
		return nil, err
	}

	return iface, nil
}

// fetchAddressList reads a newline-separated list of addresses from IMDS.
func fetchAddressList(getMetadataKey func(string) (string, error), path string) ([]string, error) {
	raw, err := getMetadataKey(path)
	if err != nil {
		return nil, err
	}

	var addrs []string

	for line := range strings.Lines(raw) {
		addr := strings.TrimSpace(line)
		if addr == "" {
			continue
		}

		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func isNotFoundError(err error) bool {
	var awsErr *smithyhttp.ResponseError
	if errors.As(err, &awsErr) {
		return awsErr.HTTPStatusCode() == http.StatusNotFound
	}

	return false
}
