// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// the build is constrained to architectures supported by the hypercall package
//go:build amd64 || arm64

package vmware

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/equinix-ms/go-vmw-guestrpc/pkg/hypercall"
	"github.com/equinix-ms/go-vmw-guestrpc/pkg/nanotoolbox"
	"github.com/siderolabs/go-procfs/procfs"
	yaml "gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Read and de-base64 a property from `extraConfig`. This is commonly referred to as `guestinfo`.
func readConfigFromExtraConfig(rpci *nanotoolbox.RPCI, key string) ([]byte, error) {
	val, err := rpci.InfoGet(constants.VMwareGuestInfoPrefix+key, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get extraConfig %s: %w", key, err)
	}

	if val == "" { // not present
		log.Printf("empty (thus absent) %s", key)

		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("failed to decode extraConfig %s: %w", key, err)
	}

	if len(decoded) == 0 {
		log.Printf("skipping zero-length config in extraConfig")

		return nil, nil
	}

	return decoded, nil
}

// ofvEnv and related types are extracted from github.com/vmware/govmomi/ovf/env.go.
type ovfEnvFile struct {
	XMLName xml.Name `xml:"http://schemas.dmtf.org/ovf/environment/1 Environment"`
	ID      string   `xml:"id,attr"`
	EsxID   string   `xml:"http://www.vmware.com/schema/ovfenv esxId,attr"`

	Platform *ovfPlatformSection `xml:"PlatformSection"`
	Property *ovfPropertySection `xml:"PropertySection"`
}

type ovfPlatformSection struct {
	Kind    string `xml:"Kind"`
	Version string `xml:"Version"`
	Vendor  string `xml:"Vendor"`
	Locale  string `xml:"Locale"`
}

type ovfPropertySection struct {
	Properties []ovfEnvProperty `xml:"Property"`
}

type ovfEnvProperty struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

// Read and de-base64 a property from the OVF env. This is different way to pass data to your VM.
// This is how data gets passed when using vCloud Director.
func readConfigFromOvf(rpci *nanotoolbox.RPCI, key string) ([]byte, error) {
	ovfXML, err := rpci.InfoGet(constants.VMwareGuestInfoPrefix+constants.VMwareGuestInfoOvfEnvKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read extraConfig var '%s': %w", key, err)
	}

	if ovfXML == "" { // value empty (probably because not present)
		return nil, nil
	}

	var ovfEnv ovfEnvFile

	err = xml.Unmarshal([]byte(ovfXML), &ovfEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall XML from OVF env: %w", err)
	}

	if ovfEnv.Property == nil || ovfEnv.Property.Properties == nil { // no data in OVF env
		log.Printf("empty OVF env")

		return nil, nil
	}

	log.Printf("searching for property '%s' in OVF", key)

	for _, property := range ovfEnv.Property.Properties { // iterate to check if our key is present
		if property.Key == key {
			log.Printf("it is there, decoding")

			decoded, err := base64.StdEncoding.DecodeString(property.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to decode OVF property %s: %w", property.Key, err)
			}

			if len(decoded) == 0 {
				log.Printf("skipping zero-length config in OVF")

				return nil, nil
			}

			return decoded, nil
		}
	}

	return nil, nil
}

func initializeRPCI() (*nanotoolbox.RPCI, error) {
	if !hypercall.IsVMWareVM() {
		return nil, errors.New("this is not a VMWare VM")
	}

	rpci, err := nanotoolbox.NewRPCI(slog.New(slog.NewTextHandler(log.Writer(), nil)))
	if err != nil {
		return nil, fmt.Errorf("could not initialize RPCI: %w", err)
	}

	if err = rpci.Start(); err != nil {
		return nil, fmt.Errorf("could not start RPCI: %w", err)
	}

	return rpci, nil
}

