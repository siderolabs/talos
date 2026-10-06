// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cloudinit builds NoCloud ISO9660 seeds without interpreting guest payloads.
package cloudinit

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

// Seed is the verbatim content of the NoCloud files.
type Seed struct {
	MetaData      string
	UserData      string
	NetworkConfig string
}

// MaxSeedBytes limits the work and disk consumed by one guest configuration.
const MaxSeedBytes = 4 << 20

// WriteISO creates a private ISO file at path. The caller owns the staging path and publication.
// A failure removes its partial file; the destination must not already exist.
func WriteISO(ctx context.Context, path string, seed Seed) (err error) {
	if err = validateSeed(ctx, seed); err != nil {
		return err
	}

	workspace, err := os.MkdirTemp(filepath.Dir(path), ".cloud-init-work-")
	if err != nil {
		return fmt.Errorf("create private ISO workspace: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(workspace); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove private ISO workspace: %w", removeErr))
		}
	}()

	if err = writeSeedFiles(ctx, workspace, seed); err != nil {
		return err
	}

	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create ISO: %w", err)
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = closeErr
		}

		if err != nil {
			if removeErr := os.Remove(path); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("remove partial ISO: %w", removeErr))
			}
		}
	}()

	return finalizeISO(ctx, out, workspace)
}

func validateSeed(ctx context.Context, seed Seed) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(seed.MetaData)+len(seed.UserData)+len(seed.NetworkConfig) > MaxSeedBytes {
		return fmt.Errorf("cloud-init seed exceeds %d bytes", MaxSeedBytes)
	}

	return nil
}

// writeSeedFiles stages the verbatim NoCloud files with stable timestamps.
func writeSeedFiles(ctx context.Context, workspace string, seed Seed) error {
	stamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	files := map[string]string{
		"meta-data": seed.MetaData,
		"user-data": seed.UserData,
	}

	if seed.NetworkConfig != "" {
		files["network-config"] = seed.NetworkConfig
	}

	for _, name := range []string{"meta-data", "user-data", "network-config"} {
		contents, ok := files[name]
		if !ok {
			continue
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		target := filepath.Join(workspace, name)
		if err := os.WriteFile(target, []byte(contents), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}

		if err := os.Chtimes(target, stamp, stamp); err != nil {
			return fmt.Errorf("timestamp %s: %w", name, err)
		}
	}

	if err := os.Chtimes(workspace, stamp, stamp); err != nil {
		return fmt.Errorf("timestamp workspace: %w", err)
	}

	return nil
}

// finalizeISO writes the filesystem, normalizes the descriptor, and syncs the output.
func finalizeISO(ctx context.Context, out *os.File, workspace string) error {
	fs, err := iso9660.Create(file.New(out, false), 0, 0, 0, workspace)
	if err != nil {
		return fmt.Errorf("create ISO filesystem: %w", err)
	}

	if err = fs.Finalize(iso9660.FinalizeOptions{RockRidge: true, VolumeIdentifier: "CIDATA"}); err != nil {
		return fmt.Errorf("finalize ISO filesystem: %w", err)
	}

	if err = normalizeRockRidgeTimes(out); err != nil {
		return fmt.Errorf("normalize Rock Ridge timestamps: %w", err)
	}
	// go-diskfs stamps the primary descriptor with wall time. ISO9660 defines four
	// 17-byte timestamps at offsets 813..880; normalize these after finalization.
	fixed := []byte("2000010100000000\x00")
	for offset := int64(813); offset < 881; offset += 17 {
		if _, err = out.WriteAt(fixed, 16*2048+offset); err != nil {
			return fmt.Errorf("normalize ISO descriptor: %w", err)
		}
	}

	if err = ctx.Err(); err != nil {
		return err
	}

	if err = out.Sync(); err != nil {
		return fmt.Errorf("sync ISO: %w", err)
	}

	if _, err = out.Seek(0, io.SeekStart); err != nil {
		return err
	}

	return nil
}

const isoSectorSize = 2048

// Rock Ridge TF attribute time comes from filesystem ctime, which Chtimes cannot set.
func normalizeRockRidgeTimes(out *os.File) error {
	info, err := out.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	if fileSize < 19*isoSectorSize || fileSize > MaxSeedBytes+128*1024 {
		return fmt.Errorf("unexpected NoCloud ISO size %d", fileSize)
	}

	block, base, length, err := readNoCloudRoot(out, fileSize)
	if err != nil {
		return err
	}

	// Validate the complete flat directory before writing any timestamps. In
	// particular, CE must not point into the payload of a NoCloud file.
	positions, payloadStart, err := noCloudRecordPositions(block[:length], fileSize)
	if err != nil {
		return err
	}

	for _, pos := range positions {
		record := block[pos : pos+int(block[pos])]

		start, err := suspOffset(record)
		if err != nil {
			return fmt.Errorf("directory record %d: %w", pos, err)
		}

		found := false
		if err = normalizeSUSP(out, record[start:], base+int64(pos+start), base, payloadStart, &found, true); err != nil {
			return fmt.Errorf("directory record %d: %w", pos, err)
		}

		if !found {
			return fmt.Errorf("missing Rock Ridge TF in directory record %d", pos)
		}
	}

	return nil
}

func readNoCloudRoot(out *os.File, fileSize int64) ([isoSectorSize]byte, int64, int, error) {
	var block [isoSectorSize]byte

	var pvd [isoSectorSize]byte
	if _, err := out.ReadAt(pvd[:], 16*isoSectorSize); err != nil {
		return block, 0, 0, err
	}

	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" || pvd[6] != 1 {
		return block, 0, 0, fmt.Errorf("unexpected ISO primary volume descriptor")
	}

	root := pvd[156 : 156+34]
	location := binary.LittleEndian.Uint32(root[2:6])
	length := binary.LittleEndian.Uint32(root[10:14])
	base := int64(location) * isoSectorSize

	if !validNoCloudRoot(root, length, base, fileSize) {
		return block, 0, 0, fmt.Errorf("unexpected NoCloud root directory: record=%d flags=%d name=%d/%d extent=%d length=%d file=%d", root[0], root[25], root[32], root[33], location, length, fileSize)
	}

	if _, err := out.ReadAt(block[:], base); err != nil {
		return block, 0, 0, err
	}

	return block, base, int(length), nil
}

