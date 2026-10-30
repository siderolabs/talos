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
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"strings"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/opencontainers/go-digest"
	"libvirt.org/go/libvirtxml"
)

// volumeMode is the permission a created volume is given.
//
// Set explicitly rather than inherited: libvirt's own default for a directory pool depends on the
// daemon's configuration, which lives in the extension rather than here, and a volume the guest
// cannot open is indistinguishable from one that was never created.
const volumeMode = 0o600

// stagedVolumePrefix and stagedVolumeSuffix name a volume which is still being created.
//
// A volume is built under a staged name and linked into place only if the requested name is
// still free. A crash mid-create cannot leave a half-written file under the name a virtual
// machine would adopt. The prefix is a dot, which keeps a leftover out of the way; the suffix is
// what lets one be swept.
const (
	stagedVolumePrefix = "."
	stagedVolumeSuffix = ".staged"
)

// errVolumeExists reports a name already taken in the pool. CreateVolume never overwrites.
var errVolumeExists = errors.New("volume already exists")

// ErrVolumeObservationUnavailable identifies a read RPC interrupted by transport loss.
// A received rejection or a malformed completed reply is not an outage.
var ErrVolumeObservationUnavailable = errors.New("volume observation unavailable")

func volumeReadRPCError(err error) error {
	if err == nil {
		return nil
	}

	// The SDK returns XDR decode failures from completed replies as opaque errors,
	// including an EOF inside its decoder. Only direct transport errors are evidence
	// of lost observation; unknown errors and unsupported procedures stay permanent.
	if rpcErr, ok := errors.AsType[libvirt.Error](err); ok {
		if rpcErr.Code != uint32(libvirt.ErrNoConnect) {
			return err
		}

		return fmt.Errorf("%w: %w", ErrVolumeObservationUnavailable, err)
	}

	for _, interrupted := range []error{
		libvirt.ErrInterrupted,
		context.Canceled,
		context.DeadlineExceeded,
		io.EOF,
		io.ErrUnexpectedEOF,
		io.ErrClosedPipe,
		io.ErrShortWrite,
		net.ErrClosed,
	} {
		if errors.Is(err, interrupted) {
			return fmt.Errorf("%w: %w", ErrVolumeObservationUnavailable, err)
		}
	}

	if netErr, ok := errors.AsType[*net.OpError](err); ok && netErr != nil {
		return fmt.Errorf("%w: %w", ErrVolumeObservationUnavailable, err)
	}

	return err
}

// Volume is an observed volume within a pool.
type Volume struct {
	// Name is the volume's name within its pool, which is its file name in the pool directory.
	Name string
	// Path is the absolute host path of the volume's file.
	Path string
	// Format is the volume's on-disk format, as libvirt reports it: "raw", "qcow2", and so on.
	Format string
	// Capacity is the volume's logical size in bytes -- what a guest sees, not what the file
	// occupies.
	Capacity uint64
}

// stagedVolumeName returns the name a volume of name is built under.
//
// Unique per attempt, so a staged file left behind by an interrupted create can never block a
// later one of the same name.
func stagedVolumeName(name string) string {
	return fmt.Sprintf("%s%s.%016x%s", stagedVolumePrefix, name, rand.Uint64(), stagedVolumeSuffix)
}

// isStagedVolumeName reports whether name is a volume staged by stagedVolumeName.
func isStagedVolumeName(name string) bool {
	if !strings.HasPrefix(name, stagedVolumePrefix) || !strings.HasSuffix(name, stagedVolumeSuffix) {
		return false
	}

	body := strings.TrimSuffix(strings.TrimPrefix(name, stagedVolumePrefix), stagedVolumeSuffix)

	separator := strings.LastIndexByte(body, '.')
	if separator <= 0 || body[:separator] == "." || body[:separator] == ".." {
		return false
	}

	return isStagedVolumeNonce(body[separator+1:])
}

// isStagedVolumeNonce matches the fixed-width lowercase hexadecimal staging nonce.
func isStagedVolumeNonce(nonce string) bool {
	if len(nonce) != 16 {
		return false
	}

	for _, character := range nonce {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}

	return true
}

