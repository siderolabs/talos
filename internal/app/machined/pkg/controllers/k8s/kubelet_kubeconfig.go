// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// kubeletKubeconfigPollInterval is the maximum interval between kubelet kubeconfig
// file reads when fsnotify events are not seen. It acts as a safety net against
// missed inotify events and also covers the startup window before the kubeconfig
// directory exists.
const kubeletKubeconfigPollInterval = 30 * time.Second

// KubeletKubeconfigController watches the kubelet kubeconfig file on disk and
// exposes its content hash via the [k8s.KubeletKubeconfig] resource. Consumers
// (e.g. [NodeStatusController]) rebuild their Kubernetes clients whenever the
// hash changes, which is how we detect that a stale endpoint baked into an
// existing client should be discarded.
type KubeletKubeconfigController struct {
	// Path is the on-disk location of the kubelet kubeconfig. Defaults to
	// [constants.KubeletKubeconfig] when empty; overridable for tests.
	Path string
}

func (ctrl *KubeletKubeconfigController) path() string {
	if ctrl.Path != "" {
		return ctrl.Path
	}

	return constants.KubeletKubeconfig
}

// Name implements controller.Controller interface.
func (ctrl *KubeletKubeconfigController) Name() string {
	return "k8s.KubeletKubeconfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletKubeconfigController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletKubeconfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.KubeletKubeconfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KubeletKubeconfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	defer watcher.Close() //nolint:errcheck

	kubeconfigDir := filepath.Dir(ctrl.path())
	watchedDir := false

	// Forward fsnotify events and errors into the controller loop via QueueReconcile.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if filepath.Clean(event.Name) == ctrl.path() {
					r.QueueReconcile()
				}
			case werr, ok := <-watcher.Errors:
				if !ok {
					return
				}

				logger.Warn("fsnotify error on kubelet kubeconfig", zap.Error(werr))
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-time.After(kubeletKubeconfigPollInterval):
		}

		if !watchedDir {
			if _, statErr := os.Stat(kubeconfigDir); statErr == nil {
				if addErr := watcher.Add(kubeconfigDir); addErr != nil {
					return fmt.Errorf("failed to add %q to fsnotify watcher: %w", kubeconfigDir, addErr)
				}

				watchedDir = true
			}
		}

		r.StartTrackingOutputs()

		hash, err := hashKubeletKubeconfig(ctrl.path())
		if err != nil {
			return fmt.Errorf("failed to hash kubelet kubeconfig: %w", err)
		}

		if hash != "" {
			if err = safe.WriterModify(
				ctx, r,
				k8s.NewKubeletKubeconfig(k8s.NamespaceName, k8s.KubeletKubeconfigID),
				func(res *k8s.KubeletKubeconfig) error {
					res.TypedSpec().Hash = hash

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to update KubeletKubeconfig resource: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*k8s.KubeletKubeconfig](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup KubeletKubeconfig resource: %w", err)
		}
	}
}

// hashKubeletKubeconfig returns the hex-encoded SHA-256 hash of the file at the
// given path. If the file does not exist, it returns an empty string and nil
// error.
func hashKubeletKubeconfig(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	defer f.Close() //nolint:errcheck

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
