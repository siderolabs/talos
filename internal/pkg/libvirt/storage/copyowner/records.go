// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copyowner

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Bindings remembers durability acknowledgments only for this controller run.
// Its zero value is ready for use by a single reconciliation owner.
type Bindings struct {
	proven map[string]copyBindingProof
}

type copyBindingProof struct {
	root    os.FileInfo
	private os.FileInfo
	record  os.FileInfo
}

// BindingPath returns the private record for binding key, rejecting non-SHA-256 keys.
func BindingPath(key string) (string, error) {
	if len(key) != 64 || strings.Trim(key, "0123456789abcdef") != "" {
		return "", fmt.Errorf("invalid copy binding key %q", key)
	}

	return filepath.Join(".copy-on-use", "binding-"+key), nil
}

func ownedCopy(root *os.Root, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// Bindings are shared by immutable media and writable imports and have no
	// requested mode. Adoption/mutation validate the specific witness class.
	// Here either class is allowed, but an invalid present witness fails closed.
	for _, image := range []bool{false, true} {
		witness := Witness(name, image)
		if _, err := root.Lstat(witness); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}

		exists, err := Check(root, name, witness)
		if err != nil {
			return err
		}

		if exists {
			return nil
		}
	}

	return fmt.Errorf("copy %q is not published", name)
}

// PublicationAbsent proves both publication paths are missing, not merely unobservable.
func PublicationAbsent(target, name string) (bool, error) {
	if err := ValidateName(name); err != nil {
		return false, err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return false, err
	}
	defer root.Close() //nolint:errcheck

	// A public payload or an immutable hard-link witness may still be
	// recoverable. A writable marker contains only inode metadata and cannot
	// recover guest bytes after an interrupted publication removes the stage.
	for _, path := range []string{name, Witness(name, false)} {
		_, err := root.Lstat(path)
		if err == nil {
			return false, nil
		}

		if !os.IsNotExist(err) {
			return false, err
		}
	}

	_, _, valid, err := ReadImageMarker(root, Witness(name, true))
	if err != nil {
		return false, err
	}

	if valid {
		return true, nil
	}

	if _, err := root.Lstat(Witness(name, true)); err == nil {
		return false, errors.New("invalid writable copy ownership marker")
	} else if !os.IsNotExist(err) {
		return false, err
	}

	return true, nil
}