// Volume looks a volume up by name within a pool.
//
// A miss refreshes the pool once before being reported: libvirt rereads a directory pool when it
// activates it and at no other time, so a volume which appeared since -- including one this host
// created before its last restart -- is otherwise invisible.
func (c *Client) Volume(pool Pool, name string) (Volume, bool, error) {
	p, exists, err := c.lookup(pool)
	if err != nil {
		return Volume{}, false, err
	}

	if !exists {
		return Volume{}, false, fmt.Errorf("storage pool %q is not defined", pool.Name)
	}

	return c.volume(p, name, true)
}

// volume looks a volume up, optionally refreshing the pool once on a miss.
func (c *Client) volume(p libvirt.StoragePool, name string, refresh bool) (Volume, bool, error) {
	vol, err := c.rpc.StorageVolLookupByName(p, name)
	if err != nil {
		if !isNoStorageVol(err) {
			return Volume{}, false, volumeReadRPCError(err)
		}

		if !refresh {
			return Volume{}, false, nil
		}

		if err = c.rpc.StoragePoolRefresh(p, 0); err != nil {
			return Volume{}, false, fmt.Errorf("failed to refresh storage pool %q: %w", p.Name, volumeReadRPCError(err))
		}

		return c.volume(p, name, false)
	}

	return c.describeVolume(vol, name)
}

// describeVolume reads a volume's path, format and logical capacity.
func (c *Client) describeVolume(vol libvirt.StorageVol, name string) (Volume, bool, error) {
	path, err := c.rpc.StorageVolGetPath(vol)
	if err != nil {
		return Volume{}, false, volumeReadRPCError(err)
	}

	text, err := c.rpc.StorageVolGetXMLDesc(vol, 0)
	if err != nil {
		return Volume{}, false, volumeReadRPCError(err)
	}

	var desc libvirtxml.StorageVolume
	if err = desc.Unmarshal(text); err != nil {
		return Volume{}, false, err
	}

	var format string

	if desc.Target != nil && desc.Target.Format != nil {
		format = desc.Target.Format.Type
	}

	// Capacity, not allocation: allocation is what the file occupies on the host, which for a
	// sparse volume says nothing about the size the guest is offered.
	_, capacity, _, err := c.rpc.StorageVolGetInfo(vol)
	if err != nil {
		return Volume{}, false, volumeReadRPCError(err)
	}

	return Volume{Name: name, Path: path, Format: format, Capacity: capacity}, true, nil
}

// CreateVolume creates a sparse volume of capacity bytes in the given format.
//
// It never overwrites: a name already taken in the pool is refused rather than reused, so no call
// here can destroy a volume's contents.
//
// The volume is built under a staged name and atomically linked into place without replacing an
// existing name, so the name asked for either does not exist or names a volume created in full.
// That matters most for qcow2, where a truncated header is a file QEMU refuses to open at domain
// start rather than one this can grade.
func (c *Client) CreateVolume(pool Pool, name, format string, capacity uint64, backingFile, backingFormat string) (Volume, error) {
	p, target, err := c.poolDirectory(pool)
	if err != nil {
		return Volume{}, err
	}

	switch _, found, volErr := c.volume(p, name, true); {
	case volErr != nil:
		return Volume{}, volErr
	case found:
		return Volume{}, fmt.Errorf("%w: %q in storage pool %q", errVolumeExists, name, pool.Name)
	}

	if err = c.stageVolumeIntoPlace(p, target, name, format, capacity, backingFile, backingFormat); err != nil {
		return Volume{}, err
	}

	volume, found, err := c.volume(p, name, false)

	switch {
	case err != nil:
		return Volume{}, err
	case !found:
		return Volume{}, fmt.Errorf("volume %q is missing from storage pool %q after creation", name, pool.Name)
	}

	return volume, nil
}

// ErrCopySourceRequired identifies an owned copy which has not yet been provisioned.
var ErrCopySourceRequired = errors.New("copy source is required for initial provisioning")

