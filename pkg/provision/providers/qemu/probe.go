// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"

	"github.com/siderolabs/talos/pkg/provision"
)

var _ provision.HTTPProbeProvisioner = (*provisioner)(nil)

// ProbeHTTP probes an address through the provisioner host FIB.
func (p *provisioner) ProbeHTTP(
	ctx context.Context,
	_ provision.Cluster,
	request provision.HTTPProbeRequest,
) (provision.HTTPProbeResponse, error) {
	if runtime.GOOS != "linux" {
		return provision.HTTPProbeResponse{}, fmt.Errorf("provisioner HTTP probe is only supported on Linux")
	}

	return probeHTTP(ctx, request)
}

func probeHTTP(ctx context.Context, request provision.HTTPProbeRequest) (provision.HTTPProbeResponse, error) {
	request, err := request.Normalize()
	if err != nil {
		return provision.HTTPProbeResponse{}, err
	}

	target := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(request.IP.String(), strconv.FormatUint(uint64(request.Port), 10)),
		Path:   request.Path,
	}

	probeCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(probeCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return provision.HTTPProbeResponse{}, fmt.Errorf("error building provisioner HTTP probe: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert
	transport.Proxy = nil

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return provision.HTTPProbeResponse{Failure: err.Error()}, nil
	}

	defer response.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(response.Body, provision.HTTPProbeMaxResponseBody))
	if err != nil {
		return provision.HTTPProbeResponse{Failure: err.Error()}, nil
	}

	return provision.HTTPProbeResponse{
		StatusCode: response.StatusCode,
		Body:       body,
	}, nil
}
