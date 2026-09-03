// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/foxboron/go-uefi/efi"
	"github.com/fsnotify/fsnotify"
	"github.com/siderolabs/gen/panicsafe"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// kernelLockdownPath is the securityfs file exposing the kernel lockdown level.
const kernelLockdownPath = "/sys/kernel/security/lockdown"

// lockdownPollInterval is the maximum interval between kernel lockdown level reads when
// fsnotify events are not seen. It acts as a safety net against missed inotify events.
const lockdownPollInterval = 30 * time.Second

// SecurityStateController is a controller that updates the security state of Talos.
type SecurityStateController struct {
	V1Alpha1Mode machineruntime.Mode

	// LockdownPath is the on-disk location of the securityfs kernel lockdown file.
	//
	// Defaults to [kernelLockdownPath] when empty; overridable for tests.
	LockdownPath string
}

func (ctrl *SecurityStateController) lockdownPath() string {
	if ctrl.LockdownPath != "" {
		return ctrl.LockdownPath
	}

	return kernelLockdownPath
}

// Name implements controller.Controller interface.
func (ctrl *SecurityStateController) Name() string {
	return "runtime.SecurityStateController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SecurityStateController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SecurityStateController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.SecurityStateType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// Most of the security state is fixed at boot time and is therefore only gathered once, but
// the kernel lockdown level can still be raised at runtime by anything holding CAP_SYS_ADMIN,
// so it is watched for the lifetime of the controller.
func (ctrl *SecurityStateController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := waitForMachined(ctx, r); err != nil {
		return err
	}

	// in container mode the FIPS mode of the running binary is the only part of the security
	// state which describes the container itself, the rest belongs to the host
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return publishSecurityState(ctx, r, runtimeres.SecurityStateSpec{FIPSState: getFIPSState()})
	}

	spec, err := bootTimeState()
	if err != nil {
		return err
	}

	return ctrl.watchLockdownState(ctx, r, logger, spec)
}

// waitForMachined waits for the `machined` service to start, as by that time initial mounts
// will be done.
func waitForMachined(ctx context.Context, r controller.Runtime) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.EventCh():
		}

		_, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "machined")

		switch {
		case err == nil:
			return nil
		case state.IsNotFoundError(err):
		default:
			return fmt.Errorf("failed to get machined state: %w", err)
		}
	}
}

// bootTimeState gathers the parts of the security state which are fixed for the lifetime of
// the running kernel.
//
//nolint:gocyclo
func bootTimeState() (runtimeres.SecurityStateSpec, error) {
	var spec runtimeres.SecurityStateSpec

	if efi.GetSecureBoot() && !efi.GetSetupMode() {
		spec.SecureBoot = true
	}

	defaultEntry, err := sdboot.ReadVariable(sdboot.LoaderEntryDefaultName)
	if err == nil {
		if strings.HasPrefix(defaultEntry, "Talos-") {
			spec.BootedWithUKI = true
		}
	}

	// if defaultEntry is empty in the case when we booted off a disk image when installer never runs, we can rely on the
	// stub image identifier to determine if we booted with UKI
	if defaultEntry == "" {
		stubImageIdentifier, err := sdboot.ReadVariable(sdboot.StubImageIdentifierName)
		if err == nil {
			if strings.HasPrefix(filepath.Base(strings.ReplaceAll(stubImageIdentifier, "\\", "/")), "Talos-") {
				spec.BootedWithUKI = true
			}
		}
	}

	if pcrPublicKeyData, err := os.ReadFile(constants.PCRPublicKey); err == nil {
		block, _ := pem.Decode(pcrPublicKeyData)
		if block == nil {
			return spec, errors.New("failed to decode PEM block for PCR public key")
		}

		cert := x509.Certificate{
			Raw: block.Bytes,
		}

		spec.PCRSigningKeyFingerprint = x509CertFingerprint(cert)
	}

	spec.SELinuxState, err = getSelinuxState()
	if err != nil {
		return spec, fmt.Errorf("failed to get SELinux state: %w", err)
	}

	moduleSignatureEnforcedInfo := procfs.ProcCmdline().Get(constants.KernelParamEnforceModuleSigVerify).First()
	if moduleSignatureEnforcedInfo != nil && *moduleSignatureEnforcedInfo == "1" {
		spec.ModuleSignatureEnforced = true
	}

	spec.FIPSState = getFIPSState()

	return spec, nil
}

