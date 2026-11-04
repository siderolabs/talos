// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// SELinuxBooleanController sets the SELinux booleans of the machine config, and reports every boolean of the loaded
// policy with its default and its value.
//
// A boolean the machine config leaves out goes back to the value it had when the policy was loaded. Setting a boolean
// does not load the policy again.
type SELinuxBooleanController struct {
	// BooleansPath is the directory of the booleans in selinuxfs, absent when SELinux is disabled.
	BooleansPath string
	// CommitPath is the selinuxfs file that commits the pending values of the booleans.
	CommitPath string

	defaults map[string]bool
}

// Name implements controller.Controller interface.
func (ctrl *SELinuxBooleanController) Name() string {
	return "runtime.SELinuxBooleanController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SELinuxBooleanController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SELinuxBooleanController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.SELinuxBooleanStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *SELinuxBooleanController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *SELinuxBooleanController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	var configured map[string]bool

	if cfg != nil && cfg.Config().SecurityProfileConfig() != nil {
		configured = cfg.Config().SecurityProfileConfig().SELinuxBooleans()
	}

	current, err := ctrl.read()
	if err != nil {
		return err
	}

	// the first read is the policy as loaded, before any boolean of the machine config is set
	if ctrl.defaults == nil {
		ctrl.defaults = current
	}

	if current, err = ctrl.apply(current, configured, logger); err != nil {
		return err
	}

	return ctrl.publish(ctx, r, current, configured)
}

// publish reports every boolean of the loaded policy, and every boolean of the machine config the policy does not define.
func (ctrl *SELinuxBooleanController) publish(ctx context.Context, r controller.Runtime, current, configured map[string]bool) error {
	r.StartTrackingOutputs()

	statuses := make(map[string]runtime.SELinuxBooleanStatusSpec, len(current)+len(configured))

	for name, value := range current {
		_, isConfigured := configured[name]
		statuses[name] = runtime.SELinuxBooleanStatusSpec{Value: value, Default: ctrl.defaults[name], Configured: isConfigured}
	}

	for name := range configured {
		if _, known := current[name]; !known {
			statuses[name] = runtime.SELinuxBooleanStatusSpec{Configured: true, Unknown: true}
		}
	}

	for name, spec := range statuses {
		if err := safe.WriterModify(ctx, r, runtime.NewSELinuxBooleanStatus(name), func(res *runtime.SELinuxBooleanStatus) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return err
		}
	}

	return safe.CleanupOutputs[*runtime.SELinuxBooleanStatus](ctx, r)
}

// apply sets every boolean to the value of the machine config, else to its default, commits the change, and returns
// the values the kernel reports afterwards.
func (ctrl *SELinuxBooleanController) apply(current, configured map[string]bool, logger *zap.Logger) (map[string]bool, error) {
	changed := false

	for name, value := range current {
		want, ok := configured[name]
		if !ok {
			want = ctrl.defaults[name]
		}

		if value == want {
			continue
		}

		if err := os.WriteFile(filepath.Join(ctrl.BooleansPath, name), []byte(boolString(want)), 0); err != nil {
			return nil, fmt.Errorf("error setting SELinux boolean %q: %w", name, err)
		}

		logger.Info("setting SELinux boolean", zap.String("boolean", name), zap.Bool("value", want))

		changed = true
	}

	if !changed {
		return current, nil
	}

	if err := os.WriteFile(ctrl.CommitPath, []byte("1"), 0); err != nil {
		return nil, fmt.Errorf("error committing SELinux booleans: %w", err)
	}

	return ctrl.read()
}

// read returns the current value of every boolean of the loaded policy, none when SELinux is disabled.
func (ctrl *SELinuxBooleanController) read() (map[string]bool, error) {
	entries, err := os.ReadDir(ctrl.BooleansPath)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error listing SELinux booleans: %w", err)
	}

	booleans := make(map[string]bool, len(entries))

	for _, entry := range entries {
		// each file holds the current and the pending value, "1 1"
		contents, err := os.ReadFile(filepath.Join(ctrl.BooleansPath, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("error reading SELinux boolean %q: %w", entry.Name(), err)
		}

		fields := strings.Fields(string(contents))
		booleans[entry.Name()] = len(fields) > 0 && fields[0] == "1"
	}

	return booleans, nil
}

func boolString(value bool) string {
	if value {
		return "1"
	}

	return "0"
}
