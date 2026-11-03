// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/opencontainers/go-digest"
)

// ErrCopySourceChanged identifies a source identity race that a fresh attempt can retry.
var ErrCopySourceChanged = errors.New("copy source changed")

// CopyPhase identifies a copy boundary; it does not report durable readiness.
type CopyPhase string

// Phase boundaries precede their work; transfer updates also follow each write.
const (
	CopyPhaseTransfer CopyPhase = "transfer"
	CopyPhaseVerify   CopyPhase = "verify"
	CopyPhaseSync     CopyPhase = "sync"
	CopyPhaseValidate CopyPhase = "validate"
	CopyPhasePublish  CopyPhase = "publish"
	CopyPhaseCleanup  CopyPhase = "cleanup"
)

// CopyProgress is a synchronous observation owned by the operation's worker.
// TotalKnown distinguishes an empty regular file from an unknown stream length.
// Verify finalizes the digest computed inline; it does not reread the source.
type CopyProgress struct {
	Phase       CopyPhase
	BytesCopied uint64
	TotalBytes  uint64
	TotalKnown  bool
	StartedAt   time.Time
	Elapsed     time.Duration
}

// SetProgressObserver installs a worker-owned callback before Transfer. The callback
// must not call back into the operation. Callers may throttle transfer observations.
// Cleanup is a boundary, not terminal success: the caller reports outcome only after
// FinishCopy/Close and its own binding publication and dependency cleanup return.
func (op *CopyOperation) SetProgressObserver(observer func(CopyProgress)) {
	op.observer = observer
}

func (op *CopyOperation) observe(phase CopyPhase) {
	if op.observer == nil {
		return
	}

	now := time.Now()
	if op.progress.StartedAt.IsZero() {
		op.progress.StartedAt = now
	}

	op.progress.Phase = phase
	op.progress.Elapsed = now.Sub(op.progress.StartedAt)
	op.observer(op.progress)
}

func (op *CopyOperation) write(ctx context.Context, writer io.Writer, buffer []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	n, err := writer.Write(buffer)
	if op.observer != nil {
		op.progress.BytesCopied += uint64(n)
		op.observe(CopyPhaseTransfer)
	}

	return n, err
}

// CopyOperation carries a private stage between short daemon sessions.
// The worker owns the operation and must Close it on every exit path.
type CopyOperation struct {
	Name     string
	Source   string
	Expected string

	root     *os.Root
	backing  os.FileInfo
	target   string
	staged   string
	witness  string
	release  func()
	observer func(CopyProgress)
	progress CopyProgress
}

// Close removes the private stage before releasing its sweep protection.
func (op *CopyOperation) Close() {
	if op == nil || op.root == nil {
		return
	}

	op.observe(CopyPhaseCleanup)

	op.root.Remove(op.staged) //nolint:errcheck
	op.root.Close()           //nolint:errcheck
	op.release()
	op.root = nil
}

// PrepareCopy observes the pool and reserves a stage without copying bytes.
func (c *Client) PrepareCopy(pool Pool, name, source, expected string) (*CopyOperation, error) {
	if err := validateCopyName(name); err != nil {
		return nil, err
	}

	_, target, err := c.poolDirectory(pool)
	if err != nil {
		return nil, err
	}

	staged := stagedVolumeName(name)
	release := protectStage(target, staged)

	root, err := os.OpenRoot(target)
	if err != nil {
		release()

		return nil, err
	}

	backing, err := root.Stat(".")
	if err != nil {
		root.Close() //nolint:errcheck
		release()

		return nil, err
	}

	if err = root.Mkdir(".copy-on-use", 0o700); err != nil && !os.IsExist(err) {
		root.Close() //nolint:errcheck
		release()

		return nil, err
	}

	return &CopyOperation{
		Name:     name,
		Source:   source,
		Expected: expected,

		root:    root,
		backing: backing,
		target:  target,
		staged:  staged,
		witness: fmt.Sprintf(".copy-on-use/%x", sha256.Sum256([]byte(name))),
		release: release,
	}, nil
}

