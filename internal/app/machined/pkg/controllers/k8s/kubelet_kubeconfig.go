// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// kubeletKubeconfigPollInterval is the maximum interval between kubelet kubeconfig
// file reads when fsnotify events are not seen. It acts as a safety net against
// missed inotify events and also covers the startup window before the kubeconfig
// directory exists.
const kubeletKubeconfigPollInterval = 30 * time.Second

// errKubeconfigNotReady is reported when the kubeconfig on disk can't be parsed.
//
// Neither kubelet nor Talos write the kubeconfig atomically, so a read triggered by an
// fsnotify event may catch a partially written file; this is a transient condition to be
// retried, not a failure (see also [conditions.WaitForKubeconfigReady]).
var errKubeconfigNotReady = errors.New("kubelet kubeconfig is not ready")

// KubeletKubeconfigController watches the kubelet client credentials on disk and
// exposes their content hash via the [k8s.KubeletKubeconfig] resource. Consumers
// (e.g. [NodeStatusController]) rebuild their Kubernetes clients whenever the
// hash changes, which is how we detect that a stale endpoint or an expired client
// certificate baked into an existing client should be discarded.
//
// The credentials are not just the kubeconfig itself: the kubeconfig points at the
// certificate kubelet rotates on its own (via the `kubelet-client-current.pem`
// symlink), and that rotation never touches the kubeconfig file.
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
	watcher, err := newCredentialsWatcher(ctrl.path())
	if err != nil {
		return err
	}

	defer watcher.Close() //nolint:errcheck

	go watcher.run(ctx, r, logger)

	// last successfully computed credentials, kept around to ride over a kubeconfig which
	// can't be parsed at the moment
	var (
		lastHash string
		lastRefs []string
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-time.After(kubeletKubeconfigPollInterval):
		}

		// Establish the watch before reading, so that a change landing while the credentials
		// are being read is not lost.
		watchAdded, err := watcher.watchDirs([]string{ctrl.path()})
		if err != nil {
			return err
		}

		hash, refs, err := kubeletCredentialsHash(ctrl.path())

		switch {
		case errors.Is(err, errKubeconfigNotReady):
			// Both kubelet and Talos rewrite the kubeconfig in place, so a read might catch a
			// partially written file: keep whatever was published so far (rather than hashing
			// a torn file and making every consumer rebuild its client for nothing), and try
			// again on the next event.
			logger.Debug("kubelet kubeconfig is not parseable, retrying", zap.Error(err))

			hash, refs = lastHash, lastRefs
		case err != nil:
			return fmt.Errorf("failed to hash kubelet credentials: %w", err)
		default:
			lastHash, lastRefs = hash, refs
		}

		paths := slices.Concat([]string{ctrl.path()}, refs)

		refsAdded, err := watcher.watchDirs(paths)
		if err != nil {
			return err
		}

		// The tracked set is swapped only once the new one is known, so events for the files
		// discovered by an earlier iteration keep queueing a reconcile in the meantime.
		watcher.setTracked(paths)

		if watchAdded || refsAdded {
			// A directory has just come under watch: re-read the credentials, as a change
			// which landed before the watch was established would otherwise go unnoticed
			// until the next poll tick.
			r.QueueReconcile()
		}

		r.StartTrackingOutputs()

		if hash != "" {
			if err = safe.WriterModify(ctx, r,
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

// credentialsWatcher watches the directories holding the kubelet credentials and queues a
// reconcile whenever one of the tracked files changes.
//
// The files don't all live in the same directory (the kubeconfig sits in /etc/kubernetes,
// while the certificates it points at live in kubelet's PKI directory), and which files to
// track is only known once the kubeconfig has been parsed, so watches are established
// incrementally via [credentialsWatcher.watchDirs], and the set of files to report on is
// swapped in via [credentialsWatcher.setTracked].
type credentialsWatcher struct {
	watcher *fsnotify.Watcher

	trackedMu sync.Mutex
	tracked   map[string]struct{}
}

// newCredentialsWatcher creates a watcher tracking the given files to start with.
func newCredentialsWatcher(paths ...string) (*credentialsWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	w := &credentialsWatcher{
		watcher: watcher,
		tracked: map[string]struct{}{},
	}

	w.setTracked(paths)

	return w, nil
}

// Close stops the watcher.
func (w *credentialsWatcher) Close() error {
	return w.watcher.Close()
}

// setTracked replaces the set of files changes are reported for.
//
// The swap is atomic with respect to [credentialsWatcher.isTracked], so a file which is
// in both the old and the new set is never seen as untracked.
func (w *credentialsWatcher) setTracked(paths []string) {
	w.trackedMu.Lock()
	defer w.trackedMu.Unlock()

	clear(w.tracked)

	for _, path := range paths {
		w.tracked[path] = struct{}{}
	}
}

// watchDirs establishes watches on the directories holding the given files.
//
// It reports whether a new directory watch was established.
//
// The set of already watched directories is queried from the watcher itself rather than
// tracked here, as the kernel drops a watch when the directory it points at goes away
// (kubelet's PKI directory is removed and recreated when the client certificate no longer
// verifies), and such a directory has to be watched anew.
func (w *credentialsWatcher) watchDirs(paths []string) (bool, error) {
	watched := map[string]struct{}{}

	for _, dir := range w.watcher.WatchList() {
		watched[dir] = struct{}{}
	}

	var added bool

	for _, path := range paths {
		dir := filepath.Dir(path)

		if _, ok := watched[dir]; ok {
			continue
		}

		if _, err := os.Stat(dir); err != nil {
			// the directory is not there yet, retry on the next poll tick
			continue
		}

		if err := w.watcher.Add(dir); err != nil {
			return false, fmt.Errorf("failed to add %q to fsnotify watcher: %w", dir, err)
		}

		watched[dir] = struct{}{}

		added = true
	}

	return added, nil
}

func (w *credentialsWatcher) isTracked(path string) bool {
	w.trackedMu.Lock()
	defer w.trackedMu.Unlock()

	_, ok := w.tracked[path]

	return ok
}

// run forwards fsnotify events and errors into the controller loop until the context is
// canceled.
func (w *credentialsWatcher) run(ctx context.Context, r controller.Runtime, logger *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if w.isTracked(filepath.Clean(event.Name)) {
				r.QueueReconcile()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			logger.Warn("fsnotify error on kubelet credentials", zap.Error(err))
		}
	}
}

// kubeletCredentialsHash returns the hex-encoded SHA-256 hash of the kubelet
// kubeconfig combined with the certificates it references on disk, along with the
// list of the referenced files.
//
// If the kubeconfig does not exist, it returns an empty hash and no error.
func kubeletCredentialsHash(path string) (string, []string, error) {
	kubeconfig, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil
		}

		return "", nil, err
	}

	refs, err := referencedFiles(path, kubeconfig)
	if err != nil {
		return "", nil, err
	}

	hash := sha256.New()

	hash.Write(kubeconfig) //nolint:errcheck

	for _, ref := range refs {
		certs, err := certificatesFromFile(ref)
		if err != nil {
			return "", nil, err
		}

		// the path is part of the digest as well, so that repointing the kubeconfig to a
		// different (but identical) file is still seen as a change
		hash.Write([]byte(ref)) //nolint:errcheck
		hash.Write(certs)       //nolint:errcheck
	}

	return hex.EncodeToString(hash.Sum(nil)), refs, nil
}

