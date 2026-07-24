// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
)

func TestRemoteProvisionHTTPProbeValidation(t *testing.T) {
	server := &remoteProvisionImpl{stateDir: t.TempDir()}

	tests := map[string]*remoteprovisionpb.ProbeHTTPRequest{
		"cluster": {Ip: "203.0.113.100", Port: 80, Path: "/"},
		"IP":      {ClusterName: "test", Ip: "example.com", Port: 80, Path: "/"},
		"port":    {ClusterName: "test", Ip: "203.0.113.100", Path: "/"},
		"path":    {ClusterName: "test", Ip: "203.0.113.100", Port: 80, Path: "relative"},
	}

	for name, request := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := server.ProbeHTTP(t.Context(), request)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestRemoteProvisionHTTPProbeUnknownCluster(t *testing.T) {
	server := &remoteProvisionImpl{stateDir: t.TempDir()}

	_, err := server.ProbeHTTP(t.Context(), &remoteprovisionpb.ProbeHTTPRequest{
		ClusterName: "missing",
		Ip:          "203.0.113.100",
		Port:        80,
		Path:        "/",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
