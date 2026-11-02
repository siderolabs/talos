// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

const healthCheckDocHeader = `apiVersion: v1alpha1
kind: ContainerConfig
name: web
image: docker.io/library/nginx:1.27
`

func TestContainerConfigNoHealthCheck(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		doc  string
	}{
		{name: "no lifecycle", doc: ""},
		{name: "empty lifecycle", doc: "lifecycle: {}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, healthCheckDocHeader+test.doc)

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			assert.False(t, cfg.HealthCheck().IsPresent())
		})
	}
}

func TestContainerConfigHealthCheck(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		doc  string

		kind         config.ContainerHealthCheckKind
		port         uint16
		path         string
		service      string
		initialDelay time.Duration
		interval     time.Duration
		timeout      time.Duration
		threshold    uint32
	}{
		{
			name:      "tcp with defaults",
			doc:       "lifecycle:\n  healthCheck:\n    tcp:\n      port: 5432\n",
			kind:      config.ContainerHealthCheckKindTCP,
			port:      5432,
			interval:  10 * time.Second,
			timeout:   time.Second,
			threshold: 3,
		},
		{
			name:      "http defaults the path",
			doc:       "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n",
			kind:      config.ContainerHealthCheckKindHTTP,
			port:      80,
			path:      "/",
			interval:  10 * time.Second,
			timeout:   time.Second,
			threshold: 3,
		},
		{
			name: "http with everything set",
			doc: `lifecycle:
  healthCheck:
    http:
      port: 8080
      path: /healthz
    initialDelay: 5s
    interval: 30s
    timeout: 3s
    failureThreshold: 5
`,
			kind:         config.ContainerHealthCheckKindHTTP,
			port:         8080,
			path:         "/healthz",
			initialDelay: 5 * time.Second,
			interval:     30 * time.Second,
			timeout:      3 * time.Second,
			threshold:    5,
		},
		{
			name:      "http path with a query and an escape",
			doc:       "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n      path: /health%20z?full=1\n",
			kind:      config.ContainerHealthCheckKindHTTP,
			port:      80,
			path:      "/health%20z?full=1",
			interval:  10 * time.Second,
			timeout:   time.Second,
			threshold: 3,
		},
		{
			name:      "grpc with a service",
			doc:       "lifecycle:\n  healthCheck:\n    grpc:\n      port: 9000\n      service: api\n",
			kind:      config.ContainerHealthCheckKindGRPC,
			port:      9000,
			service:   "api",
			interval:  10 * time.Second,
			timeout:   time.Second,
			threshold: 3,
		},
		{
			name:      "timeout equal to the interval",
			doc:       "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: 2s\n    timeout: 2s\n",
			kind:      config.ContainerHealthCheckKindTCP,
			port:      80,
			interval:  2 * time.Second,
			timeout:   2 * time.Second,
			threshold: 3,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, healthCheckDocHeader+test.doc)

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			healthCheck, ok := cfg.HealthCheck().Get()
			require.True(t, ok)

			assert.Equal(t, test.kind, healthCheck.Kind())
			assert.Equal(t, test.port, healthCheck.Port())
			assert.Equal(t, test.path, healthCheck.HTTPPath())
			assert.Equal(t, test.service, healthCheck.GRPCService())
			assert.Equal(t, test.initialDelay, healthCheck.InitialDelay())
			assert.Equal(t, test.interval, healthCheck.Interval())
			assert.Equal(t, test.timeout, healthCheck.Timeout())
			assert.Equal(t, test.threshold, healthCheck.FailureThreshold())
		})
	}
}

func TestContainerConfigHealthCheckValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		doc         string
		expectedErr string
	}{
		{
			name:        "no probe",
			doc:         "lifecycle:\n  healthCheck:\n    interval: 5s\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "two probes",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    http:\n      port: 80\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "http and grpc probes",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      port: 8080\n      path: /healthz\n    grpc:\n      port: 9000\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "tcp and grpc probes",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 5432\n    grpc:\n      port: 9000\n      service: api\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "all three probes",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    http:\n      port: 8080\n    grpc:\n      port: 9000\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "no port",
			doc:         "lifecycle:\n  healthCheck:\n    grpc:\n      service: api\n",
			expectedErr: "lifecycle.healthCheck.grpc.port is required",
		},
		{
			name:        "relative path",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n      path: healthz\n",
			expectedErr: `lifecycle.healthCheck.http.path "healthz" must start with /`,
		},
		{
			name:        "invalid escape in path",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n      path: /health%zz\n",
			expectedErr: `lifecycle.healthCheck.http.path "/health%zz" is not a valid request path`,
		},
		{
			name:        "control character in path",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n      path: \"/a\\x7fb\"\n",
			expectedErr: `is not a valid request path`,
		},
		{
			name:        "fragment in path",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      port: 80\n      path: /healthz#top\n",
			expectedErr: `lifecycle.healthCheck.http.path "/healthz#top" must not contain a fragment`,
		},
		{
			name:        "timeout longer than the interval",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: 2s\n    timeout: 5s\n",
			expectedErr: "lifecycle.healthCheck.timeout 5s must not exceed lifecycle.healthCheck.interval 2s",
		},
		{
			name:        "timeout longer than the default interval",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    timeout: 1m\n",
			expectedErr: "lifecycle.healthCheck.timeout 1m0s must not exceed lifecycle.healthCheck.interval 10s",
		},
		{
			name:        "negative initial delay",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    initialDelay: -1s\n",
			expectedErr: "lifecycle.healthCheck.initialDelay must not be negative",
		},
		{
			name:        "empty health check",
			doc:         "lifecycle:\n  healthCheck: {}\n",
			expectedErr: "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set",
		},
		{
			name:        "tcp without a port",
			doc:         "lifecycle:\n  healthCheck:\n    tcp: {}\n",
			expectedErr: "lifecycle.healthCheck.tcp.port is required",
		},
		{
			name:        "http without a port",
			doc:         "lifecycle:\n  healthCheck:\n    http:\n      path: /healthz\n",
			expectedErr: "lifecycle.healthCheck.http.port is required",
		},
		{
			name:        "negative interval",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: -1s\n",
			expectedErr: "lifecycle.healthCheck.interval must not be negative",
		},
		{
			name:        "negative timeout",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    timeout: -1s\n",
			expectedErr: "lifecycle.healthCheck.timeout must not be negative",
		},
		{
			name:        "default timeout longer than the interval",
			doc:         "lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: 500ms\n",
			expectedErr: "lifecycle.healthCheck.timeout 1s must not exceed lifecycle.healthCheck.interval 500ms",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, healthCheckDocHeader+test.doc)

			_, err := cfg.Validate(validationMode{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedErr)
		})
	}
}

func TestContainerConfigHealthCheckValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("negative interval skips the timeout comparison", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, healthCheckDocHeader+"lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: -1s\n    timeout: 5s\n")

		_, err := cfg.Validate(validationMode{})
		require.ErrorContains(t, err, "lifecycle.healthCheck.interval must not be negative")
		assert.NotContains(t, err.Error(), "must not exceed")
	})

	t.Run("negative timeout skips the timeout comparison", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, healthCheckDocHeader+"lifecycle:\n  healthCheck:\n    tcp:\n      port: 80\n    interval: 2s\n    timeout: -5s\n")

		_, err := cfg.Validate(validationMode{})
		require.ErrorContains(t, err, "lifecycle.healthCheck.timeout must not be negative")
		assert.NotContains(t, err.Error(), "must not exceed")
	})

	t.Run("probe and timing errors are both reported", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, healthCheckDocHeader+"lifecycle:\n  healthCheck:\n    interval: 2s\n    timeout: 5s\n")

		_, err := cfg.Validate(validationMode{})
		require.ErrorContains(t, err, "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set")
		assert.Contains(t, err.Error(), "lifecycle.healthCheck.timeout 5s must not exceed lifecycle.healthCheck.interval 2s")
	})
}

func TestContainerConfigHealthCheckWithoutProbe(t *testing.T) {
	t.Parallel()

	cfg := load(t, healthCheckDocHeader+"lifecycle:\n  healthCheck:\n    interval: 5s\n")

	healthCheck, ok := cfg.HealthCheck().Get()
	require.True(t, ok)

	assert.Equal(t, config.ContainerHealthCheckKind(""), healthCheck.Kind())
	assert.Equal(t, uint16(0), healthCheck.Port())
	assert.Empty(t, healthCheck.HTTPPath())
	assert.Empty(t, healthCheck.GRPCService())
}