// referencedFiles returns the sorted list of PKI files the kubeconfig points at.
//
// Relative paths are resolved against the directory holding the kubeconfig, matching
// the way client-go loads them.
func referencedFiles(kubeconfigPath string, kubeconfig []byte) ([]string, error) {
	cfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse kubeconfig %q: %w", errKubeconfigNotReady, kubeconfigPath, err)
	}

	// [clientcmd.Load] short-circuits on empty input and returns an empty config with no error,
	// and a truncated file may still parse into a config which is missing the sections we hash.
	// A kubeconfig is rewritten in place (`O_TRUNC`, which itself wakes the fsnotify watch), so a
	// read is quite likely to catch it at zero length: treat that as "not ready" as well, rather
	// than publishing the hash of an empty file.
	if cfg.CurrentContext == "" || len(cfg.AuthInfos) == 0 || len(cfg.Clusters) == 0 {
		return nil, fmt.Errorf("%w: kubeconfig %q has no credentials", errKubeconfigNotReady, kubeconfigPath)
	}

	base := filepath.Dir(kubeconfigPath)

	var refs []string

	appendRef := func(path string) {
		if path == "" {
			return
		}

		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}

		refs = append(refs, filepath.Clean(path))
	}

	for _, authInfo := range cfg.AuthInfos {
		appendRef(authInfo.ClientCertificate)
		appendRef(authInfo.ClientKey)
	}

	for _, cluster := range cfg.Clusters {
		appendRef(cluster.CertificateAuthority)
	}

	slices.Sort(refs)

	return slices.Compact(refs), nil
}

// certificatesFromFile returns the DER bytes of the certificates found in the PEM file.
//
// Only certificates contribute to the digest: kubelet keeps the client key in the very
// same file as the client certificate (`kubelet-client-current.pem`), and the digest is
// exposed via the resource API, so private key material is deliberately left out.
//
// A file which doesn't exist (yet) contributes nothing.
func certificatesFromFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var certs []byte

	for {
		var block *pem.Block

		block, raw = pem.Decode(raw)
		if block == nil {
			return certs, nil
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		certs = append(certs, block.Bytes...)
	}
}