// Transfer copies and verifies bytes with no libvirt connection or session deadline.
func (op *CopyOperation) Transfer(ctx context.Context) error {
	if op == nil || op.root == nil {
		return fmt.Errorf("copy stage is closed")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	input, original, err := openCopySource(op.Source)
	if err != nil {
		return err
	}
	defer input.Close() //nolint:errcheck

	if op.observer != nil {
		op.progress = CopyProgress{TotalBytes: uint64(original.Size()), TotalKnown: true}
		op.observe(CopyPhaseTransfer)
	}

	if err = op.writeCopyContext(ctx, input); err != nil {
		return err
	}

	current, err := os.Lstat(op.Source)
	if err != nil {
		return err
	}

	if !os.SameFile(original, current) || current.Size() != original.Size() {
		return fmt.Errorf("%w: %q changed during transfer", ErrCopySourceChanged, op.Source)
	}

	return ctx.Err()
}

func openCopySource(source string) (*os.File, os.FileInfo, error) {
	original, err := os.Lstat(source)
	if err != nil {
		return nil, nil, err
	}

	if !original.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("copy source %q is not a regular file", source)
	}

	input, err := os.Open(source)
	if err != nil {
		return nil, nil, err
	}

	opened, err := input.Stat()
	if err != nil {
		return nil, nil, errors.Join(err, input.Close())
	}

	if !os.SameFile(original, opened) || original.Size() != opened.Size() {
		return nil, nil, errors.Join(fmt.Errorf("%w: %q changed while opening", ErrCopySourceChanged, source), input.Close())
	}

	return input, opened, nil
}

// These function adapters expose no ReaderFrom or WriterTo fast path: every chunk
// passes the cancellation checks, including file-to-file transfers on Linux.
type copyReadFunc func([]byte) (int, error)

func (read copyReadFunc) Read(buffer []byte) (int, error) {
	return read(buffer)
}

type copyWriteFunc func([]byte) (int, error)

func (write copyWriteFunc) Write(buffer []byte) (int, error) {
	return write(buffer)
}

func (op *CopyOperation) writeCopyContext(ctx context.Context, input io.Reader) error {
	verifier, err := copyVerifier(op.Expected)
	if err != nil {
		return err
	}

	output, err := op.root.OpenFile(op.staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, volumeMode)
	if err != nil {
		return err
	}

	var writer io.Writer = output
	if verifier != nil {
		writer = io.MultiWriter(output, verifier)
	}

	reader := copyReadFunc(func(buffer []byte) (int, error) {
		if readErr := ctx.Err(); readErr != nil {
			return 0, readErr
		}

		return input.Read(buffer)
	})
	checkedWriter := copyWriteFunc(func(buffer []byte) (int, error) {
		return op.write(ctx, writer, buffer)
	})

	_, copyErr := io.CopyBuffer(checkedWriter, reader, make([]byte, 128<<10))
	if copyErr == nil {
		copyErr = ctx.Err()
	}

	if copyErr == nil {
		op.observe(CopyPhaseVerify)
		copyErr = verifyCopyDigest(verifier, op.Expected)
	}

	if copyErr == nil {
		op.observe(CopyPhaseSync)

		copyErr = output.Sync()
	}

	return errors.Join(copyErr, output.Close())
}

func copyVerifier(expected string) (digest.Verifier, error) {
	if expected == "" {
		return nil, nil
	}

	parsed, err := digest.Parse(expected)
	if err != nil {
		return nil, err
	}

	return parsed.Verifier(), nil
}

func verifyCopyDigest(verifier digest.Verifier, expected string) error {
	if verifier != nil && !verifier.Verified() {
		return fmt.Errorf("copy source digest mismatch: expected %s", expected)
	}

	return nil
}

