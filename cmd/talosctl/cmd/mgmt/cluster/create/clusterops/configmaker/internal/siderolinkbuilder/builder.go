// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolinkbuilder

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configbase "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

// New creates a new SiderolinkBuilder.
func New(ctx context.Context, wgHost string, useTLS bool) (*SiderolinkBuilder, error) {
	prefix, err := networkPrefix("")
	if err != nil {
		return nil, err
	}

	result := &SiderolinkBuilder{
		wgHost:       wgHost,
		binds:        map[uuid.UUID]netip.Addr{},
		prefix:       prefix,
		nodeIPv6Addr: prefix.Addr().Next().String(),
	}

	if useTLS {
		ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true), x509.IPAddresses([]net.IP{net.ParseIP(wgHost)}))
		if err != nil {
			return nil, err
		}

		result.apiCert = ca.CrtPEM
		result.apiKey = ca.KeyPEM
	}

	var resultErr error

	for range 10 {
		for _, d := range []struct {
			field *int
			net   string
			what  string
		}{
			{&result.wgPort, "udp", "WireGuard"},
			{&result.apiPort, "tcp", "gRPC API"},
			{&result.sinkPort, "tcp", "Event Sink"},
			{&result.logPort, "tcp", "Log Receiver"},
		} {
			var err error

			*d.field, err = getDynamicPort(ctx, d.net)
			if err != nil {
				return nil, fmt.Errorf("failed to get dynamic port for %s: %w", d.what, err)
			}
		}

		resultErr = checkPortsDontOverlap(result.wgPort, result.apiPort, result.sinkPort, result.logPort)
		if resultErr == nil {
			break
		}
	}

	if resultErr != nil {
		return nil, fmt.Errorf("failed to get non-overlapping dynamic ports in 10 attempts: %w", resultErr)
	}

	return result, nil
}

// SiderolinkBuilder is responsible for building Siderolink configurations.
type SiderolinkBuilder struct {
	wgHost string

	binds        map[uuid.UUID]netip.Addr
	prefix       netip.Prefix
	nodeIPv6Addr string
	wgPort       int
	apiPort      int
	sinkPort     int
	logPort      int

	apiCert []byte
	apiKey  []byte
}

// DefineIPv6ForUUID defines an IPv6 address for a given UUID. It is safe to call this method on a nil pointer.
func (slb *SiderolinkBuilder) DefineIPv6ForUUID(id uuid.UUID) error {
	if slb == nil {
		return nil
	}

	result, err := generateRandomNodeAddr(slb.prefix)
	if err != nil {
		return err
	}

	slb.binds[id] = result.Addr()

	return nil
}

// SiderolinkRequest returns a SiderolinkRequest based on the current state of the builder.
// It is safe to call this method on a nil pointer.
func (slb *SiderolinkBuilder) SiderolinkRequest() provision.SiderolinkRequest {
	if slb == nil {
		return provision.SiderolinkRequest{}
	}

	return provision.SiderolinkRequest{
		WireguardEndpoint: net.JoinHostPort(slb.wgHost, strconv.Itoa(slb.wgPort)),
		APIEndpoint:       ":" + strconv.Itoa(slb.apiPort),
		APICertificate:    slb.apiCert,
		APIKey:            slb.apiKey,
		SinkEndpoint:      ":" + strconv.Itoa(slb.sinkPort),
		LogEndpoint:       ":" + strconv.Itoa(slb.logPort),
		SiderolinkBind: maps.ToSlice(slb.binds, func(k uuid.UUID, v netip.Addr) provision.SiderolinkBind {
			return provision.SiderolinkBind{
				UUID: k,
				Addr: v,
			}
		}),
	}
}

// ConfigPatches returns the config patches for the current builder.
func (slb *SiderolinkBuilder) ConfigPatches(tunnel bool) []configpatcher.Patch {
	cfg := slb.ConfigDocument(tunnel)
	if cfg == nil {
		return nil
	}

	return []configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)}
}

