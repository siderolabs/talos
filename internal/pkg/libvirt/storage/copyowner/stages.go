// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copyowner

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

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

// StageName returns the name a volume of name is built under.
//
// Unique per attempt, so a staged file left behind by an interrupted create can never block a
// later one of the same name.
func StageName(name string) string {
	return fmt.Sprintf("%s%s.%016x%s", stagedVolumePrefix, name, rand.Uint64(), stagedVolumeSuffix)
}

// IsStageName reports whether name is a volume staged by StageName.
func IsStageName(name string) bool {
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

// activeStages records in-flight stage names. The shared mutex guards only the
// maps; each pool directory has its own lock, held across that pool's sweep or
// collection I/O, so a slow fsync in one pool never blocks staging in another.
var activeStages = struct {
	sync.Mutex
	paths map[string]struct{}
	pools map[string]*sync.Mutex
}{paths: make(map[string]struct{}), pools: make(map[string]*sync.Mutex)}

func stageKey(target, name string) string { return filepath.Clean(target) + "/" + name }

// lockPool serializes stage registration with sweeps and collection of one pool.
func lockPool(target string) func() {
	activeStages.Lock()

	lock := activeStages.pools[filepath.Clean(target)]
	if lock == nil {
		lock = &sync.Mutex{}
		activeStages.pools[filepath.Clean(target)] = lock
	}

	activeStages.Unlock()

	lock.Lock()

	return lock.Unlock
}

func stageActive(target, name string) bool {
	activeStages.Lock()
	defer activeStages.Unlock()

	_, active := activeStages.paths[stageKey(target, name)]

	return active
}

// activeStagesIn snapshots the stage names registered under target.
func activeStagesIn(target string) []string {
	prefix := filepath.Clean(target) + "/"

	activeStages.Lock()
	defer activeStages.Unlock()

	var stages []string

	for key := range activeStages.paths {
		if stage, ok := strings.CutPrefix(key, prefix); ok {
			stages = append(stages, stage)
		}
	}

	return stages
}

// ProtectStage registers an in-flight stage so sweeps and collection leave it
// alone; the returned function releases it.
func ProtectStage(target, name string) func() {
	key := stageKey(target, name)

	unlock := lockPool(target)

	activeStages.Lock()
	activeStages.paths[key] = struct{}{}
	activeStages.Unlock()
	unlock()

	return func() {
		activeStages.Lock()
		delete(activeStages.paths, key)
		activeStages.Unlock()
	}
}

// SweepStages clears what interrupted creates left behind.
//
// Only names this package stages are removed. A volume is never deleted here: a staged file is one
// no virtual machine has ever been told about, which is what makes clearing it safe.
func SweepStages(poolDir string) error {
	defer lockPool(poolDir)()

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
		if stageActive(poolDir, entry.Name()) {
			continue
		}

		if !isStagedVolumeEntry(entry) {
			continue
		}

		if err = root.Remove(entry.Name()); err != nil && !os.IsNotExist(err) {
			errs = errors.Join(errs, err)
		}
	}

	return errors.Join(errs, sweepPrivateStages(root, poolDir))
}

// sweepPrivateStages removes only interrupted metadata stages, never published
// ownership markers or binding records. The same lock protects live writers.
func sweepPrivateStages(root *os.Root, poolDir string) error {
	info, err := root.Lstat(".copy-on-use")
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if !info.IsDir() {
		return errors.New("copy ownership directory is not a directory")
	}

	dir, err := root.Open(".copy-on-use")
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
		errs = errors.Join(errs, removeInactivePrivateStage(root, poolDir, entry))
	}

	return errs
}

func removeInactivePrivateStage(root *os.Root, poolDir string, entry os.DirEntry) error {
	stage, ok := privateStage(entry)
	if !ok {
		return nil
	}

	if stageActive(poolDir, stage) {
		return nil
	}

	err := root.Remove(filepath.Join(".copy-on-use", entry.Name()))
	if os.IsNotExist(err) {
		return nil
	}

	return err
}

func privateStage(entry os.DirEntry) (string, bool) {
	if !entry.Type().IsRegular() {
		return "", false
	}

	name := entry.Name()

	stage := strings.TrimSuffix(name, ".owner")
	if strings.HasSuffix(name, ".owner") {
		return stage, IsStageName(stage)
	}

	if !strings.HasPrefix(name, ".binding-") || !IsStageName(name) {
		return "", false
	}

	key := strings.TrimPrefix(strings.TrimSuffix(name, stagedVolumeSuffix), ".binding-")
	key = key[:strings.LastIndexByte(key, '.')]
	_, err := BindingPath(key)

	return name, err == nil
}

func isStagedVolumeEntry(entry os.DirEntry) bool {
	return entry.Type().IsRegular() && IsStageName(entry.Name())
}
