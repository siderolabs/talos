// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	v1alpha1config "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

// patchNodeConfig updates node configuration by means of patch function.
//
//nolint:gocyclo
func patchNodeConfig(ctx context.Context, cluster UpgradeProvider, node string, patchFunc func(config *v1alpha1config.Config) error) error {
	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	resources, err := c.Resources.Get(ctx, config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID)
	if err != nil {
		return fmt.Errorf("error fetching config resource: %w", err)
	}

	if len(resources) != 1 {
		return fmt.Errorf("expected 1 instance of config resource, got %d", len(resources))
	}

	r := resources[0]

	yamlConfig, err := yaml.Marshal(r.Resource.Spec())
	if err != nil {
		return fmt.Errorf("error getting YAML config: %w", err)
	}

	config, err := configloader.NewFromBytes(yamlConfig)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	cfg, ok := config.Raw().(*v1alpha1config.Config)
	if !ok {
		return fmt.Errorf("config is not v1alpha1 config")
	}

	if !cfg.Persist() {
		return fmt.Errorf("config persistence is disabled, patching is not supported")
	}

	if err = patchFunc(cfg); err != nil {
		return fmt.Errorf("error patching config: %w", err)
	}

	cfgBytes, err := cfg.Bytes()
	if err != nil {
		return fmt.Errorf("error serializing config: %w", err)
	}

	_, err = c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data:      cfgBytes,
		Immediate: true,
	})
	if err != nil {
		return fmt.Errorf("error applying config: %w", err)
	}

	return nil
}