// ConfigDocument returns the config document for the current builder.
func (slb *SiderolinkBuilder) ConfigDocument(tunnel bool) config.Provider {
	if slb == nil {
		return nil
	}

	scheme := "grpc://"

	if slb.apiCert != nil {
		scheme = "https://"
	}

	apiLink := scheme + net.JoinHostPort(slb.wgHost, strconv.Itoa(slb.apiPort)) + "?jointoken=foo"

	if tunnel {
		apiLink += "&grpc_tunnel=true"
	}

	apiURL, err := url.Parse(apiLink)
	if err != nil {
		panic(fmt.Sprintf("failed to parse API URL: %s", err))
	}

	sdlConfig := siderolink.NewConfigV1Alpha1()
	sdlConfig.APIUrlConfig.URL = apiURL

	eventsConfig := runtime.NewEventSinkV1Alpha1()
	eventsConfig.Endpoint = net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.sinkPort))

	logURL, err := url.Parse("tcp://" + net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.logPort)))
	if err != nil {
		panic(fmt.Sprintf("failed to parse log URL: %s", err))
	}

	logConfig := runtime.NewKmsgLogV1Alpha1()
	logConfig.MetaName = "siderolink"
	logConfig.KmsgLogURL.URL = logURL

	documents := []configbase.Document{
		sdlConfig,
		eventsConfig,
		logConfig,
	}

	if slb.apiCert != nil {
		trustedRootsConfig := security.NewTrustedRootsConfigV1Alpha1()
		trustedRootsConfig.MetaName = "siderolink-ca"
		trustedRootsConfig.Certificates = string(slb.apiCert)

		documents = append(documents, trustedRootsConfig)
	}

	ctr, err := container.New(documents...)
	if err != nil {
		panic(fmt.Sprintf("failed to create container for Siderolink config: %s", err))
	}

	return ctr
}

// SetKernelArgs sets the kernel arguments for the current builder. It is safe to call this method on a nil pointer.
func (slb *SiderolinkBuilder) SetKernelArgs(extraKernelArgs *procfs.Cmdline, tunnel bool) error {
	switch {
	case slb == nil:
		return nil
	case extraKernelArgs.Get("siderolink.api") != nil,
		extraKernelArgs.Get("talos.events.sink") != nil,
		extraKernelArgs.Get("talos.logging.kernel") != nil:
		return errors.New("siderolink kernel arguments are already set, cannot run with --with-siderolink")
	default:
		marshaled, err := slb.ConfigDocument(tunnel).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			panic(fmt.Sprintf("failed to marshal trusted roots config: %s", err))
		}

		var buf bytes.Buffer

		zencoder, err := zstd.NewWriter(&buf)
		if err != nil {
			return fmt.Errorf("failed to create zstd encoder: %w", err)
		}

		_, err = zencoder.Write(marshaled)
		if err != nil {
			return fmt.Errorf("failed to write zstd data: %w", err)
		}

		if err = zencoder.Close(); err != nil {
			return fmt.Errorf("failed to close zstd encoder: %w", err)
		}

		extraKernelArgs.Append(constants.KernelParamConfigEarly, base64.StdEncoding.EncodeToString(buf.Bytes()))

		return nil
	}
}

func getDynamicPort(ctx context.Context, network string) (int, error) {
	var (
		closeFn func() error
		addrFn  func() net.Addr
	)

	switch network {
	case "tcp", "tcp4", "tcp6":
		l, err := (&net.ListenConfig{}).Listen(ctx, network, "127.0.0.1:0")
		if err != nil {
			return 0, err
		}

		addrFn, closeFn = l.Addr, l.Close
	case "udp", "udp4", "udp6":
		l, err := (&net.ListenConfig{}).ListenPacket(ctx, network, "127.0.0.1:0")
		if err != nil {
			return 0, err
		}

		addrFn, closeFn = l.LocalAddr, l.Close
	default:
		return 0, fmt.Errorf("unsupported network: %s", network)
	}

	_, portStr, err := net.SplitHostPort(addrFn().String())
	if err != nil {
		return 0, handleCloseErr(err, closeFn())
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}

	return port, handleCloseErr(nil, closeFn())
}

func handleCloseErr(err error, closeErr error) error {
	switch {
	case err != nil && closeErr != nil:
		return fmt.Errorf("error: %w, close error: %w", err, closeErr)
	case err == nil && closeErr != nil:
		return closeErr
	case err != nil && closeErr == nil:
		return err
	default:
		return nil
	}
}

func checkPortsDontOverlap(ports ...int) error {
	slices.Sort(ports)

	if len(ports) != len(slices.Compact(ports)) {
		return errors.New("generated ports overlap")
	}

	return nil
}