func validNoCloudRoot(root []byte, length uint32, base, fileSize int64) bool {
	return root[0] == 34 && root[25]&2 != 0 && root[32] == 1 && root[33] == 0 &&
		length > 0 && length <= isoSectorSize && base == 18*isoSectorSize && base+isoSectorSize <= fileSize
}

func noCloudRecordPositions(block []byte, fileSize int64) ([]int, int64, error) {
	positions := make([]int, 0, 5)
	payloadStart := fileSize

	for pos := 0; pos < len(block); {
		if block[pos] == 0 {
			if err := validDirectoryPadding(block[pos:]); err != nil {
				return nil, 0, fmt.Errorf("invalid NoCloud directory padding at %d: %w", pos, err)
			}

			break
		}

		size := int(block[pos])
		if size < 34 || pos+size > len(block) {
			return nil, 0, fmt.Errorf("invalid NoCloud directory record at %d", pos)
		}

		record := block[pos : pos+size]

		extent, err := validateNoCloudRecord(record, len(positions), fileSize)
		if err != nil {
			return nil, 0, fmt.Errorf("directory record %d: %w", pos, err)
		}

		if extent < payloadStart {
			payloadStart = extent
		}

		positions = append(positions, pos)
		pos += size
	}

	// Two dot records and two or three NoCloud files. Fail closed if the
	// library changes its directory layout instead of publishing a partial fix.
	if len(positions) < 4 || len(positions) > 5 {
		return nil, 0, fmt.Errorf("unexpected NoCloud directory record count %d", len(positions))
	}

	return positions, payloadStart, nil
}

func validDirectoryPadding(padding []byte) error {
	for _, b := range padding {
		if b != 0 {
			return fmt.Errorf("nonzero byte after end of records")
		}
	}

	return nil
}