// FinishCopy uses a fresh session to validate the stage and rechecks owner intent
// immediately before the atomic links. It never replaces an existing destination.
func (c *Client) FinishCopy(ctx context.Context, pool Pool, op *CopyOperation, validate func() error) (Volume, error) {
	if op == nil || op.root == nil {
		return Volume{}, fmt.Errorf("copy stage is closed")
	}
	defer op.Close()

	if err := ctx.Err(); err != nil {
		return Volume{}, err
	}

	op.observe(CopyPhaseValidate)

	p, err := c.copyPool(pool, op)
	if err != nil {
		return Volume{}, err
	}

	if err = validate(); err != nil {
		return Volume{}, err
	}

	if err = ctx.Err(); err != nil {
		return Volume{}, err
	}

	op.observe(CopyPhasePublish)

	if err = publishCopy(op.root, op.staged, op.Name, op.witness); err != nil {
		return Volume{}, err
	}

	return c.copiedVolume(p, op.Name)
}

func (c *Client) copyPool(pool Pool, op *CopyOperation) (libvirt.StoragePool, error) {
	p, target, err := c.poolDirectory(pool)
	if err != nil {
		return p, err
	}

	backing, err := os.Stat(target)
	if err != nil {
		return p, err
	}

	if filepath.Clean(target) != filepath.Clean(op.target) || !os.SameFile(backing, op.backing) {
		return p, fmt.Errorf("copy pool backing changed")
	}

	if _, err = c.copiedVolume(p, op.staged); err != nil {
		return p, fmt.Errorf("validate staged copy: %w", err)
	}

	return p, nil
}

// CopyCache contains ownership and durability proofs for one controller run.
// A new run must construct a fresh cache so its first adoption restores durability.
type CopyCache struct {
	entries map[string]copyProof
}

type copyProof struct {
	file    os.FileInfo
	backing os.FileInfo
	target  string
}

// Forget invalidates the proof when the request's intent changes.
func (cache *CopyCache) Forget(pool Pool, name string) {
	if cache != nil {
		delete(cache.entries, pool.UUID.String()+"/"+name)
	}
}

// AdoptCopy checks ownership, backing identity and libvirt observation every pass.
// A first adoption restores the witness and directory barriers before caching.
func (c *Client) AdoptCopy(pool Pool, name string, cache *CopyCache) (Volume, bool, error) {
	if err := validateCopyName(name); err != nil {
		return Volume{}, false, err
	}

	p, target, err := c.poolDirectory(pool)
	if err != nil {
		cache.Forget(pool, name)

		return Volume{}, false, err
	}

	witness := fmt.Sprintf(".copy-on-use/%x", sha256.Sum256([]byte(name)))
	if volume, checked, cacheErr := c.cachedCopy(p, pool, target, name, witness, cache); checked {
		return volume, true, cacheErr
	}

	return c.durableCopy(p, pool, target, name, witness, cache)
}

func (c *Client) cachedCopy(p libvirt.StoragePool, pool Pool, target, name, witness string, cache *CopyCache) (Volume, bool, error) {
	if cache == nil {
		return Volume{}, false, nil
	}

	proof, ok := cache.entries[pool.UUID.String()+"/"+name]
	if !ok {
		return Volume{}, false, nil
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		cache.Forget(pool, name)

		return Volume{}, true, err
	}
	defer root.Close() //nolint:errcheck

	matches, err := proof.matches(root, target, name, witness)
	if err != nil || !matches {
		cache.Forget(pool, name)

		return Volume{}, err != nil, err
	}

	volume, err := c.copiedVolume(p, name)
	if err != nil {
		cache.Forget(pool, name)
	}

	return volume, true, err
}

func (proof copyProof) matches(root *os.Root, target, name, witness string) (bool, error) {
	exists, err := checkCopyOwnership(root, name, witness)
	if err != nil || !exists {
		return false, err
	}

	file, err := root.Lstat(name)
	if err != nil {
		return false, err
	}

	backing, err := root.Stat(".")
	if err != nil {
		return false, err
	}

	return filepath.Clean(proof.target) == filepath.Clean(target) &&
		os.SameFile(file, proof.file) && os.SameFile(backing, proof.backing), nil
}

