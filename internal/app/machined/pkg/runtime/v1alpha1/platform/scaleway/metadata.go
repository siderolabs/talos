// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/siderolabs/go-retry/retry"

	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// ScalewayMetadataEndpoint is the local Scaleway IPv4 metadata endpoint.
	ScalewayMetadataEndpoint = "http://169.254.42.42/conf?format=json"
	// ScalewayMetadataEndpointIPv6 is the local Scaleway IPv6 metadata endpoint.
	ScalewayMetadataEndpointIPv6 = "http://[fd00:42::42]/conf?format=json"
	// ScalewayUserDataEndpoint is the local Scaleway IPv4 endpoint for the config.
	ScalewayUserDataEndpoint = "http://169.254.42.42/user_data/cloud-init"
	// ScalewayUserDataEndpointIPv6 is the local Scaleway IPv6 endpoint for the config.
	ScalewayUserDataEndpointIPv6 = "http://[fd00:42::42]/user_data/cloud-init"

	// endpointAttemptTimeout bounds a single attempt against one address family. An instance
	// that only has one of them fails over to the other after this long, rather than spending
	// the whole retry budget on an endpoint it can never reach.
	endpointAttemptTimeout = 5 * time.Second
	endpointRetryTimeout   = 3 * time.Minute
)

// metadataRoute is the host route to the metadata service, needed to reach it before any
// address is configured on the link.
var metadataRoute = netip.MustParsePrefix("169.254.42.42/32")

type endpointFamily uint32

const (
	endpointFamilyIPv4 endpointFamily = iota
	endpointFamilyIPv6
)

// downloadAlternating retries against the IPv4 and IPv6 endpoints in turn.
func (s *Scaleway) downloadAlternating(ctx context.Context, ipv4Endpoint, ipv6Endpoint string, options ...download.Option) (data []byte, err error) {
	endpoints := [...]string{ipv4Endpoint, ipv6Endpoint}

	family := endpointFamilyIPv4
	if s.lastSuccessfulIPv6.Load() {
		family = endpointFamilyIPv6
	}

	attemptOptions := append([]download.Option{}, options...)
	attemptOptions = append(attemptOptions, download.WithTimeout(endpointAttemptTimeout))

	err = retry.Exponential(
		endpointRetryTimeout,
		retry.WithUnits(time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		endpoint := endpoints[family]

		data, err = download.Download(ctx, endpoint, attemptOptions...)
		if err == nil {
			s.lastSuccessfulIPv6.Store(family == endpointFamilyIPv6)

			return nil
		}

		if errors.Is(err, platformerrors.ErrNoConfigSource) {
			return platformerrors.ErrNoConfigSource
		}

		family = (family + 1) % endpointFamily(len(endpoints))

		// A timed-out download can return only the retry timeout, so retain the
		// endpoint here as well to keep failures from both families distinct.
		return retry.ExpectedError(fmt.Errorf("fetching %q: %w", endpoint, err))
	})
	if errors.Is(err, platformerrors.ErrNoConfigSource) {
		return nil, platformerrors.ErrNoConfigSource
	}

	return data, err
}

type ipv6Address struct {
	address netip.Prefix
	gateway netip.Addr
}

func parseIPv6MetadataIPs(metadataIPs []instance.MetadataIP) []ipv6Address {
	var addresses []ipv6Address

	for _, metadataIP := range metadataIPs {
		address, gateway, err := parseIPv6MetadataIP(metadataIP)
		if err != nil {
			log.Printf("skipping malformed Scaleway public IPv6 entry for %q: %v", metadataIP.Address, err)

			continue
		}

		addresses = append(addresses, ipv6Address{address: address, gateway: gateway})
	}

	return addresses
}

func parseIPv6MetadataIP(metadataIP instance.MetadataIP) (netip.Prefix, netip.Addr, error) {
	if metadataIP.Address == "" {
		return netip.Prefix{}, netip.Addr{}, errors.New("address is empty")
	}

	if metadataIP.Netmask == "" {
		return netip.Prefix{}, netip.Addr{}, errors.New("netmask is empty")
	}

	ip, err := netip.ParseAddr(metadataIP.Address)
	if err != nil {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("invalid address: %w", err)
	}

	if !ip.Unmap().Is6() {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("address has the wrong family: %q", metadataIP.Address)
	}

	prefixBits, err := parseIPv6Netmask(metadataIP.Netmask)
	if err != nil {
		return netip.Prefix{}, netip.Addr{}, err
	}

	addr := netip.PrefixFrom(ip, prefixBits)
	if !addr.IsValid() {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("invalid address prefix: %q/%d", metadataIP.Address, prefixBits)
	}

	if metadataIP.Gateway == "" {
		return addr, netip.Addr{}, nil
	}

	gw, err := netip.ParseAddr(metadataIP.Gateway)
	if err != nil {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("invalid gateway: %w", err)
	}

	if !gw.Unmap().Is6() {
		return netip.Prefix{}, netip.Addr{}, fmt.Errorf("gateway has the wrong family: %q", metadataIP.Gateway)
	}

	return addr, gw, nil
}

func parseIPv6Netmask(mask string) (int, error) {
	const bits = 128

	prefixBits, err := strconv.Atoi(mask)
	if err == nil {
		if prefixBits < 0 || prefixBits > bits {
			return 0, fmt.Errorf("invalid netmask %q", mask)
		}

		return prefixBits, nil
	}

	maskAddr, err := netip.ParseAddr(mask)
	if err != nil || !maskAddr.Unmap().Is6() {
		return 0, fmt.Errorf("invalid netmask %q", mask)
	}

	maskBytes := maskAddr.As16()

	prefixBits, maskBits := net.IPMask(maskBytes[:]).Size()
	if maskBits != bits {
		return 0, fmt.Errorf("invalid netmask %q", mask)
	}

	return prefixBits, nil
}

func (s *Scaleway) getMetadata(ctx context.Context) (*instance.Metadata, error) {
	data, err := s.downloadAlternating(ctx, ScalewayMetadataEndpoint, ScalewayMetadataEndpointIPv6)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var metadata instance.Metadata
	if err = json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}