// CopyVolume publishes an independent raw copy in the destination pool. A same-inode witness
// distinguishes completed copies from preexisting guest or foreign volumes on later passes.
func (c *Client) CopyVolume(pool Pool, name, source, expected string) (Volume, error) {
	if err := validateCopyName(name); err != nil {
		return Volume{}, err
	}

	p, root, err := c.openCopyPool(pool)
	if err != nil {
		return Volume{}, err
	}

	defer root.Close() //nolint:errcheck

	witness := fmt.Sprintf(".copy-on-use/%x", sha256.Sum256([]byte(name)))

	volume, adopted, err := c.adoptCopy(p, root, name, witness)
	if adopted || err != nil {
		return volume, err
	}

	if source == "" {
		return Volume{}, ErrCopySourceRequired
	}

	staged := stagedVolumeName(name)
	defer root.Remove(staged) //nolint:errcheck

	if err = c.stageCopy(p, root, staged, source, expected); err != nil {
		return Volume{}, err
	}

	if err = publishCopy(root, staged, name, witness); err != nil {
		return Volume{}, err
	}

	return c.copiedVolume(p, name)
}

func validateCopyName(name string) error {
	if name == "" || filepath.Base(name) != name || name == "." || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid copy volume name %q", name)
	}

	return nil
}

func (c *Client) openCopyPool(pool Pool) (libvirt.StoragePool, *os.Root, error) {
	p, target, err := c.poolDirectory(pool)
	if err != nil {
		return p, nil, err
	}

	if err = sweepStagedVolumes(target); err != nil {
		return p, nil, err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return p, nil, err
	}

	if err = root.Mkdir(".copy-on-use", 0o700); err != nil && !os.IsExist(err) {
		return p, nil, errors.Join(err, root.Close())
	}

	return p, root, nil
}

func (c *Client) adoptCopy(p libvirt.StoragePool, root *os.Root, name, witness string) (Volume, bool, error) {
	exists, err := checkCopyOwnership(root, name, witness)
	if err != nil {
		return Volume{}, false, err
	}

	if !exists {
		exists, err = recoverCopy(root, name, witness)
		if !exists || err != nil {
			return Volume{}, false, err
		}
	} else {
		// Ownership links may survive an attempt whose directory sync failed.
		// Restore both ordered barriers before admitting the existing copy.
		if err := syncCopyDirectory(root, ".copy-on-use"); err != nil {
			return Volume{}, false, err
		}

		if err := syncCopyDirectory(root, "."); err != nil {
			return Volume{}, false, err
		}
	}

	volume, err := c.copiedVolume(p, name)

	return volume, true, err
}

func checkCopyOwnership(root *os.Root, name, witness string) (bool, error) {
	file, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	owned, err := root.Lstat(witness)
	if err != nil {
		return false, fmt.Errorf("copy volume %q has no ownership witness: %w", name, err)
	}

	if !file.Mode().IsRegular() || !owned.Mode().IsRegular() || !os.SameFile(file, owned) {
		return false, fmt.Errorf("copy volume %q exists without a matching ownership witness", name)
	}

	return true, nil
}

func recoverCopy(root *os.Root, name, witness string) (bool, error) {
	owned, err := root.Lstat(witness)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !owned.Mode().IsRegular() {
		return false, fmt.Errorf("copy volume %q has a nonregular ownership witness", name)
	}

	// A witness from an interrupted publication is not proven durable yet.
	if err := syncCopyDirectory(root, ".copy-on-use"); err != nil {
		return false, err
	}

	if err = root.Link(witness, name); err != nil {
		return false, fmt.Errorf("recover copy volume %q: %w", name, err)
	}

	return true, syncCopyDirectory(root, ".")
}

func (c *Client) stageCopy(p libvirt.StoragePool, root *os.Root, staged, source, expected string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("copy source %q is not a regular file", source)
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}

	defer input.Close() //nolint:errcheck

	opened, err := input.Stat()
	if err != nil {
		return err
	}

	if !os.SameFile(info, opened) {
		return fmt.Errorf("copy source %q changed while opening", source)
	}

	if err = writeCopy(root, staged, input, expected); err != nil {
		return err
	}

	if _, err = c.copiedVolume(p, staged); err != nil {
		return fmt.Errorf("copy source must be a self-contained raw image or ISO: %w", err)
	}

	return nil
}

