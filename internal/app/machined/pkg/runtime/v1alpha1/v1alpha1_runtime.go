// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"syscall"

	"github.com/google/go-cmp/cmp"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// Runtime implements the Runtime interface.
type Runtime struct {
	c config.Provider
	s runtime.State
	e runtime.EventStream
	l runtime.LoggingManager
}

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(c config.Provider, s runtime.State, e runtime.EventStream, l runtime.LoggingManager) *Runtime {
	return &Runtime{
		c: c,
		s: s,
		e: e,
		l: l,
	}
}

// Config implements the Runtime interface.
func (r *Runtime) Config() config.Provider {
	return r.c
}

// ValidateConfig implements the Runtime interface.
func (r *Runtime) ValidateConfig(b []byte) (config.Provider, error) {
	cfg, err := configloader.NewFromBytes(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if _, err := cfg.Validate(r.State().Platform().Mode()); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	return cfg, nil
}

// SetConfig implements the Runtime interface.
func (r *Runtime) SetConfig(b []byte) error {
	cfg, err := r.ValidateConfig(b)
	if err != nil {
		return err
	}

	r.c = cfg

	return r.s.V1Alpha2().SetConfig(cfg)
}

// CanApplyImmediate implements the Runtime interface.
func (r *Runtime) CanApplyImmediate(b []byte) error {
	cfg, err := r.ValidateConfig(b)
	if err != nil {
		return err
	}

	// serialize and load back current config to remove any changes made
	// to the config in-memory while the node was running
	currentBytes, err := r.Config().Bytes()
	if err != nil {
		return fmt.Errorf("error serializing current config: %w", err)
	}

	currentConfigProvider, err := configloader.NewFromBytes(currentBytes)
	if err != nil {
		return fmt.Errorf("error loading current config: %w", err)
	}

	currentConfig, ok := currentConfigProvider.(*v1alpha1.Config)
	if !ok {
		return fmt.Errorf("current config is not v1alpha1")
	}

	newConfig, ok := cfg.(*v1alpha1.Config)
	if !ok {
		return fmt.Errorf("new config is not v1alpha1")
	}

	// the config changes allowed to be applied immediately are:
	// * cluster config
	// * .machine.time
	newConfig.ClusterConfig = currentConfig.ClusterConfig

	if newConfig.MachineConfig != nil && currentConfig.MachineConfig != nil {
		newConfig.MachineConfig.MachineTime = currentConfig.MachineConfig.MachineTime
	}

	if !reflect.DeepEqual(currentConfig, newConfig) {
		diff := cmp.Diff(currentConfig, newConfig)

		return fmt.Errorf("this config change can't be applied in immediate mode\ndiff: %s", diff)
	}

	return nil
}

// State implements the Runtime interface.
func (r *Runtime) State() runtime.State {
	return r.s
}

// Events implements the Runtime interface.
func (r *Runtime) Events() runtime.EventStream {
	return r.e
}

// Logging implements the Runtime interface.
func (r *Runtime) Logging() runtime.LoggingManager {
	return r.l
}

// NodeName implements the Runtime interface.
func (r *Runtime) NodeName() (string, error) {
	// attempt to fetch hostname and domain name via syscalls and concat them if necessary
	if r.Config().Machine().Kubelet().RegisterWithFQDN() {
		var utsName syscall.Utsname
		if err := syscall.Uname(&utsName); err != nil {
			return "", err
		}

		nodeName := utsNameVarToString(utsName.Nodename)
		log.Printf("Nodename is %s", nodeName)

		domainName := utsNameVarToString(utsName.Domainname)
		log.Printf("Domain name is %s", domainName)

		// As odd as it looks, the Uname method sets domainName to this "(none)" string if not set.
		if domainName != "(none)" {
			return fmt.Sprintf("%s.%s", nodeName, domainName), nil
		}

		return nodeName, nil
	}

	// default to os.Hostname if we don't need to worry about fqdn.
	return os.Hostname()
}

// converts int8 array to a string
// borrowed from https://github.com/aisola/go-coreutils/blob/master/uname/uname.go#L98
func utsNameVarToString(unameArray [65]int8) string {
	var byteString [65]byte

	var indexLength int

	for unameArray[indexLength] != 0 {
		byteString[indexLength] = uint8(unameArray[indexLength])
		indexLength++
	}

	return string(byteString[:indexLength])
}