// ReadBinding selects an explicitly completed destination, or one uniquely
// owned immutable recovery asset. Missing or ambiguous selections return an empty
// name; malformed, stale and foreign records fail closed. Lookup never publishes.
func ReadBinding(target, key string) (string, error) {
	path, err := BindingPath(key)
	if err != nil {
		return "", err
	}

	root, err := os.OpenRoot(target)
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck

	name, _, err := readBindingRecord(root, path)
	if err != nil {
		return "", err
	}

	if name == "" {
		return unboundImmutableCopy(root, key)
	}

	if err := validateSelectedCopyBinding(root, key, name); err != nil {
		return "", err
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

	if err := ValidateName(string(data)); err != nil {
		return "", nil, err
	}

	return string(data), info, nil
}

// Record durably selects a completed owned copy. Cold observations reestablish directory
// barriers; unchanged, acknowledged root/private-directory/record identities need no writes.
func (bindings *Bindings) Record(target, key, name string) error {
	path, err := BindingPath(key)
	if err != nil {
		return err
	}

	root, proof, err := openBindingRoot(target, name)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	return bindings.record(root, target, target+"\x00"+key, path, key, name, proof)
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

func (bindings *Bindings) acknowledged(key string, proof copyBindingProof) bool {
	previous, ok := bindings.proven[key]

	return ok && os.SameFile(proof.root, previous.root) && os.SameFile(proof.private, previous.private) &&
		os.SameFile(proof.record, previous.record)
}

func (bindings *Bindings) record(root *os.Root, target, cacheKey, path, key, name string, proof copyBindingProof) error {
	previous, recordInfo, err := readBindingRecord(root, path)
	if err != nil {
		return err
	}

	proof.record = recordInfo
	if previous == name && bindings.acknowledged(cacheKey, proof) {
		return nil
	}

	if previous != name {
		if err := writeBindingRecord(root, target, path, key, name); err != nil {
			return err
		}

		proof.record, err = root.Lstat(path)
		if err != nil {
			return err
		}
	}

	if err := Sync(root, ".copy-on-use"); err != nil {
		return err
	}

	if err := Sync(root, "."); err != nil {
		return err
	}

	if bindings.proven == nil {
		bindings.proven = make(map[string]copyBindingProof)
	}

	bindings.proven[cacheKey] = proof

	return nil
}

func writeBindingRecord(root *os.Root, target, path, key, name string) error {
	// StageName includes a fresh random nonce on every attempt, including retries.
	stageName := StageName("binding-" + key)
	staged := ".copy-on-use/" + stageName

	release := ProtectStage(target, stageName)
	defer release()

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

// Retention contains authoritative desired binding keys and currently held,
// requested or in-flight destination names. Call only with a canonical config.
type Retention struct {
	Bindings map[string]struct{}
	Volumes  map[string]struct{}
}

// copyInode identifies the witness inode in the durable owner record. st_dev is
// omitted: device numbers change across reboots, inode numbers on one filesystem do not.
func copyInode(info os.FileInfo) string {
	return strconv.FormatUint(info.Sys().(*syscall.Stat_t).Ino, 10)
}

// OwnerPath returns the private owner record of an immutable copy named name.
func OwnerPath(name string) string {
	return fmt.Sprintf(".copy-on-use/owner-%x", sha256.Sum256([]byte(name)))
}

// OwnerStage is an immutable copy's private payload stage awaiting publication
// under Name, recorded for binding Key. Witness is the hard link it will gain.
type OwnerStage struct {
	Root    *os.Root
	Staged  string
	Name    string
	Witness string
	Key     string
}

// PublishOwner persists the immutable destination's intent before the
// witness can become visible. The live payload stage protects its metadata stage.
func (op OwnerStage) PublishOwner() error {
	path := OwnerPath(op.Name)

	stagedInfo, err := op.Root.Lstat(op.Staged)
	if err != nil || !stagedInfo.Mode().IsRegular() {
		return errors.Join(err, errors.New("immutable copy stage is not regular"))
	}

	if existing, err := op.checkPublishedOwner(path, stagedInfo); err != nil || existing {
		return err
	}

	content := op.Key + "\n" + op.Name + "\n" + copyInode(stagedInfo) + "\n"

	return op.writeCopyOwner(path, content)
}

func (op OwnerStage) checkPublishedOwner(path string, stagedInfo os.FileInfo) (bool, error) {
	if _, err := op.Root.Lstat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	key, name, _, valid, err := readCopyOwner(op.Root, path)
	if err != nil {
		return false, err
	}

	if !valid || key != op.Key || name != op.Name {
		return false, fmt.Errorf("copy owner %q already exists with different identity", path)
	}

	if ownerWitnessMatches(op.Root, cleanupOwner{key: key, name: name, path: path}, stagedInfo) {
		return true, Sync(op.Root, ".copy-on-use")
	}

	if _, witnessErr := op.Root.Lstat(op.Witness); !os.IsNotExist(witnessErr) {
		return false, fmt.Errorf("copy owner %q has an existing witness or cannot be checked: %v", path, witnessErr)
	}

	if err := op.Root.Remove(path); err != nil {
		return false, err
	}

	return false, Sync(op.Root, ".copy-on-use")
}

func (op OwnerStage) writeCopyOwner(path, content string) error {
	stage := filepath.Join(".copy-on-use", filepath.Base(op.Staged)+".owner")

	f, err := op.Root.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	defer op.Root.Remove(stage) //nolint:errcheck

	_, err = f.WriteString(content)
	if err == nil {
		err = f.Sync()
	}

	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}

	if err = op.Root.Link(stage, path); os.IsExist(err) {
		matches, readErr := op.copyOwnerMatchesStage(path)
		if readErr == nil && matches {
			err = nil
		} else {
			err = errors.Join(err, readErr)
		}
	}

	if err != nil {
		return err
	}

	return Sync(op.Root, ".copy-on-use")
}

func (op OwnerStage) copyOwnerMatchesStage(path string) (bool, error) {
	info, err := op.Root.Lstat(op.Staged)
	if err != nil {
		return false, err
	}

	owner := cleanupOwner{key: op.Key, name: op.Name, path: path}

	return ownerWitnessMatches(op.Root, owner, info), nil
}

// 64-byte binding key, 255-byte filesystem name and two decimal uint64 inode fields.
const maxCopyOwnerSize = 64 + 1 + 255 + 1 + 20 + 1 + 20 + 1

func readCopyOwner(root *os.Root, path string) (key, name string, info os.FileInfo, valid bool, err error) {
	before, err := root.Lstat(path)
	if err != nil {
		return "", "", nil, false, err
	}

	if !before.Mode().IsRegular() || before.Size() < 67 || before.Size() > maxCopyOwnerSize {
		return "", "", before, false, nil
	}

	f, err := root.Open(path)
	if err != nil {
		return "", "", before, false, err
	}

	opened, statErr := f.Stat()

	data, readErr := io.ReadAll(io.LimitReader(f, maxCopyOwnerSize+1))
	if err = errors.Join(statErr, readErr, f.Close()); err != nil {
		return "", "", before, false, err
	}

	if !os.SameFile(before, opened) {
		return "", "", before, false, errors.New("copy owner changed during read")
	}

	parts := strings.Split(string(data), "\n")
	if !validCopyOwnerParts(path, parts) {
		return "", "", before, false, nil
	}

	return parts[0], parts[1], before, true, nil
}

func validCopyOwnerParts(path string, parts []string) bool {
	if len(parts) != 4 || parts[3] != "" || ValidateName(parts[1]) != nil || filepath.Base(path) != filepath.Base(OwnerPath(parts[1])) {
		return false
	}

	if inode, err := strconv.ParseUint(parts[2], 10, 64); err != nil || parts[2] != strconv.FormatUint(inode, 10) {
		return false
	}

	_, err := BindingPath(parts[0])

	return err == nil
}
