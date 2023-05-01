// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/martinlindhe/base36"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// UdevRuleController is a controller that generates udev rules.
type UdevRuleController struct{}

// Name implements controller.Controller interface.
func (ctrl *UdevRuleController) Name() string {
	return "files.UdevRuleController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UdevRuleController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UdevRuleController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.UdevRuleType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// nolint:gocyclo
func (ctrl *UdevRuleController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		touchedIDs := make(map[string]struct{}, len(cfg.Config().Machine().Udev().Rules()))

		for _, rule := range cfg.Config().Machine().Udev().Rules() {
			ruleID := ctrl.generateRuleHash(rule)

			if err = safe.WriterModify(ctx, r, files.NewUdevRule(ruleID), func(udevRule *files.UdevRule) error {
				udevRule.TypedSpec().Rule = rule

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[ruleID] = struct{}{}
		}

		// list keys for cleanup
		list, err := safe.ReaderList[*files.UdevRule](ctx, r, resource.NewMetadata(files.NamespaceName, files.UdevRuleType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing udev rules: %w", err)
		}

		for iter := safe.IteratorFromList(list); iter.Next(); {
			rule := iter.Value()

			if _, ok := touchedIDs[rule.Metadata().ID()]; !ok {
				if err := r.Destroy(ctx, rule.Metadata()); err != nil {
					return fmt.Errorf("error deleting udev rule %s: %w", rule.Metadata().ID(), err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *UdevRuleController) generateRuleHash(rule string) string {
	h := sha256.New()
	h.Write([]byte(rule))

	hashBytes := h.Sum(nil)

	b36 := strings.ToLower(base36.EncodeBytes(hashBytes))

	return b36[:8]
}
