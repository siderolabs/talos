// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta provides access to META partition: key-value partition persisted across reboots.
package meta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	goruntime "runtime"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/meta/internal/adv"
	"github.com/siderolabs/talos/internal/pkg/meta/internal/adv/syslinux"
	"github.com/siderolabs/talos/internal/pkg/meta/internal/adv/talos"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Meta represents the META reader/writer.
//
// Meta abstracts away all details about loading/storing the metadata providing an easy to use interface.
type Meta struct {
	mu sync.Mutex

	legacy adv.ADV
	talos  adv.ADV
	state  state.State
	opts   Options
}

// Options configures the META.
type Options struct {
	fixedPath string
	printer   func(string, ...any)
}

// Option is a functional option.
type Option func(*Options)

// WithFixedPath sets the fixed path to META partition.
func WithFixedPath(path string) Option {
	return func(o *Options) {
		o.fixedPath = path
	}
}

// WithPrinter sets the function to print the logs, default is log.Printf.
func WithPrinter(printer func(string, ...any)) Option {
	return func(o *Options) {
		o.printer = printer
	}
}

// New initializes empty META, trying to probe the existing META first.
func New(ctx context.Context, st state.State, opts ...Option) (*Meta, error) {
	meta := &Meta{
		state: st,
		opts: Options{
			printer: log.Printf,
		},
	}

	for _, opt := range opts {
		opt(&meta.opts)
	}

	var err error

	meta.legacy, err = syslinux.NewADV(nil)
	if err != nil {
		return nil, err
	}

	meta.talos, err = talos.NewADV(nil)
	if err != nil {
		return nil, err
	}

	err = meta.Reload(ctx)

	return meta, err
}

func (meta *Meta) getPath(ctx context.Context) (string, string, error) {
	if meta.opts.fixedPath != "" {
		return meta.opts.fixedPath, "", nil
	}

	if meta.state == nil {
		return "", "", os.ErrNotExist
	}

	metaStatus, err := block.WaitForVolumePhase(ctx, meta.state, constants.MetaPartitionLabel, block.VolumePhaseReady, block.VolumePhaseMissing, block.VolumePhaseClosed)
	if err != nil {
		return "", "", err
	}

	// add our own finalizer for the META volume to ensure it never gets removed, even in the late stages of the reboot
	if err = meta.state.AddFinalizer(ctx, metaStatus.Metadata(), constants.MetaPartitionLabel); err != nil {
		return "", "", err
	}

	if metaStatus.TypedSpec().Phase == block.VolumePhaseMissing {
		return "", "", os.ErrNotExist
	}

	if metaStatus.TypedSpec().Phase == block.VolumePhaseClosed && metaStatus.TypedSpec().MountLocation == "" {
		return "", "", os.ErrNotExist
	}

	return metaStatus.TypedSpec().MountLocation, metaStatus.TypedSpec().ParentLocation, nil
}

// Reload refreshes the META from the disk.
//
//nolint:gocyclo
func (meta *Meta) Reload(ctx context.Context) error {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	path, parentPath, err := meta.getPath(ctx)
	if err != nil {
		return err
	}

	if parentPath != "" {
		parentDev, err := blockdev.NewFromPath(parentPath)
		if err != nil {
			return err
		}

		defer parentDev.Close() //nolint:errcheck

		if err = parentDev.RetryLock(ctx, true); err != nil {
			return err
		}

		defer parentDev.Unlock() //nolint:errcheck
	}

	meta.opts.printer("META: loading from %s", path)

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if err := flock(f, unix.LOCK_SH); err != nil {
		return err
	}

	adv, err := talos.NewADV(f)
	if adv == nil && err != nil {
		// if adv is not nil, but err is nil, it might be missing ADV, ignore it
		return fmt.Errorf("failed to load Talos adv: %w", err)
	}

	legacyAdv, err := syslinux.NewADV(f)
	if err != nil {
		return fmt.Errorf("failed to load syslinux adv: %w", err)
	}

	// copy values from in-memory to on-disk version
	for _, t := range meta.talos.ListTags() {
		val, _ := meta.talos.ReadTagBytes(t)
		adv.SetTagBytes(t, val)
	}

	meta.opts.printer("META: loaded %d keys", len(adv.ListTags()))

	meta.talos = adv
	meta.legacy = legacyAdv

	return meta.syncState(ctx)
}