// Configuration implements the platform.Platform interface.
//
//nolint:gocyclo
func (v *VMware) Configuration(context.Context, state.State) ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, fmt.Errorf("%s not found", constants.KernelParamConfig)
	}

	if *option == constants.ConfigGuestInfo {
		log.Printf("fetching machine config from VMware extraConfig or OVF env")

		rpci, err := initializeRPCI()
		if err != nil {
			return nil, fmt.Errorf("error initiliazing RPCI: %w", err)
		}
		defer rpci.Stop() //nolint:errcheck

		// try to fetch `talos.config` from plain extraConfig (ie, the old behavior)
		log.Printf("trying to find '%s' in extraConfig", constants.VMwareGuestInfoConfigKey)

		config, err := readConfigFromExtraConfig(rpci, constants.VMwareGuestInfoConfigKey)
		if err != nil {
			return nil, err
		}

		if config != nil {
			return config, nil
		}

		// try to fetch `userdata` from plain extraConfig (ie, the old behavior)
		log.Printf("trying to find '%s' in extraConfig", constants.VMwareGuestInfoFallbackKey)

		config, err = readConfigFromExtraConfig(rpci, constants.VMwareGuestInfoFallbackKey)
		if err != nil {
			return nil, err
		}

		if config != nil {
			return config, nil
		}

		// try to fetch `talos.config` from OVF
		log.Printf("trying to find '%s' in OVF env", constants.VMwareGuestInfoConfigKey)

		config, err = readConfigFromOvf(rpci, constants.VMwareGuestInfoConfigKey)
		if err != nil {
			return nil, err
		}

		if config != nil {
			return config, nil
		}

		// try to fetch `userdata` from OVF
		log.Printf("trying to find '%s' in OVF env", constants.VMwareGuestInfoFallbackKey)

		config, err = readConfigFromOvf(rpci, constants.VMwareGuestInfoFallbackKey)
		if err != nil {
			return nil, err
		}

		if config != nil {
			return config, nil
		}

		return nil, platformerrors.ErrNoConfigSource
	}

	return nil, nil
}

// Read VMware GuestInfo metadata if available.
func (v *VMware) readMetadata(rpci *nanotoolbox.RPCI) ([]byte, error) {
	guestInfoMetadata, err := readConfigFromExtraConfig(rpci, constants.VMwareGuestInfoMetadataKey)
	if err != nil {
		return nil, err
	}

	if guestInfoMetadata == nil {
		guestInfoMetadata, err = readConfigFromOvf(rpci, constants.VMwareGuestInfoMetadataKey)
	}

	if err != nil {
		return nil, err
	}

	return guestInfoMetadata, nil
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *VMware) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	rpci, err := initializeRPCI()
	if err != nil {
		return fmt.Errorf("error initiliazing RPCI: %w", err)
	}
	defer rpci.Stop() //nolint:errcheck

	guestInfoMetadata, err := v.readMetadata(rpci)
	if err != nil {
		return fmt.Errorf("failed to read GuestInfo: %w", err)
	}

	networkConfig := &runtime.PlatformNetworkConfig{
		Metadata: &runtimeres.PlatformMetadataSpec{Platform: v.Name()},
	}

	if guestInfoMetadata != nil {
		var unmarshalledNetworkConfig NetworkConfig
		if err = yaml.Unmarshal(guestInfoMetadata, &unmarshalledNetworkConfig); err != nil {
			return fmt.Errorf("failed to unmarshall metadata '%s'. Error '%w'", guestInfoMetadata, err)
		}

		switch unmarshalledNetworkConfig.Network.Version {
		case 2:
			err := v.ApplyNetworkConfigV2(ctx, st, &unmarshalledNetworkConfig, networkConfig)
			if err != nil {
				return fmt.Errorf("failed to apply metadata '%s'. Error '%w'", guestInfoMetadata, err)
			}

			networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
				Platform:   v.Name(),
				Hostname:   unmarshalledNetworkConfig.LocalHostname,
				InstanceID: unmarshalledNetworkConfig.InstanceID,
			}
		default:
			return fmt.Errorf("GuestInfo version=%d is not supported. GuestInfo: %s", unmarshalledNetworkConfig.Network.Version, guestInfoMetadata)
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- networkConfig:
	}

	return nil
}