func (c *Client) durableCopy(p libvirt.StoragePool, pool Pool, target, name, witness string, cache *CopyCache) (Volume, bool, error) {
	if err := sweepStagedVolumes(target); err != nil {
		return Volume{}, false, err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return Volume{}, false, err
	}
	defer root.Close() //nolint:errcheck

	if err = root.Mkdir(".copy-on-use", 0o700); err != nil && !os.IsExist(err) {
		return Volume{}, false, err
	}

	volume, adopted, err := c.adoptCopy(p, root, name, witness)
	if err != nil || !adopted {
		return volume, adopted, err
	}

	file, err := root.Lstat(name)
	if err != nil {
		return Volume{}, false, err
	}

	backing, err := root.Stat(".")
	if err != nil {
		return Volume{}, false, err
	}

	cache.remember(pool, name, copyProof{
		file:    file,
		backing: backing,
		target:  target,
	})

	return volume, true, nil
}

func (cache *CopyCache) remember(pool Pool, name string, proof copyProof) {
	if cache == nil {
		return
	}

	if cache.entries == nil {
		cache.entries = make(map[string]copyProof)
	}

	cache.entries[pool.UUID.String()+"/"+name] = proof
}

// CopyBindingLabel identifies stable VM/disk/configuration intent on a copy request and its disk status.
const CopyBindingLabel = "storage.talos.dev/copy-binding"

// CopyBindingPublicationFinalizer fences disk-intent CAS updates during durable publication.
const CopyBindingPublicationFinalizer = "storage.StoragePoolVolumeController"

// CopyBindings remembers durability acknowledgments only for this controller run.
// Its zero value is ready for use by a single reconciliation owner.
type CopyBindings struct {
	proven map[string]copyBindingProof
}

type copyBindingProof struct {
	root    os.FileInfo
	private os.FileInfo
	record  os.FileInfo
}

func bindingPath(key string) (string, error) {
	if len(key) != 64 || strings.Trim(key, "0123456789abcdef") != "" {
		return "", fmt.Errorf("invalid copy binding key %q", key)
	}

	return filepath.Join(".copy-on-use", "binding-"+key), nil
}

func ownedCopy(root *os.Root, name string) error {
	if err := validateCopyName(name); err != nil {
		return err
	}

	witness := fmt.Sprintf(".copy-on-use/%x", sha256.Sum256([]byte(name)))

	exists, err := checkCopyOwnership(root, name, witness)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("copy %q is not published", name)
	}

	return nil
}

// CopyPublicationAbsent proves both publication paths are missing, not merely unobservable.
func CopyPublicationAbsent(target, name string) (bool, error) {
	if err := validateCopyName(name); err != nil {
		return false, err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return false, err
	}
	defer root.Close() //nolint:errcheck

	for _, path := range []string{name, fmt.Sprintf(".copy-on-use/%x", sha256.Sum256([]byte(name)))} {
		_, err := root.Lstat(path)
		if err == nil {
			return false, nil
		}

		if !os.IsNotExist(err) {
			return false, err
		}
	}

	return true, nil
}

// ReadCopyBinding selects only an explicitly recorded completed, owned destination.
// Missing bindings return an empty name; malformed, stale and foreign records fail closed.
func ReadCopyBinding(target, key string) (string, error) {
	path, err := bindingPath(key)
	if err != nil {
		return "", err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck

	name, _, err := readBindingRecord(root, path)
	if err != nil || name == "" {
		return "", err
	}

	if err := ownedCopy(root, name); err != nil {
		return "", fmt.Errorf("invalid copy binding: %w", err)
	}

	return name, nil
}

func readBindingRecord(root *os.Root, path string) (string, os.FileInfo, error) {
	before, err := root.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil, nil
	}

	if err != nil {
		return "", nil, err
	}

	if !before.Mode().IsRegular() || before.Size() == 0 || before.Size() > 255 {
		return "", nil, errors.New("invalid copy binding record")
	}

	return readBindingFile(root, path, before)
}

