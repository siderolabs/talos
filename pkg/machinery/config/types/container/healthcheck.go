// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Health check defaults, applied when the corresponding field is unset.
const (
	DefaultHealthCheckInterval         = 10 * time.Second
	DefaultHealthCheckTimeout          = time.Second
	DefaultHealthCheckFailureThreshold = 3
)

// ContainerHealthCheck configures a probe which decides whether a running container is healthy.
//
// Exactly one of tcp, http or grpc must be set.
type ContainerHealthCheck struct {
	//   description: |
	//     Probe by opening a TCP connection to the port.
	//
	//     The check passes if the connection is established.
	TCPCheck *ContainerTCPHealthCheck `yaml:"tcp,omitempty"`
	//   description: |
	//     Probe by sending an HTTP GET request to the port.
	//
	//     The check passes if the response status is in the 200-399 range.
	HTTPCheck *ContainerHTTPHealthCheck `yaml:"http,omitempty"`
	//   description: |
	//     Probe by calling the gRPC health checking protocol (`grpc.health.v1.Health/Check`) on the port.
	//
	//     The check passes if the reported status is `SERVING`.
	GRPCCheck *ContainerGRPCHealthCheck `yaml:"grpc,omitempty"`
	//   description: |
	//     Delay after the container starts before the first check runs.
	//
	//     Defaults to `0s`.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	CheckInitialDelay time.Duration `yaml:"initialDelay,omitempty"`
	//   description: |
	//     Interval between checks.
	//
	//     Defaults to `10s`.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	CheckInterval time.Duration `yaml:"interval,omitempty"`
	//   description: |
	//     Time after which a single check is considered failed.
	//
	//     Defaults to `1s`, and may not exceed the interval.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	CheckTimeout time.Duration `yaml:"timeout,omitempty"`
	//   description: |
	//     Number of consecutive failed checks after which the container is reported degraded.
	//
	//     Defaults to `3`.
	CheckFailureThreshold uint32 `yaml:"failureThreshold,omitempty"`
}

// ContainerTCPHealthCheck probes a TCP port.
type ContainerTCPHealthCheck struct {
	//   description: |
	//     Port to connect to on the container's loopback address.
	CheckPort uint16 `yaml:"port"`
}

// ContainerHTTPHealthCheck probes an HTTP endpoint.
type ContainerHTTPHealthCheck struct {
	//   description: |
	//     Port to connect to on the container's loopback address.
	CheckPort uint16 `yaml:"port"`
	//   description: |
	//     Request path.
	//
	//     Defaults to `/`.
	//   examples:
	//     - value: '"/healthz"'
	CheckPath string `yaml:"path,omitempty"`
}

// ContainerGRPCHealthCheck probes a gRPC health service.
type ContainerGRPCHealthCheck struct {
	//   description: |
	//     Port to connect to on the container's loopback address.
	CheckPort uint16 `yaml:"port"`
	//   description: |
	//     Service name to pass in the health check request.
	//
	//     Empty asks for the overall health of the server.
	CheckService string `yaml:"service,omitempty"`
}

// Check interfaces.
var _ config.ContainerHealthCheckConfig = &ContainerHealthCheck{}

// Kind implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) Kind() config.ContainerHealthCheckKind {
	switch {
	case h.TCPCheck != nil:
		return config.ContainerHealthCheckKindTCP
	case h.HTTPCheck != nil:
		return config.ContainerHealthCheckKindHTTP
	case h.GRPCCheck != nil:
		return config.ContainerHealthCheckKindGRPC
	default:
		return ""
	}
}

// Port implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) Port() uint16 {
	switch {
	case h.TCPCheck != nil:
		return h.TCPCheck.CheckPort
	case h.HTTPCheck != nil:
		return h.HTTPCheck.CheckPort
	case h.GRPCCheck != nil:
		return h.GRPCCheck.CheckPort
	default:
		return 0
	}
}

// HTTPPath implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) HTTPPath() string {
	if h.HTTPCheck == nil {
		return ""
	}

	if h.HTTPCheck.CheckPath == "" {
		return "/"
	}

	return h.HTTPCheck.CheckPath
}

// GRPCService implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) GRPCService() string {
	if h.GRPCCheck == nil {
		return ""
	}

	return h.GRPCCheck.CheckService
}

// InitialDelay implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) InitialDelay() time.Duration { return h.CheckInitialDelay }

// Interval implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) Interval() time.Duration {
	if h.CheckInterval == 0 {
		return DefaultHealthCheckInterval
	}

	return h.CheckInterval
}

// Timeout implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) Timeout() time.Duration {
	if h.CheckTimeout == 0 {
		return DefaultHealthCheckTimeout
	}

	return h.CheckTimeout
}

// FailureThreshold implements config.ContainerHealthCheckConfig interface.
func (h *ContainerHealthCheck) FailureThreshold() uint32 {
	if h.CheckFailureThreshold == 0 {
		return DefaultHealthCheckFailureThreshold
	}

	return h.CheckFailureThreshold
}

// Validate checks the health check settings.
func (h *ContainerHealthCheck) Validate() error {
	return errors.Join(h.validateProbe(), h.validateTiming())
}

// validateProbe checks that exactly one probe is set, and its settings.
func (h *ContainerHealthCheck) validateProbe() error {
	var (
		validationErrors error
		set              int
	)

	for _, present := range []bool{h.TCPCheck != nil, h.HTTPCheck != nil, h.GRPCCheck != nil} {
		if present {
			set++
		}
	}

	if set != 1 {
		validationErrors = errors.Join(validationErrors, errors.New("lifecycle.healthCheck: exactly one of tcp, http or grpc must be set"))
	} else if h.Port() == 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("lifecycle.healthCheck.%s.port is required", h.Kind()))
	}

	if h.HTTPCheck != nil && h.HTTPCheck.CheckPath != "" {
		validationErrors = errors.Join(validationErrors, validateHTTPPath(h.HTTPCheck.CheckPath))
	}

	return validationErrors
}

// validateHTTPPath checks that path can be sent as the request target of the probe.
//
// The probe builds its URL by appending the path to the scheme and address, so anything which does
// not parse back as exactly that would fail every check rather than at apply time.
func validateHTTPPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("lifecycle.healthCheck.http.path %q must start with /", path)
	}

	parsed, err := url.Parse("http://127.0.0.1" + path)
	if err != nil {
		return fmt.Errorf("lifecycle.healthCheck.http.path %q is not a valid request path: %w", path, err)
	}

	// A fragment is never sent: the probe would silently request a different path than configured.
	if parsed.Fragment != "" || strings.Contains(path, "#") {
		return fmt.Errorf("lifecycle.healthCheck.http.path %q must not contain a fragment", path)
	}

	return nil
}

// validateTiming checks the delay, interval and timeout settings.
func (h *ContainerHealthCheck) validateTiming() error {
	var validationErrors error

	if h.CheckInitialDelay < 0 {
		validationErrors = errors.Join(validationErrors, errors.New("lifecycle.healthCheck.initialDelay must not be negative"))
	}

	if h.CheckInterval < 0 {
		validationErrors = errors.Join(validationErrors, errors.New("lifecycle.healthCheck.interval must not be negative"))
	}

	if h.CheckTimeout < 0 {
		validationErrors = errors.Join(validationErrors, errors.New("lifecycle.healthCheck.timeout must not be negative"))
	}

	if h.CheckInterval >= 0 && h.CheckTimeout >= 0 && h.Timeout() > h.Interval() {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("lifecycle.healthCheck.timeout %s must not exceed lifecycle.healthCheck.interval %s", h.Timeout(), h.Interval()))
	}

	return validationErrors
}
