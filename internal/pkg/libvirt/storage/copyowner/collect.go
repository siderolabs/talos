// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copyowner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type cleanupBinding struct {
	key, name, path string
	info            os.FileInfo
}

type cleanupOwner struct {
	key, name, path string
	info            os.FileInfo
}

// OwnerMatches reports whether name's owner record binds key to witness's inode.
func OwnerMatches(root *os.Root, key, name string, witness os.FileInfo) bool {
	return ownerWitnessMatches(root, cleanupOwner{key: key, name: name, path: OwnerPath(name)}, witness)
}

func ownerWitnessMatches(root *os.Root, owner cleanupOwner, witness os.FileInfo) bool {
	file, err := root.Open(owner.path)
	if err != nil {
		return false
	}

	data, readErr := io.ReadAll(io.LimitReader(file, maxCopyOwnerSize+1))
	if errors.Join(readErr, file.Close()) != nil {
		return false
	}

	return string(data) == owner.key+"\n"+owner.name+"\n"+copyInode(witness)+"\n"
}

// Collect reclaims only proven private artifacts. Public payloads are
// never removed; unknown private entries (including unbound legacy copies) stay.
func Collect(ctx context.Context, target string, keep Retention) error {
	return Collector{}.Collect(ctx, target, keep)
}

// Collector configures collection. The zero value uses Sync for every directory barrier.
type Collector struct {
	// Sync makes one directory's entries durable; nil selects Sync.
	Sync func(root *os.Root, dir string) error
}

// Collect is Collect with the collector's directory barrier.
func (c Collector) Collect(ctx context.Context, target string, keep Retention) error {
	sync := c.Sync
	if sync == nil {
		sync = Sync
	}

	return collectCopies(ctx, target, keep, sync)
}

