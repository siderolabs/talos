// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vfat

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// FileSystem provides simple way to read files from FAT32 filesystems.
//
// This code is far from being production quality, it might break if filesystem
// is corrupted or is not actually FAT32.
type FileSystem struct {
	sb *SuperBlock
	f  io.ReaderAt

	sectorSize  uint16
	clusterSize uint32
	fatSize     uint64
	fatOffset   uint32
	dataStart   uint64

	fat map[uint32]uint32
}

// NewFileSystem initializes Filesystem, reads FAT.
func NewFileSystem(f io.ReaderAt, sb *SuperBlock) (*FileSystem, error) {
	fs := &FileSystem{
		sb: sb,
		f:  f,
	}

	fs.sectorSize = binary.LittleEndian.Uint16(sb.SectorSize[:])
	fs.clusterSize = uint32(sb.ClusterSize) * uint32(fs.sectorSize)
	fs.fatSize = uint64(fs.sectorSize) * uint64(binary.LittleEndian.Uint32(sb.Fat32Length[:]))
	fs.fatOffset = uint32(binary.LittleEndian.Uint16(sb.Reserved[:])) * uint32(fs.sectorSize)
	fs.dataStart = uint64(fs.fatOffset) + 2*fs.fatSize

	rawFat := make([]byte, fs.fatSize)

	if err := readAtFull(fs.f, int64(fs.fatOffset), rawFat); err != nil {
		return nil, fmt.Errorf("error reading FAT: %w", err)
	}

	fs.fat = make(map[uint32]uint32)

	for i := uint32(2); i < uint32(fs.fatSize/4); i++ {
		c := binary.LittleEndian.Uint32(rawFat[i*4 : i*4+4])

		if c != 0 {
			fs.fat[i] = c
		}
	}

	return fs, nil
}

// Open the file as read-only stream on the filesystem.
func (fs *FileSystem) Open(path string) (*File, error) {
	components := strings.Split(path, string(os.PathSeparator))

	// start with rootDirectory
	dir := &directory{
		fs:           fs,
		firstCluster: binary.LittleEndian.Uint32(fs.sb.RootCluster[:]),
	}

	for i, component := range components {
		if component == "" {
			continue
		}

		entries, err := dir.scan()
		if err != nil {
			return nil, err
		}

		found := false

		for _, entry := range entries {
			if entry.filename == component {
				found = true

				if i == len(components)-1 {
					// should be a file
					if entry.isDirectory {
						return nil, fmt.Errorf("expected file entry, but directory found")
					}

					return &File{
						fs:    fs,
						chain: fs.fatChain(entry.firstCluster),
						size:  entry.size,
					}, nil
				}

				if !entry.isDirectory {
					return nil, fmt.Errorf("expected directory, but file found")
				}

				dir = &directory{
					fs:           fs,
					firstCluster: entry.firstCluster,
				}

				break
			}
		}

		if !found {
			return nil, os.ErrNotExist
		}
	}

	return nil, os.ErrNotExist
}

// fatChain results list of clusters for a file (or directory) based on first
// cluster number and FAT.
func (fs *FileSystem) fatChain(firstCluster uint32) (chain []uint32) {
	chain = []uint32{firstCluster}

	for {
		next := fs.fat[firstCluster]
		if next == 0 || next&0xFFFFFF8 == 0xFFFFFF8 {
			return
		}

		chain = append(chain, next)
		firstCluster = next
	}
}

type directory struct {
	fs           *FileSystem
	firstCluster uint32
}

type directoryEntry struct {
	filename    string
	isDirectory bool

	size         uint32
	firstCluster uint32
}

// scan a directory building list of entries.
//
// Only LFN are supported, entries without LFN are ignored.
func (d *directory) scan() ([]directoryEntry, error) {
	// read whole directory into memory
	chain := d.fs.fatChain(d.firstCluster)
	raw := make([]byte, uint32(len(chain))*d.fs.clusterSize)

	for i, cluster := range chain {
		if err := readAtFull(d.fs.f, int64(d.fs.dataStart)+int64(cluster-2)*int64(d.fs.clusterSize), raw[uint32(i)*d.fs.clusterSize:uint32(i+1)*d.fs.clusterSize]); err != nil {
			return nil, fmt.Errorf("error reading directory contents: %w", err)
		}
	}

	var (
		entries []directoryEntry
		lfn     string
	)

	for i := 0; i < len(raw); i += 32 {
		entry := raw[i : i+32]

		if entry[0] == 0 {
			return entries, nil
		}

		if entry[0] == 0xe5 {
			continue
		}

		if entry[11] == 0x0f {
			if entry[0]&0x40 == 0x40 {
				lfn = ""
			}

			lfn = parseLfn(entry) + lfn

			continue
		}

		if lfn == "" {
			// no lfn, skip directory entry
			continue
		}

		entries = append(entries, directoryEntry{
			filename:     lfn,
			isDirectory:  entry[11]&0x10 == 0x10,
			size:         binary.LittleEndian.Uint32(entry[28:32]),
			firstCluster: binary.LittleEndian.Uint32(append(entry[26:28], entry[20:22]...)),
		})
	}

	return entries, nil
}

// File represents a VFAT backed file.
type File struct {
	fs     *FileSystem
	chain  []uint32
	offset uint32
	size   uint32
}

func (f *File) Read(p []byte) (n int, err error) {
	remaining := len(p)
	if uint32(remaining) > f.size-f.offset {
		remaining = int(f.size - f.offset)
		if remaining == 0 {
			err = io.EOF
			return
		}
	}

	for remaining > 0 {
		clusterIdx := f.offset / f.fs.clusterSize
		clusterOffset := f.offset % f.fs.clusterSize

		if clusterIdx > uint32(len(f.chain)) {
			err = fmt.Errorf("FAT chain overrun")
			return
		}

		cluster := f.chain[clusterIdx]
		readLen := f.fs.clusterSize - clusterOffset

		if readLen > uint32(remaining) {
			readLen = uint32(remaining)
		}

		if err = readAtFull(f.fs.f, int64(f.fs.dataStart)+int64(cluster-2)*int64(f.fs.clusterSize)+int64(clusterOffset), p[:readLen]); err != nil {
			return
		}

		remaining -= int(readLen)
		n += int(readLen)
		f.offset += readLen

		p = p[readLen:]
	}

	return n, err
}

// Seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		f.offset = uint32(offset)
	case 1:
		f.offset = uint32(int64(f.offset) + offset)
	case 2:
		f.offset = uint32(int64(f.size) + offset)
	default:
		return 0, fmt.Errorf("unknown whence: %d", whence)
	}

	return int64(f.offset), nil
}

func readAtFull(r io.ReaderAt, off int64, buf []byte) error {
	remaining := len(buf)

	for remaining > 0 {
		n, err := r.ReadAt(buf, off)
		if err != nil {
			return err
		}

		remaining -= n
		off += int64(n)
		buf = buf[n:]
	}

	return nil
}

func parseLfn(entry []byte) string {
	raw := append(entry[1:11], append(entry[14:26], entry[28:32]...)...)

	parsed := []rune{}

	for i := 0; i < len(raw); i += 2 {
		val := binary.LittleEndian.Uint16(raw[i : i+2])
		if val == 0 {
			break
		}

		parsed = append(parsed, rune(val))
	}

	return string(parsed)
}
