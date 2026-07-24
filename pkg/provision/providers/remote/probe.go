// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/provision"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
)

var _ provision.HTTPProbeProvisioner = (*Provisioner)(nil)

// ProbeHTTP performs a bounded HTTP GET from the remote provisioner host network namespace.
func (p *Provisioner) ProbeHTTP(
	ctx context.Context,
	cluster provision.Cluster,
	request provision.HTTPProbeRequest,
) (provision.HTTPProbeResponse, error) {
	request, err := request.Normalize()
	if err != nil {
		return provision.HTTPProbeResponse{}, err
	}

	client, err := p.dial(ctx)
	if err != nil {
		return provision.HTTPProbeResponse{}, err
	}

	response, err := client.ProbeHTTP(ctx, &remoteprovisionpb.ProbeHTTPRequest{
		ClusterName:   cluster.Info().ClusterName,
		Ip:            request.IP.String(),
		Port:          uint32(request.Port),
		Path:          request.Path,
		TimeoutMillis: uint32(request.Timeout.Milliseconds()),
	})
	if err != nil {
		return provision.HTTPProbeResponse{}, fmt.Errorf("remote HTTP probe: %w", err)
	}

	switch outcome := response.GetOutcome().(type) {
	case *remoteprovisionpb.ProbeHTTPResponse_Result:
		return provision.HTTPProbeResponse{
			StatusCode: int(outcome.Result.GetStatusCode()),
			Body:       outcome.Result.GetBody(),
		}, nil
	case *remoteprovisionpb.ProbeHTTPResponse_Failure:
		return provision.HTTPProbeResponse{Failure: outcome.Failure}, nil
	default:
		return provision.HTTPProbeResponse{}, fmt.Errorf("remote HTTP probe returned no outcome")
	}
}