// syncState sync resources with adv contents.
func (meta *Meta) syncState(ctx context.Context) error {
	if meta.state == nil {
		return nil
	}

	existingTags := make(map[resource.ID]struct{})

	for _, t := range meta.talos.ListTags() {
		existingTags[runtime.MetaKeyTagToID(t)] = struct{}{}
		val, _ := meta.talos.ReadTag(t)

		if err := updateTagResource(ctx, meta.state, t, val); err != nil {
			return err
		}
	}

	items, err := meta.state.List(ctx, runtime.NewMetaKey(runtime.NamespaceName, "").Metadata())
	if err != nil {
		return err
	}

	for _, item := range items.Items {
		if _, exists := existingTags[item.Metadata().ID()]; exists {
			continue
		}

		if err = meta.state.Destroy(ctx, item.Metadata()); err != nil {
			return err
		}
	}

	return nil
}

// Flush writes the META to the disk.
//
//nolint:gocyclo
func (meta *Meta) Flush() error {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	path, parentPath, err := meta.getPath(context.TODO())
	if err != nil {
		return err
	}

	if parentPath != "" {
		parentDev, err := blockdev.NewFromPath(parentPath)
		if err != nil {
			return err
		}

		defer parentDev.Close() //nolint:errcheck

		if err = parentDev.RetryLock(context.Background(), true); err != nil {
			return err
		}

		defer parentDev.Unlock() //nolint:errcheck
	}

	meta.opts.printer("META: saving to %s", path)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if err := flock(f, unix.LOCK_EX); err != nil {
		return err
	}

	serialized, err := meta.talos.Bytes()
	if err != nil {
		return err
	}

	n, err := f.WriteAt(serialized, 0)
	if err != nil {
		return err
	}

	if n != len(serialized) {
		return fmt.Errorf("expected to write %d bytes, wrote %d", len(serialized), n)
	}

	serialized, err = meta.legacy.Bytes()
	if err != nil {
		return err
	}

	offset, err := f.Seek(-int64(len(serialized)), io.SeekEnd)
	if err != nil {
		return err
	}

	n, err = f.WriteAt(serialized, offset)
	if err != nil {
		return err
	}

	if n != len(serialized) {
		return fmt.Errorf("expected to write %d bytes, wrote %d", len(serialized), n)
	}

	meta.opts.printer("META: saved %d keys", len(meta.talos.ListTags()))

	return f.Sync()
}

// ReadTag reads a tag from the META.
func (meta *Meta) ReadTag(t uint8) (val string, ok bool) {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	val, ok = meta.talos.ReadTag(t)
	if !ok {
		val, ok = meta.legacy.ReadTag(t)
	}

	return val, ok
}

// ReadTagBytes reads a tag from the META.
func (meta *Meta) ReadTagBytes(t uint8) (val []byte, ok bool) {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	val, ok = meta.talos.ReadTagBytes(t)
	if !ok {
		val, ok = meta.legacy.ReadTagBytes(t)
	}

	return val, ok
}

// SetTag writes a tag to the META.
func (meta *Meta) SetTag(ctx context.Context, t uint8, val string) (bool, error) {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	ok := meta.talos.SetTag(t, val)

	if ok {
		err := updateTagResource(ctx, meta.state, t, val)
		if err != nil {
			return false, err
		}
	}

	return ok, nil
}

// SetTagBytes writes a tag to the META.
func (meta *Meta) SetTagBytes(ctx context.Context, t uint8, val []byte) (bool, error) {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	ok := meta.talos.SetTagBytes(t, val)

	if ok {
		err := updateTagResource(ctx, meta.state, t, string(val))
		if err != nil {
			return false, err
		}
	}

	return ok, nil
}

// DeleteTag deletes a tag from the META.
func (meta *Meta) DeleteTag(ctx context.Context, t uint8) (bool, error) {
	meta.mu.Lock()
	defer meta.mu.Unlock()

	ok := meta.talos.DeleteTag(t)
	if !ok {
		ok = meta.legacy.DeleteTag(t)
	}

	if meta.state == nil {
		return ok, nil
	}

	err := meta.state.Destroy(ctx, runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(t)).Metadata())
	if state.IsNotFoundError(err) {
		err = nil
	}

	return ok, err
}

func updateTagResource(ctx context.Context, st state.State, t uint8, val string) error {
	if st == nil {
		return nil
	}

	_, err := safe.StateUpdateWithConflicts(ctx, st, runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(t)).Metadata(), func(r *runtime.MetaKey) error {
		r.TypedSpec().Value = val

		return nil
	})
	if err == nil {
		return nil
	}

	if state.IsNotFoundError(err) {
		r := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(t))
		r.TypedSpec().Value = val

		return st.Create(ctx, r)
	}

	return err
}

func flock(f *os.File, flag int) error {
	for {
		if err := unix.Flock(int(f.Fd()), flag); !errors.Is(err, unix.EINTR) {
			return err
		}

		goruntime.KeepAlive(f)
	}
}