func readBindingFile(root *os.Root, path string, before os.FileInfo) (string, os.FileInfo, error) {
	file, err := root.Open(path)
	if err != nil {
		return "", nil, err
	}

	info, statErr := file.Stat()

	data, readErr := io.ReadAll(io.LimitReader(file, 256))
	if err := errors.Join(statErr, readErr, file.Close()); err != nil {
		return "", nil, err
	}

	if !os.SameFile(before, info) || len(data) == 0 || len(data) > 255 {
		return "", nil, errors.New("copy binding record changed while reading")
	}

	if err := validateCopyName(string(data)); err != nil {
		return "", nil, err
	}

	return string(data), info, nil
}

// Record durably selects a completed owned copy. Cold observations reestablish directory
// barriers; unchanged, acknowledged root/private-directory/record identities need no writes.
func (bindings *CopyBindings) Record(target, key, name string) error {
	path, err := bindingPath(key)
	if err != nil {
		return err
	}

	root, proof, err := openBindingRoot(target, name)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	return bindings.record(root, target+"\x00"+key, path, key, name, proof)
}

func openBindingRoot(target, name string) (*os.Root, copyBindingProof, error) {
	root, err := os.OpenRoot(target)
	if err != nil {
		return nil, copyBindingProof{}, err
	}

	if err := ownedCopy(root, name); err != nil {
		return nil, copyBindingProof{}, errors.Join(err, root.Close())
	}

	rootInfo, err := root.Stat(".")
	if err != nil {
		return nil, copyBindingProof{}, errors.Join(err, root.Close())
	}

	privateInfo, err := root.Lstat(".copy-on-use")
	if err != nil {
		return nil, copyBindingProof{}, errors.Join(err, root.Close())
	}

	if !privateInfo.IsDir() {
		return nil, copyBindingProof{}, errors.Join(errors.New("copy binding directory is not a directory"), root.Close())
	}

	return root, copyBindingProof{
		root:    rootInfo,
		private: privateInfo,
	}, nil
}

func (bindings *CopyBindings) acknowledged(key string, proof copyBindingProof) bool {
	previous, ok := bindings.proven[key]

	return ok && os.SameFile(proof.root, previous.root) && os.SameFile(proof.private, previous.private) &&
		os.SameFile(proof.record, previous.record)
}

func (bindings *CopyBindings) record(root *os.Root, cacheKey, path, key, name string, proof copyBindingProof) error {
	previous, recordInfo, err := readBindingRecord(root, path)
	if err != nil {
		return err
	}

	proof.record = recordInfo
	if previous == name && bindings.acknowledged(cacheKey, proof) {
		return nil
	}

	if previous != name {
		if err := writeBindingRecord(root, path, key, name); err != nil {
			return err
		}

		proof.record, err = root.Lstat(path)
		if err != nil {
			return err
		}
	}

	if err := syncCopyDirectory(root, ".copy-on-use"); err != nil {
		return err
	}

	if err := syncCopyDirectory(root, "."); err != nil {
		return err
	}

	if bindings.proven == nil {
		bindings.proven = make(map[string]copyBindingProof)
	}

	bindings.proven[cacheKey] = proof

	return nil
}

func writeBindingRecord(root *os.Root, path, key, name string) error {
	// stagedVolumeName includes a fresh random nonce on every attempt, including retries.
	staged := ".copy-on-use/" + stagedVolumeName("binding-"+key)

	file, err := root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(staged) //nolint:errcheck

	_, err = file.WriteString(name)
	if err == nil {
		err = file.Sync()
	}

	if err := errors.Join(err, file.Close()); err != nil {
		return err
	}

	return root.Rename(staged, path)
}
