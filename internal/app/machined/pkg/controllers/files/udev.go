// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

const (
	udevdServiceID    = "udevd"
	udevRulesBindAttr = unix.MOUNT_ATTR_RDONLY | unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV
)

type rulesInfo struct {
	root   xfs.Root
	umount func() error
}

// UdevRulesController reconciles Talos-managed custom udev rules.
type UdevRulesController struct {
	V1Alpha1Mode machineruntime.Mode

	UdevRulesPath string
	// Exposed for testing.
	CommandRunner func(ctx context.Context, name string, args []string) (string, error)

	pendingReload bool
	rulesInfo     *rulesInfo
}

// Name implements controller.Controller interface.
func (ctrl *UdevRulesController) Name() string {
	return "files.UdevRulesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UdevRulesController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(udevdServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UdevRulesController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *UdevRulesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	defer ctrl.teardownRulesRoot() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		rules, err := ctrl.rules(ctx, r)
		if err != nil {
			return err
		}

		changed, err := ctrl.writeRules(rules, logger)
		if err != nil {
			return err
		}

		ctrl.pendingReload = ctrl.pendingReload || changed

		if ctrl.pendingReload {
			healthy, err := ctrl.udevdHealthy(ctx, r)
			if err != nil {
				return err
			}

			if healthy {
				if err = ctrl.reload(ctx); err != nil {
					return err
				}

				ctrl.pendingReload = false
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *UdevRulesController) rules(ctx context.Context, r controller.Runtime) ([]string, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get machine config: %w", err)
	}

	udevConfig := cfg.Config().UdevRulesConfig()
	if udevConfig == nil {
		return nil, nil
	}

	return udevConfig.Rules(), nil
}

func (ctrl *UdevRulesController) udevdHealthy(ctx context.Context, r controller.Runtime) (bool, error) {
	svc, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, udevdServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get udevd service state: %w", err)
	}

	return svc.TypedSpec().Running && (svc.TypedSpec().Healthy || svc.TypedSpec().Unknown), nil
}

func (ctrl *UdevRulesController) writeRules(rules []string, logger *zap.Logger) (bool, error) {
	if len(rules) == 0 {
		return false, ctrl.teardownRulesRoot()
	}

	var content strings.Builder

	for _, rule := range rules {
		content.WriteString(strings.ReplaceAll(rule, "\n", "\\\n"))
		content.WriteByte('\n')
	}

	return ctrl.writeRulesContent([]byte(content.String()), logger)
}

func (ctrl *UdevRulesController) writeRulesContent(newContent []byte, logger *zap.Logger) (bool, error) {
	if ctrl.rulesInfo == nil {
		if err := ctrl.setupRulesRoot(logger); err != nil {
			return false, fmt.Errorf("failed to setup udev rules root: %w", err)
		}
	}

	rulesFile := filepath.Base(ctrl.rulesPath())

	oldContent, err := xfs.ReadFile(ctrl.rulesInfo.root, rulesFile)
	if err == nil && bytes.Equal(oldContent, newContent) {
		return false, ctrl.setLabel()
	}

	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed reading custom udev rules: %w", err)
	}

	if err := xfs.WriteFile(ctrl.rulesInfo.root, rulesFile, newContent, 0o644); err != nil {
		return false, fmt.Errorf("failed writing custom udev rules: %w", err)
	}

	if err := ctrl.setLabel(); err != nil {
		return false, fmt.Errorf("failed labeling custom udev rules: %w", err)
	}

	return true, nil
}

func (ctrl *UdevRulesController) setupRulesRoot(logger *zap.Logger) error {
	opts := []fsopen.Option{
		fsopen.WithStringParameter("mode", "0755"),
	}

	if selinux.IsEnabled() {
		opts = append(opts, fsopen.WithStringParameter("context", constants.UdevRulesLabel))
	}

	mgr := mountv3.NewManager(
		mountv3.WithDetached(),
		mountv3.WithPrinter(logger.Sugar().Infof),
		mountv3.WithFsopen("tmpfs", opts...),
	)

	point, err := mgr.Mount()
	if err != nil {
		return fmt.Errorf("failed to mount anonymous tmpfs for udev rules: %w", err)
	}

	rulesRoot := point.Root()

	cleanup := func() {
		rulesRoot.Close() //nolint:errcheck
		mgr.Unmount()     //nolint:errcheck
	}

	udevRulesFile := filepath.Base(ctrl.rulesPath())

	if err = xfs.WriteFile(rulesRoot, udevRulesFile, nil, 0o640); err != nil {
		cleanup()

		return fmt.Errorf("failed to create custom udev rules file: %w", err)
	}

	if err = ctrl.setLabelInRoot(rulesRoot, udevRulesFile); err != nil {
		cleanup()

		return err
	}

	if err := mountv3.BindRootPath(rulesRoot, udevRulesFile, ctrl.rulesPath(), udevRulesBindAttr); err != nil {
		cleanup()

		return fmt.Errorf("failed to bind custom udev rules file: %w", err)
	}

	ctrl.rulesInfo = &rulesInfo{
		root: rulesRoot,
		umount: func() error {
			return errors.Join(
				unix.Unmount(ctrl.rulesPath(), unix.MNT_DETACH),
				rulesRoot.Close(),
				mgr.Unmount(),
			)
		},
	}

	return nil
}

func (ctrl *UdevRulesController) teardownRulesRoot() error {
	if ctrl.rulesInfo == nil {
		return nil
	}

	info := ctrl.rulesInfo
	ctrl.rulesInfo = nil

	if info.umount != nil {
		return info.umount()
	}

	return errors.Join(
		info.root.Close(),
	)
}

func (ctrl *UdevRulesController) rulesPath() string {
	if ctrl.UdevRulesPath != "" {
		return ctrl.UdevRulesPath
	}

	return constants.UdevRulesPath
}

func (ctrl *UdevRulesController) setLabel() error {
	rulesFile := filepath.Base(ctrl.rulesPath())

	return ctrl.setLabelInRoot(ctrl.rulesInfo.root, rulesFile)
}

func (ctrl *UdevRulesController) setLabelInRoot(root xfs.Root, rulesFile string) error {
	if err := selinux.FSetLabel(root, rulesFile, constants.UdevRulesLabel); err != nil {
		return fmt.Errorf("failed to label custom udev rules file: %w", err)
	}

	return nil
}

func (ctrl *UdevRulesController) reload(ctx context.Context) error {
	commandRunner := ctrl.CommandRunner
	if commandRunner == nil {
		commandRunner = func(ctx context.Context, name string, args []string) (string, error) {
			return cmd.RunWithOptions(ctx, name, args)
		}
	}

	commands := [][]string{
		{"/sbin/udevadm", "control", "--reload"},
		{"/sbin/udevadm", "trigger", "--type=devices", "--action=add"},
		{"/sbin/udevadm", "trigger", "--type=subsystems", "--action=add"},
		{"/sbin/udevadm", "settle", "--timeout=50"},
	}

	for _, command := range commands {
		if _, err := commandRunner(ctx, command[0], command[1:]); err != nil {
			return err
		}
	}

	return nil
}

var _ controller.Controller = &UdevRulesController{}