func collectCopies(ctx context.Context, target string, keep Retention, syncDirectory func(*os.Root, string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := SweepStages(target); err != nil {
		return err
	}

	defer lockPool(target)()

	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	census, err := censusCopies(root, target, keep)
	if err != nil {
		return err
	}

	if census == nil {
		return nil
	}

	return census.collect(ctx, root, keep, syncDirectory)
}

// Collectable is a read-only census reporting whether Collect has
// anything to retire, so callers can skip holding an idle pool. It is advisory:
// Collect repeats the census against a held backing.
func Collectable(target string, keep Retention) (bool, error) {
	defer lockPool(target)()

	root, err := os.OpenRoot(target)
	if err != nil {
		return false, err
	}
	defer root.Close() //nolint:errcheck

	census, err := censusCopies(root, target, keep)
	if err != nil {
		return false, err
	}

	if idle, err := idleStages(root, target, census); err != nil || idle {
		return idle, err
	}

	if census == nil {
		return false, nil
	}

	return len(census.removableNames(root, keep)) > 0 || len(census.standaloneMarkers()) > 0, nil
}

// idleStages reports a public or private stage which no live operation protects.
func idleStages(root *os.Root, target string, census *copyCensus) (bool, error) {
	public, err := readCopyEntries(root, ".")
	if err != nil {
		return false, err
	}

	for _, entry := range public {
		if isStagedVolumeEntry(entry) && !stageActive(target, entry.Name()) {
			return true, nil
		}
	}

	if census == nil {
		return false, nil
	}

	for _, entry := range census.entries {
		if stage, ok := privateStage(entry); ok && !stageActive(target, stage) {
			return true, nil
		}
	}

	return false, nil
}

type copyCensus struct {
	entries        []os.DirEntry
	protected      map[string]struct{}
	activeBindings map[string]struct{}
	bindings       []cleanupBinding
	owners         []cleanupOwner
	completed      map[string]string
}

func readCopyEntries(root *os.Root, path string) ([]os.DirEntry, error) {
	dir, err := root.Open(path)
	if err != nil {
		return nil, err
	}

	entries, err := dir.ReadDir(-1)

	return entries, errors.Join(err, dir.Close())
}

func censusCopies(root *os.Root, target string, keep Retention) (*copyCensus, error) {
	private, err := root.Lstat(".copy-on-use")
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if !private.IsDir() {
		return nil, errors.New("copy ownership directory is not a directory")
	}

	entries, err := readCopyEntries(root, ".copy-on-use")
	if err != nil {
		return nil, err
	}

	publicEntries, err := readCopyEntries(root, ".")
	if err != nil {
		return nil, err
	}

	c := &copyCensus{
		entries:        entries,
		protected:      make(map[string]struct{}, len(keep.Volumes)+len(publicEntries)),
		activeBindings: make(map[string]struct{}),
		completed:      make(map[string]string),
	}
	for name := range keep.Volumes {
		c.protect(name)
	}

	for _, entry := range publicEntries {
		c.protect(entry.Name()) // Any public entry protects its name, including foreign/symlink entries.
	}

	c.protectActiveStages(target)
	c.readRecords(root)
	c.protectRecords(root, keep)

	return c, nil
}

func (c *copyCensus) protect(name string) {
	if ValidateName(name) == nil {
		c.protected[name] = struct{}{}
	}
}

func (c *copyCensus) protectActiveStages(target string) {
	for _, stage := range activeStagesIn(target) {
		if !IsStageName(stage) {
			continue
		}

		body := strings.TrimSuffix(strings.TrimPrefix(stage, "."), stagedVolumeSuffix)

		body = body[:strings.LastIndexByte(body, '.')]
		if after, ok := strings.CutPrefix(body, "binding-"); ok {
			if _, err := BindingPath(after); err == nil {
				c.activeBindings[after] = struct{}{}
			}
		} else {
			c.protect(body)
		}
	}
}

func (c *copyCensus) readRecords(root *os.Root) {
	for _, entry := range c.entries {
		path := filepath.Join(".copy-on-use", entry.Name())
		if after, ok := strings.CutPrefix(entry.Name(), "binding-"); ok {
			key := after
			if _, e := BindingPath(key); e != nil {
				continue
			}

			name, info, e := readBindingRecord(root, path)
			if e != nil || name == "" {
				continue
			}

			c.bindings = append(c.bindings, cleanupBinding{key: key, name: name, path: path, info: info})
			c.completed[key] = name
		}
	}

	for _, entry := range c.entries {
		if !strings.HasPrefix(entry.Name(), "owner-") {
			continue
		}

		path := filepath.Join(".copy-on-use", entry.Name())

		key, name, info, valid, e := readCopyOwner(root, path)
		if e == nil && valid {
			c.owners = append(c.owners, cleanupOwner{key: key, name: name, path: path, info: info})
		}
	}
}

func (c *copyCensus) protectRecords(root *os.Root, keep Retention) {
	for _, b := range c.bindings {
		if _, active := c.activeBindings[b.key]; active {
			c.protected[b.name] = struct{}{}
		}

		if _, wanted := keep.Bindings[b.key]; wanted {
			if witness, err := root.Lstat(Witness(b.name, false)); err == nil && witness.Mode().IsRegular() {
				c.protected[b.name] = struct{}{}
			}
		}
	}

	for _, o := range c.owners {
		if _, wanted := keep.Bindings[o.key]; wanted && (c.completed[o.key] == "" || c.completed[o.key] == o.name) {
			c.protected[o.name] = struct{}{}
		}
	}
}

// The owner record (or a completed binding) is required to prove a
// witness's name. Never reverse a hash to guess ownership.
func (c *copyCensus) removableNames(root *os.Root, keep Retention) map[string]struct{} {
	names := make(map[string]struct{})

	for _, o := range c.owners {
		if c.ownerRemovable(o, keep) {
			names[o.name] = struct{}{}
		}
	}

	for _, b := range c.bindings {
		if c.bindingRemovable(root, b, keep) {
			names[b.name] = struct{}{}
		}
	}

	return names
}

func (c *copyCensus) ownerRemovable(o cleanupOwner, keep Retention) bool {
	if _, busy := c.protected[o.name]; busy {
		return false
	}

	if _, busy := c.activeBindings[o.key]; busy {
		return false
	}

	_, wanted := keep.Bindings[o.key]

	return !wanted || (c.completed[o.key] != "" && c.completed[o.key] != o.name)
}

func (c *copyCensus) bindingRemovable(root *os.Root, b cleanupBinding, keep Retention) bool {
	if _, busy := c.protected[b.name]; busy {
		return false
	}

	if _, busy := c.activeBindings[b.key]; busy {
		return false
	}

	if _, wanted := keep.Bindings[b.key]; wanted {
		_, _, valid, err := ReadImageMarker(root, Witness(b.name, true))

		return err == nil && valid
	}

	return true
}

func (c *copyCensus) collect(ctx context.Context, root *os.Root, keep Retention, syncDirectory func(*os.Root, string) error) error {
	var errs error

	names := c.removableNames(root, keep)

	for name := range names {
		if err := ctx.Err(); err != nil {
			return errors.Join(errs, err)
		}

		errs = errors.Join(errs, c.retireName(root, name, keep, syncDirectory))
	}

	if errs != nil {
		return errs
	}

	errs = c.retireStandaloneMarkers(root, syncDirectory)
	if errs != nil {
		return errs
	}

	// Standalone markers sync themselves; an idle pool needs no directory flush.
	if len(names) == 0 {
		return nil
	}

	return syncDirectory(root, ".copy-on-use")
}

func (c *copyCensus) retireName(root *os.Root, name string, keep Retention, syncDirectory func(*os.Root, string) error) error {
	retired, err := c.retireWitness(root, name, syncDirectory)
	if err != nil || !retired {
		return err
	}

	_, _, writable, err := ReadImageMarker(root, Witness(name, true))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err = c.retireRecords(root, name, keep, writable); err != nil {
		return err
	}

	// The binding unlink must be durable before marker evidence can disappear.
	if err = syncDirectory(root, ".copy-on-use"); err != nil {
		return err
	}

	if writable {
		_, err = retireWritableMarker(root, name, syncDirectory)
	}

	return err
}

func (c *copyCensus) retireWitness(root *os.Root, name string, syncDirectory func(*os.Root, string) error) (bool, error) {
	// Keep a durable owner/binding record until after the witness unlink barrier.
	witness := Witness(name, false)

	info, err := root.Lstat(witness)

	switch {
	case err == nil && info.Mode().IsRegular():
		if !c.provesWitness(root, name, info) {
			return false, nil // Legacy/changed witness: no inode proof to unlink it.
		}

		// A public hard link is never detached from its witness.
		if err = removeSamePrivate(root, witness, info); err != nil {
			return false, err
		}

		if err = syncDirectory(root, ".copy-on-use"); err != nil {
			return false, err
		}
	case err == nil: // Foreign/nonregular witness is not ours.
		return false, nil
	case !os.IsNotExist(err):
		return false, err
	}

	// This barrier is also required when the witness was already absent.
	return true, syncDirectory(root, ".copy-on-use")
}

func (c *copyCensus) provesWitness(root *os.Root, name string, info os.FileInfo) bool {
	for _, o := range c.owners {
		if o.name == name && ownerWitnessMatches(root, o, info) {
			return true
		}
	}

	return false
}

func retireWritableMarker(root *os.Root, name string, syncDirectory func(*os.Root, string) error) (bool, error) {
	// A writable marker holds no payload. Its content must parse as genuine
	// metadata; a missing public name alone is insufficient evidence.
	marker := Witness(name, true)

	_, _, valid, err := ReadImageMarker(root, marker)
	if err != nil || !valid {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	info, err := root.Lstat(marker)
	if err == nil {
		err = removeSamePrivate(root, marker, info)
	}

	if err == nil {
		err = syncDirectory(root, ".copy-on-use")
	}

	if os.IsNotExist(err) {
		return true, nil
	}

	return true, err
}

func (c *copyCensus) retireRecords(root *os.Root, name string, keep Retention, writable bool) error {
	var errs error

	for _, o := range c.owners {
		if o.name == name {
			errs = errors.Join(errs, removeOwnerRecord(root, o))
		}
	}

	for _, b := range c.bindings {
		if b.name == name {
			if _, wanted := keep.Bindings[b.key]; !wanted || writable {
				errs = errors.Join(errs, removeBindingRecord(root, b))
			}
		}
	}

	return errs
}

// standaloneMarkers lists writable markers which no public/protected destination
// or still-present binding hash matches.
func (c *copyCensus) standaloneMarkers() []string {
	hashes := make(map[string]struct{}, len(c.protected)+len(c.bindings))
	for name := range c.protected {
		hashes[filepath.Base(Witness(name, true))] = struct{}{}
	}

	for _, b := range c.bindings {
		hashes[filepath.Base(Witness(b.name, true))] = struct{}{}
	}

	var paths []string

	for _, entry := range c.entries {
		if !strings.HasPrefix(entry.Name(), "image-") {
			continue
		}

		if _, err := BindingPath(strings.TrimPrefix(entry.Name(), "image-")); err != nil {
			continue
		}

		if _, ok := hashes[entry.Name()]; ok {
			continue
		}

		paths = append(paths, filepath.Join(".copy-on-use", entry.Name()))
	}

	return paths
}

func (c *copyCensus) retireStandaloneMarkers(root *os.Root, syncDirectory func(*os.Root, string) error) error {
	// Only valid standalone markers are retired. Unknown and malformed entries remain untouched.
	var errs error

	for _, path := range c.standaloneMarkers() {
		errs = errors.Join(errs, retireStandaloneMarker(root, path, syncDirectory))
	}

	return errs
}

func retireStandaloneMarker(root *os.Root, path string, syncDirectory func(*os.Root, string) error) error {
	_, _, valid, err := ReadImageMarker(root, path)
	if err != nil {
		return err
	}

	if !valid {
		return nil
	}

	// An interrupted binding unlink may be absent in this namespace but
	// not durable yet. Commit that absence before discarding the marker.
	if err = syncDirectory(root, ".copy-on-use"); err != nil {
		return err
	}

	info, err := root.Lstat(path)
	if err != nil {
		return err
	}

	if err = removeSamePrivate(root, path, info); err != nil {
		return err
	}

	return syncDirectory(root, ".copy-on-use")
}

func removeBindingRecord(root *os.Root, b cleanupBinding) error {
	name, info, err := readBindingRecord(root, b.path)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil || name != b.name || !os.SameFile(info, b.info) {
		return err
	}

	return removeSamePrivate(root, b.path, b.info)
}

func removeOwnerRecord(root *os.Root, o cleanupOwner) error {
	key, name, info, valid, err := readCopyOwner(root, o.path)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil || !valid || key != o.key || name != o.name || !os.SameFile(info, o.info) {
		return err
	}

	return removeSamePrivate(root, o.path, o.info)
}

func removeSamePrivate(root *os.Root, path string, info os.FileInfo) error {
	current, err := root.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if !current.Mode().IsRegular() || !os.SameFile(current, info) {
		return fmt.Errorf("private copy artifact %q changed during cleanup", path)
	}

	return root.Remove(path)
}

func validateSelectedCopyBinding(root *os.Root, key, name string) error {
	if err := ownedCopy(root, name); err != nil {
		recoverable, recoveryErr := recoverableImmutableCopy(root, key, name)
		if recoveryErr != nil || !recoverable {
			return fmt.Errorf("invalid copy binding: %w", errors.Join(err, recoveryErr))
		}
	}

	return nil
}

// Binding lookup only selects. Adoption establishes durability before relinking.
func recoverableImmutableCopy(root *os.Root, key, name string) (bool, error) {
	if _, err := root.Lstat(name); !os.IsNotExist(err) {
		return false, err
	}

	return immutableOwnerWitness(root, key, name)
}

func immutableOwnerWitness(root *os.Root, key, name string) (bool, error) {
	// Writable metadata and unknown legacy witnesses cannot recover guest bytes.
	if _, err := root.Lstat(Witness(name, true)); !os.IsNotExist(err) {
		return false, err
	}

	path := OwnerPath(name)

	ownerKey, ownerName, _, valid, err := readCopyOwner(root, path)
	if err != nil || !valid || ownerKey != key || ownerName != name {
		return false, err
	}

	witness, err := root.Lstat(Witness(name, false))
	if err != nil {
		return false, err
	}

	return witness.Mode().IsRegular() && ownerWitnessMatches(root, cleanupOwner{key: key, name: name, path: path}, witness), nil
}

// An owner manifest precedes publication and the completed binding. A crash in
// that window can still leave one explicitly owned immutable recovery asset.
func unboundImmutableCopy(root *os.Root, key string) (string, error) {
	entries, err := copyOwnerEntries(root)
	if err != nil {
		return "", err
	}

	name, err := uniqueCopyOwner(root, entries, key)
	if err != nil || name == "" {
		return "", err
	}

	valid, err := immutableOwnerWitness(root, key, name)
	if os.IsNotExist(err) {
		return "", nil // No recoverable payload: normal source provisioning may retry.
	}

	if err != nil || !valid {
		return "", err
	}

	if err := validateSelectedCopyBinding(root, key, name); err != nil {
		return "", err
	}

	return name, nil
}

func copyOwnerEntries(root *os.Root) ([]os.DirEntry, error) {
	info, err := root.Lstat(".copy-on-use")
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("private copy ownership path is not a directory")
	}

	return readCopyEntries(root, ".copy-on-use")
}

func uniqueCopyOwner(root *os.Root, entries []os.DirEntry, key string) (string, error) {
	var selected string

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "owner-") {
			continue
		}

		ownerKey, name, _, valid, err := readCopyOwner(root, filepath.Join(".copy-on-use", entry.Name()))
		if err != nil {
			return "", err
		}

		if !valid || ownerKey != key {
			continue
		}

		if selected != "" && selected != name {
			return "", nil // Multiple generations require the source, never a guess.
		}

		selected = name
	}

	return selected, nil
}
