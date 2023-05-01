// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// UdevRuleFileController is a controller for UdevRule files.
type UdevRuleFileController struct {
	V1Alpha1Mode  runtimetalos.Mode
	UdevRulesFile string
	CommandRunner func(ctx context.Context, name string, args ...string) (string, error)
}

// Name implements controller.Controller interface.
func (ctrl *UdevRuleFileController) Name() string {
	return "files.UdevRuleFileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UdevRuleFileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: files.NamespaceName,
			Type:      files.UdevRuleType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UdevRuleFileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.UdevRuleStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// nolint:gocyclo
func (ctrl *UdevRuleFileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// udev rules has no effect in container mode, so skip it.
		if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
			continue
		}

		list, err := safe.ReaderList[*files.UdevRule](ctx, r, resource.NewMetadata(files.NamespaceName, files.UdevRuleType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("failed to list udev rules: %w", err)
		}

		if list.Len() == 0 {
			if err = os.RemoveAll(ctrl.UdevRulesFile); err != nil {
				return fmt.Errorf("failed to remove custom udev rules: %w", err)
			}

			continue
		}

		var content strings.Builder

		for iter := safe.IteratorFromList(list); iter.Next(); {
			rule := iter.Value().TypedSpec().Rule

			content.WriteString(strings.ReplaceAll(rule, "\n", "\\\n"))
			content.WriteByte('\n')
		}

		if err = os.WriteFile(ctrl.UdevRulesFile, []byte(content.String()), 0o644); err != nil {
			return fmt.Errorf("failed writing custom udev rules: %w", err)
		}

		if _, err := ctrl.CommandRunner(ctx, "/sbin/udevadm", "control", "--reload"); err != nil {
			return err
		}

		if _, err := ctrl.CommandRunner(ctx, "/sbin/udevadm", "trigger", "--type=devices", "--action=add"); err != nil {
			return err
		}

		if _, err := ctrl.CommandRunner(ctx, "/sbin/udevadm", "trigger", "--type=subsystems", "--action=add"); err != nil {
			return err
		}

		// This ensures that `udevd` finishes processing kernel events, triggered by
		// `udevd trigger`, to prevent a race condition when a user specifies a path
		// under `/dev/disk/*` in any disk definitions.
		if _, err := ctrl.CommandRunner(ctx, "/sbin/udevadm", "settle", "--timeout=50"); err != nil {
			return err
		}

		if err := safe.WriterModify(ctx, r, files.NewUdevRuleStatus("udev"), func(rule *files.UdevRuleStatus) error {
			rule.TypedSpec().Active = true

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update udev rule status: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
