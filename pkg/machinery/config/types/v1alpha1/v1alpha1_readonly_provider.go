// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ReadonlyProvider wraps the *v1alpha1.Config to make config read-only.
//
// +k8s:deepcopy-gen=false
type ReadonlyProvider struct {
	cfg   *Config
	bytes []byte
}

// WrapReadonly the v1alpha.Config providing read-only interface to it.
func WrapReadonly(cfg *Config, bytes []byte) *ReadonlyProvider {
	return &ReadonlyProvider{
		cfg:   cfg,
		bytes: bytes,
	}
}

// Debug implements the config.Provider interface.
func (r *ReadonlyProvider) Debug() bool {
	return r.cfg.Debug()
}

// Persist implements the config.Provider interface.
func (r *ReadonlyProvider) Persist() bool {
	return r.cfg.Persist()
}

// Machine implements the config.Provider interface.
func (r *ReadonlyProvider) Machine() config.MachineConfig {
	return r.cfg.Machine()
}

// Cluster implements the config.Provider interface.
func (r *ReadonlyProvider) Cluster() config.ClusterConfig {
	return r.cfg.Cluster()
}

// Validate checks configuration and returns warnings and fatal errors (as multierror).
func (r *ReadonlyProvider) Validate(mode validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	return r.cfg.Validate(mode, opts...)
}

// Bytes returns source YAML representation (if available) or does default encoding.
func (r *ReadonlyProvider) Bytes() ([]byte, error) {
	if r.bytes == nil {
		return r.bytes, errors.New("incorrect provider state: bytes is nil")
	}

	return r.bytes, nil
}

// RedactSecrets implements the config.Provider interface.
func (r *ReadonlyProvider) RedactSecrets(replacement string) config.Encoder {
	return r.cfg.RedactSecrets(replacement)
}

// EncodeString implements the config.Provider interface.
func (r *ReadonlyProvider) EncodeString(encoderOptions ...encoder.Option) (string, error) {
	return r.cfg.EncodeString(encoderOptions...)
}

// EncodeBytes implements the config.Provider interface.
func (r *ReadonlyProvider) EncodeBytes(encoderOptions ...encoder.Option) ([]byte, error) {
	return r.cfg.EncodeBytes(encoderOptions...)
}

// RawV1Alpha1 implements the config.Provider interface.
func (r *ReadonlyProvider) RawV1Alpha1() *Config {
	return r.cfg.DeepCopy()
}
