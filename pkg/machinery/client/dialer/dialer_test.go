// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dialer_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
)

func TestDynamicProxyDialer_SOCKS5(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "socks5://localhost:12345")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Expect a connection error because the port is not open
	_, err := dialer.DynamicProxyDialer(ctx, "example.com:443")
	if err == nil {
		t.Fatal("expected a SOCKS5 connection error, but no error received")
	}

	if _, ok := err.(net.Error); !ok {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDialerForProxyURL_SOCKS5(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := dialer.ForProxyURL("socks5://localhost:12345")(ctx, "example.com:443")
	if err == nil {
		t.Fatal("expected a SOCKS5 connection error, but no error received")
	}

	if _, ok := err.(net.Error); !ok {
		t.Fatalf("unexpected error type %T: %v", err, err)
	}
}

func TestDialerForProxyURL_Direct(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := dialer.ForProxyURL("direct")(ctx, "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected a connection error, but no error received")
	}

	if _, ok := err.(net.Error); !ok {
		t.Fatalf("unexpected error type %T: %v", err, err)
	}
}

func TestDialerForProxyURL_InvalidScheme(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := dialer.ForProxyURL("ftp://proxy.example.com:21")(ctx, "example.com:443")
	if err == nil {
		t.Fatal("expected an error for unsupported scheme, but no error received")
	}

	if _, ok := err.(net.Error); ok {
		t.Fatalf("expected a non-net.Error for unsupported scheme, got: %v", err)
	}
}