// watchLockdownState publishes the security state, keeping the kernel lockdown level in it up
// to date for the lifetime of the controller.
//
//nolint:gocyclo
func (ctrl *SecurityStateController) watchLockdownState(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	spec runtimeres.SecurityStateSpec,
) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	defer watcher.Close() //nolint:errcheck

	// securityfs is mounted by the time `machined` is up, and the level is only ever raised by a
	// write(2) to this very file, which the kernel reports as a plain modification: watching the
	// file itself is enough, as the inode is never replaced.
	//
	// the file is only missing when the lockdown LSM is disabled, in which case the kernel can
	// never be locked down and there is nothing to watch.
	switch err = watcher.Add(ctrl.lockdownPath()); {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return publishSecurityState(ctx, r, spec)
	default:
		return fmt.Errorf("failed to watch %q: %w", ctrl.lockdownPath(), err)
	}

	watchCtx, cancelWatch := context.WithCancel(ctx)

	var fsnotifyWg sync.WaitGroup

	// the watch goroutine only returns once its context is done, so it has to be canceled before
	// waiting for it: this function can return while `ctx` is still alive, e.g. on a failed read
	defer func() {
		cancelWatch()

		fsnotifyWg.Wait()
	}()

	fsnotifyWg.Go(func() {
		if notifyErr := panicsafe.RunErr(func() error { return queueOnLockdownChange(watchCtx, watcher, r) }); notifyErr != nil {
			logger.Error("kernel lockdown watch failed", zap.Error(notifyErr))
		}
	})

	for {
		// the level is read after the watch has been established, so that a change landing while
		// it is being read queues another reconcile instead of being missed
		if spec.LockdownState, err = readLockdownState(ctrl.lockdownPath()); err != nil {
			return fmt.Errorf("failed to get kernel lockdown state: %w", err)
		}

		if err = publishSecurityState(ctx, r, spec); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-time.After(lockdownPollInterval):
		}
	}
}

// publishSecurityState updates the security state resource.
func publishSecurityState(ctx context.Context, r controller.Runtime, spec runtimeres.SecurityStateSpec) error {
	return safe.WriterModify(ctx, r, runtimeres.NewSecurityStateSpec(runtimeres.NamespaceName), func(res *runtimeres.SecurityState) error {
		*res.TypedSpec() = spec

		return nil
	})
}

// queueOnLockdownChange queues a reconcile whenever the watched kernel lockdown file is modified.
func queueOnLockdownChange(ctx context.Context, watcher *fsnotify.Watcher, r controller.Runtime) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			r.QueueReconcile()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}

			return err
		}
	}
}

// readLockdownState reads the current kernel lockdown level from securityfs.
//
// The file lists every level with the active one in brackets, e.g.:
//
//	[none] integrity confidentiality
func readLockdownState(path string) (runtimeres.LockdownState, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return runtimeres.LockdownStateNone, err
	}

	for field := range strings.FieldsSeq(string(contents)) {
		level, ok := strings.CutPrefix(field, "[")
		if !ok {
			continue
		}

		return runtimeres.LockdownStateString(strings.TrimSuffix(level, "]"))
	}

	return runtimeres.LockdownStateNone, fmt.Errorf("failed to parse kernel lockdown level %q", string(contents))
}

func x509CertFingerprint(cert x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)

	var buf bytes.Buffer

	for i, b := range hex.EncodeToString(hash[:]) {
		if i > 0 && i%2 == 0 {
			buf.WriteByte(':')
		}

		buf.WriteString(strings.ToUpper(string(b)))
	}

	return buf.String()
}

// getFIPSState reports the FIPS mode the running binary was built for.
func getFIPSState() runtimeres.FIPSState {
	switch {
	case fipsmode.Strict(): // implies fipsmode.Enabled()
		return runtimeres.FIPSStateStrict
	case fipsmode.Enabled():
		return runtimeres.FIPSStateEnabled
	default:
		return runtimeres.FIPSStateDisabled
	}
}

func getSelinuxState() (runtimeres.SELinuxState, error) {
	if !selinux.IsEnabled() {
		return runtimeres.SELinuxStateDisabled, nil
	}

	// Read /sys/fs/selinux/enforce to determine if SELinux is in enforcing mode
	// Make sure LSM mode is actually enforcing, in case we later allow setenforce
	// IsEnabled is reliable, since LSM is active whenever SELinuxFS is mounted, which is done accordingly
	data, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return runtimeres.SELinuxStateDisabled, err
	}

	if strings.TrimSpace(string(data)) == "1" {
		return runtimeres.SELinuxStateEnforcing, nil
	}

	return runtimeres.SELinuxStatePermissive, nil
}