func validateNoCloudRecord(record []byte, index int, fileSize int64) (int64, error) {
	if _, err := suspOffset(record); err != nil {
		return 0, err
	}

	if index < 2 {
		if record[25]&2 == 0 || record[32] != 1 || record[33] != byte(index) {
			return 0, fmt.Errorf("unexpected NoCloud dot record")
		}

		return fileSize, nil
	}

	if record[25]&2 != 0 {
		return 0, fmt.Errorf("unexpected NoCloud subdirectory")
	}

	extent := int64(binary.LittleEndian.Uint32(record[2:6])) * isoSectorSize

	length := int64(binary.LittleEndian.Uint32(record[10:14]))
	if extent < 19*isoSectorSize || extent > fileSize-length {
		return 0, fmt.Errorf("invalid NoCloud file extent")
	}

	return extent, nil
}

func suspOffset(record []byte) (int, error) {
	nameLength := int(record[32])

	start := 33 + nameLength
	if nameLength == 0 || start > len(record) {
		return 0, fmt.Errorf("invalid NoCloud directory name")
	}

	if nameLength%2 == 0 {
		start++
	}

	if start > len(record) {
		return 0, fmt.Errorf("invalid NoCloud directory name padding")
	}

	return start, nil
}

// CE addresses a separate continuation area, not the next directory bytes.
// Bound it to one sector, before file data, and reject nested CE.
func normalizeSUSP(out *os.File, fields []byte, base, rootBase, payloadStart int64, found *bool, allowCE bool) error {
	ceSeen := false

	for len(fields) > 0 {
		if len(fields) == 1 && fields[0] == 0 { // even-length record padding
			return nil
		}

		if len(fields) < 4 {
			return fmt.Errorf("truncated SUSP field")
		}

		size := int(fields[2])
		if size < 4 || size > len(fields) || fields[3] != 1 {
			return fmt.Errorf("invalid SUSP field")
		}

		field := fields[:size]
		if err := normalizeSUSPField(out, field, base, rootBase, payloadStart, found, allowCE, &ceSeen); err != nil {
			return err
		}

		base += int64(size)
		fields = fields[size:]
	}

	return nil
}

func normalizeSUSPField(out *os.File, field []byte, base, rootBase, payloadStart int64, found *bool, allowCE bool, ceSeen *bool) error {
	switch string(field[:2]) {
	case "TF":
		return normalizeTF(out, field, base, found)
	case "CE":
		if !allowCE || *ceSeen {
			return fmt.Errorf("unexpected SUSP continuation")
		}

		*ceSeen = true

		return normalizeContinuation(out, field, rootBase, payloadStart, found)
	}

	return nil
}

func normalizeTF(out *os.File, field []byte, base int64, found *bool) error {
	// Pinned writer: short-form modify, access, attribute (0x0e).
	if *found || len(field) != 26 || field[4] != 0x0e {
		return fmt.Errorf("unexpected Rock Ridge TF layout")
	}

	*found = true

	_, err := out.WriteAt([]byte{100, 1, 1, 0, 0, 0, 0}, base+19)

	return err
}

func normalizeContinuation(out *os.File, field []byte, rootBase, payloadStart int64, found *bool) error {
	if len(field) != 28 {
		return fmt.Errorf("unexpected SUSP continuation")
	}

	location := binary.LittleEndian.Uint32(field[4:8])
	offset := binary.LittleEndian.Uint32(field[12:16])
	length := binary.LittleEndian.Uint32(field[20:24])
	start := int64(location)*isoSectorSize + int64(offset)

	// Continuations belong in metadata, not the root directory or a file's
	// payload. The one-sector limit also bounds memory and recursion depth.
	if length == 0 || int64(offset)+int64(length) > isoSectorSize ||
		start < 19*isoSectorSize || start < rootBase+isoSectorSize && start+int64(length) > rootBase ||
		start > payloadStart-int64(length) {
		return fmt.Errorf("invalid SUSP continuation extent")
	}

	var continuation [isoSectorSize]byte
	if _, err := out.ReadAt(continuation[:length], start); err != nil {
		return err
	}

	return normalizeSUSP(out, continuation[:length], start, rootBase, payloadStart, found, false)
}