func writeCopy(root *os.Root, staged string, input io.Reader, expected string) error {
	var verifier digest.Verifier

	if expected != "" {
		parsed, err := digest.Parse(expected)
		if err != nil {
			return err
		}

		verifier = parsed.Verifier()
	}

	output, err := root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, volumeMode)
	if err != nil {
		return err
	}

	var writer io.Writer = output

	if verifier != nil {
		writer = io.MultiWriter(output, verifier)
	}

	_, copyErr := io.Copy(writer, input)
	if copyErr == nil && verifier != nil && !verifier.Verified() {
		copyErr = fmt.Errorf("copy source digest mismatch: expected %s", expected)
	}

	if copyErr == nil {
		copyErr = output.Sync()
	}

	return errors.Join(copyErr, output.Close())
}

func publishCopy(root *os.Root, staged, name, witness string) error {
	if err := root.Link(staged, witness); err != nil {
		return fmt.Errorf("claim copy volume %q: %w", name, err)
	}

	if err := syncCopyDirectory(root, ".copy-on-use"); err != nil {
		return err
	}

	if err := root.Link(staged, name); err != nil {
		return fmt.Errorf("publish copy volume %q: %w", name, err)
	}

	return syncCopyDirectory(root, ".")
}

// syncCopyDirectory makes ownership and publication durable before reporting a usable copy.
func syncCopyDirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return err
	}

	return errors.Join(directory.Sync(), directory.Close())
}

func (c *Client) copiedVolume(p libvirt.StoragePool, name string) (Volume, error) {
	volume, found, err := c.volume(p, name, true)
	if err != nil {
		return Volume{}, err
	}

	if !found || (volume.Format != "raw" && volume.Format != "iso") {
		return Volume{}, fmt.Errorf("copy volume %q has no raw or ISO libvirt observation", name)
	}

	// Libvirt probes ISO9660 media as "iso"; QEMU attaches its bytes with the raw driver.
	volume.Format = "raw"

	return volume, nil
}

// poolDirectory resolves a pool to its libvirt handle and the directory its volumes are files in.
func (c *Client) poolDirectory(pool Pool) (libvirt.StoragePool, string, error) {
	p, exists, err := c.lookup(pool)
	if err != nil {
		return p, "", err
	}

	if !exists {
		return p, "", fmt.Errorf("storage pool %q is not defined", pool.Name)
	}

	target, err := c.target(p)
	if err != nil {
		return p, "", err
	}

	if target == "" {
		return p, "", fmt.Errorf("storage pool %q has no target directory", pool.Name)
	}

	return p, target, nil
}

// stageVolumeIntoPlace builds the volume under a staged name and publishes it without replacement.
func (c *Client) stageVolumeIntoPlace(p libvirt.StoragePool, target, name, format string, capacity uint64, backingFile, backingFormat string) (result error) {
	// Leftovers from interrupted or failed creates are cleared first, so a pool directory does not
	// collect one staged file per attempt. Safe here: StoragePoolVolumeController is the only writer
	// of a pool's volumes and reconciles one at a time, so no staged name in flight is another
	// caller's. The error is returned rather than ignored -- publication below needs write access
	// to this same directory, so a sweep which cannot remove a file predicts a create which cannot
	// finish.
	if err := sweepStagedVolumes(target); err != nil {
		return err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return err
	}

	defer root.Close() //nolint:errcheck

	staged := stagedVolumeName(name)

	// Also remove a partial file when libvirt fails during creation. A successful publication
	// removes the staged link before refreshing libvirt's cached view of the directory.
	defer func() {
		if err := root.Remove(staged); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, fmt.Errorf("failed to remove staged volume %q: %w", staged, err))
		}
	}()

	if err = c.createStagedVolume(p, staged, format, capacity, backingFile, backingFormat); err != nil {
		return err
	}

	// A hard link publishes the completed inode only if the final name is still free. Unlike
	// rename, link cannot replace a file (or symlink) which appeared since the libvirt lookup.
	// Both names are resolved under the same rooted pool directory.
	if err = root.Link(staged, name); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %q in storage pool %q: %w", errVolumeExists, name, p.Name, err)
		}

		return fmt.Errorf("failed to publish staged volume %q: %w", staged, err)
	}

	if err = root.Remove(staged); err != nil {
		return fmt.Errorf("failed to remove published staged volume %q: %w", staged, err)
	}

	// Publication happened underneath libvirt, which still has the staged name cached and not
	// the final one.
	if err = c.rpc.StoragePoolRefresh(p, 0); err != nil {
		return fmt.Errorf("failed to refresh storage pool %q: %w", p.Name, err)
	}

	return nil
}

