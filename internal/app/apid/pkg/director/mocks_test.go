// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package director_test

import (
	"context"

	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
)

type mockBackend struct {
	target string
}

func (m *mockBackend) String() string {
	return m.target
}

func (m *mockBackend) GetConnection(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
	return ctx, nil, nil
}

func (m *mockBackend) AppendInfo(streaming bool, resp []byte) ([]byte, error) {
	return resp, nil
}

func (m *mockBackend) BuildError(streaming bool, err error) ([]byte, error) {
	return nil, nil
}

func mockBackendFactory(target string) (proxy.Backend, error) {
	return &mockBackend{target: target}, nil
}

type mockLocalAddressProvider struct {
	local map[string]struct{}
}

func (m *mockLocalAddressProvider) IsLocalTarget(t string) bool {
	_, ok := m.local[t]

	return ok
}