// createStagedVolume builds the volume libvirt will later be told to reread under its real name.
func (c *Client) createStagedVolume(p libvirt.StoragePool, name, format string, capacity uint64, backingFile, backingFormat string) error {
	desc := libvirtxml.StorageVolume{
		Name: name,
		// Allocation is pinned at zero so the volume is sparse. Left unset, libvirt allocates the
		// whole capacity up front, which for a raw volume on a filesystem without working fallocate
		// means writing that many zeros -- far longer than any session here is given.
		Allocation: &libvirtxml.StorageVolumeSize{Unit: "bytes", Value: 0},
		Capacity:   &libvirtxml.StorageVolumeSize{Unit: "bytes", Value: capacity},
		Target: &libvirtxml.StorageVolumeTarget{
			Format:      &libvirtxml.StorageVolumeTargetFormat{Type: format},
			Permissions: &libvirtxml.StorageVolumeTargetPermissions{Mode: fmt.Sprintf("%04o", volumeMode)},
		},
	}

	if backingFile != "" {
		desc.BackingStore = &libvirtxml.StorageVolumeBackingStore{Path: backingFile}
		if backingFormat != "" {
			desc.BackingStore.Format = &libvirtxml.StorageVolumeTargetFormat{Type: backingFormat}
		}
	}

	text, err := desc.Marshal()
	if err != nil {
		return err
	}

	if _, err = c.rpc.StorageVolCreateXML(p, text, 0); err != nil {
		return fmt.Errorf("failed to create volume %q in storage pool %q: %w", name, p.Name, err)
	}

	return nil
}

// ResizeVolumeOperation grows a volume to capacity bytes and reports whether the
// resize RPC finished, was not submitted, or lost its reply.
//
// It only ever grows: the shrink flag is never passed, so libvirt refuses a capacity below the
// volume's own even if a caller asks for one. Resizing a volume a guest has open is the caller's to
// avoid -- for qcow2 QEMU holds a write lock and this fails, and for raw it succeeds while the
// guest goes on seeing the old size.
func (c *Client) ResizeVolumeOperation(pool Pool, name string, capacity uint64) (OperationOutcome, error) {
	var evidence mutationEvidence

	p, exists, err := c.lookup(pool)
	if err != nil {
		return evidence.outcome, err
	}

	if !exists {
		return evidence.outcome, fmt.Errorf("storage pool %q is not defined", pool.Name)
	}

	vol, err := c.rpc.StorageVolLookupByName(p, name)
	if err != nil {
		return evidence.outcome, fmt.Errorf("failed to look up volume %q in storage pool %q: %w", name, pool.Name, err)
	}

	err = c.rpc.StorageVolResize(vol, capacity, 0)
	evidence.record(err)

	return evidence.outcome, err
}

// sweepStagedVolumes clears what interrupted creates left behind.
//
// Only names this package stages are removed. A volume is never deleted here: a staged file is one
// no virtual machine has ever been told about, which is what makes clearing it safe.
func sweepStagedVolumes(poolDir string) error {
	root, err := os.OpenRoot(poolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	defer root.Close() //nolint:errcheck

	dir, err := root.Open(".")
	if err != nil {
		return err
	}

	defer dir.Close() //nolint:errcheck

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}

	var errs error

	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isStagedVolumeName(entry.Name()) {
			continue
		}

		if err = root.Remove(entry.Name()); err != nil && !os.IsNotExist(err) {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

// isNoStorageVol reports whether err is libvirt saying the volume does not exist.
func isNoStorageVol(err error) bool {
	rpcErr, ok := errors.AsType[libvirt.Error](err)

	return ok && rpcErr.Code == uint32(libvirt.ErrNoStorageVol)
}
