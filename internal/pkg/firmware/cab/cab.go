// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cab reads Microsoft Cabinet archives, the container format of LVFS
// firmware releases.
//
// Only what LVFS produces is supported: a single cabinet (no multi-cabinet
// sets) whose folders are stored uncompressed or MSZIP-compressed.
package cab

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

const (
	signature = "MSCF"

	headerSize = 36
	folderSize = 8
	fileSize   = 16
	blockSize  = 8

	versionMajor = 1
	versionMinor = 3

	flagPrevCabinet    = 0x0001
	flagNextCabinet    = 0x0002
	flagReservePresent = 0x0004

	compressionMask  = 0x000f
	compressionNone  = 0
	compressionMSZIP = 1

	// folderContinued is the lowest of the folder indexes that mark a file
	// continued from or into another cabinet of a set.
	folderContinued = 0xfffd

	// maxBlockSize is the uncompressed size limit of a CFDATA block, and the
	// MSZIP history window carried between blocks.
	maxBlockSize = 32768

	// mszipSignature prefixes the deflate stream of every MSZIP block.
	mszipSignature = "CK"

	// MaxFolderSize caps the uncompressed size of a folder.
	MaxFolderSize = 1 << 30
)

// File is an entry of the archive.
type File struct {
	Name string
	Size int

	folder int
	offset int
}

type folder struct {
	dataOffset  int
	blocks      int
	compression uint16

	// size is the uncompressed extent covered by the folder's files.
	size int

	// data is the folder's uncompressed contents, decoded on first use.
	data []byte
}

// Archive is a parsed cabinet.
type Archive struct {
	raw     []byte
	files   []File
	folders []folder

	// blockReserve is the size of the application-reserved area that
	// follows each CFDATA header.
	blockReserve int
}

// Parse reads the structure of a cabinet.
//
// File contents are decoded by Open; data is retained by the archive.
func Parse(data []byte) (*Archive, error) {
	a := &Archive{raw: data}

	r := reader{data: data}

	filesOffset, folderCount, fileCount, folderReserve, err := a.parseHeader(&r)
	if err != nil {
		return nil, err
	}

	for i := range folderCount {
		dataOffset := r.uint32()
		blocks := r.uint16()
		compression := r.uint16()

		r.skip(folderReserve)

		if r.err != nil {
			return nil, fmt.Errorf("folder %d: %w", i, r.err)
		}

		switch compression & compressionMask {
		case compressionNone, compressionMSZIP:
		default:
			return nil, fmt.Errorf("folder %d: unsupported compression type %d", i, compression&compressionMask)
		}

		a.folders = append(a.folders, folder{
			dataOffset:  int(dataOffset),
			blocks:      int(blocks),
			compression: compression & compressionMask,
		})
	}

	r.offset = filesOffset

	for i := range fileCount {
		if err := a.parseFile(&r); err != nil {
			return nil, fmt.Errorf("file %d: %w", i, err)
		}
	}

	return a, nil
}

// parseHeader reads CFHEADER and returns the offset of the first CFFILE, the
// folder and file counts, and the per-CFFOLDER reserve size.
func (a *Archive) parseHeader(r *reader) (filesOffset, folderCount, fileCount, folderReserve int, err error) {
	if len(r.data) < headerSize {
		return 0, 0, 0, 0, errors.New("truncated header")
	}

	if string(r.data[:len(signature)]) != signature {
		return 0, 0, 0, 0, errors.New("not a cabinet file")
	}

	r.skip(4 + len(signature)) // signature, reserved1

	size := r.uint32()

	r.skip(4) // reserved2

	filesOffset = int(r.uint32())

	r.skip(4) // reserved3

	minor, major := r.uint8(), r.uint8()
	folderCount, fileCount = int(r.uint16()), int(r.uint16())
	flags := r.uint16()

	r.skip(4) // setID, iCabinet

	if int(size) > len(r.data) {
		return 0, 0, 0, 0, fmt.Errorf("truncated: %d of %d bytes", len(r.data), size)
	}

	if major != versionMajor || minor != versionMinor {
		return 0, 0, 0, 0, fmt.Errorf("unsupported format version %d.%d", major, minor)
	}

	if flags&(flagPrevCabinet|flagNextCabinet) != 0 {
		return 0, 0, 0, 0, errors.New("multi-cabinet sets are not supported")
	}

	if flags&flagReservePresent != 0 {
		headerReserve := int(r.uint16())
		folderReserve = int(r.uint8())
		a.blockReserve = int(r.uint8())

		r.skip(headerReserve)
	}

	if r.err != nil {
		return 0, 0, 0, 0, fmt.Errorf("header: %w", r.err)
	}

	return filesOffset, folderCount, fileCount, folderReserve, nil
}

// parseFile reads one CFFILE entry and extends its folder's extent.
func (a *Archive) parseFile(r *reader) error {
	size := r.uint32()
	offset := r.uint32()
	folderIndex := int(r.uint16())

	r.skip(6) // date, time, attribs

	name := r.cstring()

	if r.err != nil {
		return r.err
	}

	if folderIndex >= folderContinued {
		return fmt.Errorf("%q spans cabinets", name)
	}

	if folderIndex >= len(a.folders) {
		return fmt.Errorf("%q references folder %d of %d", name, folderIndex, len(a.folders))
	}

	end := int64(offset) + int64(size)
	if end > MaxFolderSize {
		return fmt.Errorf("%q extends to %d bytes, limit is %d", name, end, MaxFolderSize)
	}

	a.folders[folderIndex].size = max(a.folders[folderIndex].size, int(end))

	a.files = append(a.files, File{
		Name:   name,
		Size:   int(size),
		folder: folderIndex,
		offset: int(offset),
	})

	return nil
}

// Files lists the entries in archive order.
func (a *Archive) Files() []File {
	return a.files
}

// Open returns a copy of the named entry's contents.
//
// A missing entry is reported as fs.ErrNotExist.
func (a *Archive) Open(name string) ([]byte, error) {
	for i := range a.files {
		f := &a.files[i]

		if f.Name != name {
			continue
		}

		data, err := a.folderData(f.folder)
		if err != nil {
			return nil, err
		}

		return bytes.Clone(data[f.offset : f.offset+f.Size]), nil
	}

	return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
}

// folderData returns the uncompressed contents of a folder, decoding its
// CFDATA blocks on first use.
func (a *Archive) folderData(index int) ([]byte, error) {
	f := &a.folders[index]

	if f.data != nil {
		return f.data, nil
	}

	// the declared size and block count are untrusted: never reserve more
	// than the archive itself holds, and grow as blocks actually decode
	out := make([]byte, 0, min(f.size, f.blocks*maxBlockSize, len(a.raw)))

	r := reader{data: a.raw, offset: f.dataOffset}

	for block := range f.blocks {
		// data past the last file is never read, so it isn't decoded either,
		// which bounds the output by the declared, limited, size
		if len(out) >= f.size {
			break
		}

		chunk, err := a.decodeBlock(&r, f.compression, out)
		if err != nil {
			return nil, fmt.Errorf("folder %d block %d: %w", index, block, err)
		}

		out = append(out, chunk...)
	}

	if len(out) < f.size {
		return nil, fmt.Errorf("folder %d: %d bytes of data for %d bytes of files", index, len(out), f.size)
	}

	f.data = out

	return out, nil
}

// decodeBlock reads one CFDATA block at the reader's position and returns its
// uncompressed contents; history is the folder data decoded so far, which
// MSZIP blocks may reference.
func (a *Archive) decodeBlock(r *reader, compression uint16, history []byte) ([]byte, error) {
	checksum := r.uint32()
	compressedSize := r.uint16()
	uncompressedSize := r.uint16()

	r.skip(a.blockReserve)

	payload := r.bytes(int(compressedSize))

	if r.err != nil {
		return nil, r.err
	}

	if uncompressedSize == 0 || uncompressedSize > maxBlockSize {
		return nil, fmt.Errorf("invalid uncompressed size %d", uncompressedSize)
	}

	if checksum != 0 && blockChecksum(payload, compressedSize, uncompressedSize) != checksum {
		return nil, errors.New("checksum mismatch")
	}

	switch compression {
	case compressionNone:
		if compressedSize != uncompressedSize {
			return nil, fmt.Errorf("stored block of %d bytes declares %d uncompressed", compressedSize, uncompressedSize)
		}

		return payload, nil
	case compressionMSZIP:
		return inflate(payload, history, int(uncompressedSize))
	default:
		panic("unreachable: compression type checked by Parse")
	}
}

// inflate decodes an MSZIP block: a "CK" signature followed by a raw deflate
// stream whose history window continues from the previous block.
func inflate(payload, history []byte, size int) ([]byte, error) {
	if !bytes.HasPrefix(payload, []byte(mszipSignature)) {
		return nil, errors.New("missing MSZIP signature")
	}

	dict := history
	if len(dict) > maxBlockSize {
		dict = dict[len(dict)-maxBlockSize:]
	}

	fr := flate.NewReaderDict(bytes.NewReader(payload[len(mszipSignature):]), dict)
	defer fr.Close() //nolint:errcheck

	out := make([]byte, size)

	if _, err := io.ReadFull(fr, out); err != nil {
		return nil, fmt.Errorf("inflate: %w", err)
	}

	var extra [1]byte

	n, err := fr.Read(extra[:])
	if n != 0 {
		return nil, fmt.Errorf("inflate: block larger than the declared %d bytes", size)
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("inflate: %w", err)
	}

	return out, nil
}

// blockChecksum computes the CFDATA checksum: the payload folded into the
// checksum of the size fields, as in the cabinet specification.
func blockChecksum(payload []byte, compressedSize, uncompressedSize uint16) uint32 {
	var sizes [4]byte

	binary.LittleEndian.PutUint16(sizes[0:], compressedSize)
	binary.LittleEndian.PutUint16(sizes[2:], uncompressedSize)

	return checksum(sizes[:], checksum(payload, 0))
}

// checksum XORs data as little-endian 32-bit words into seed, folding a
// partial trailing word in big-endian order.
func checksum(data []byte, seed uint32) uint32 {
	sum := seed

	for ; len(data) >= 4; data = data[4:] {
		sum ^= binary.LittleEndian.Uint32(data)
	}

	var tail uint32

	switch len(data) {
	case 3:
		tail = uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	case 2:
		tail = uint32(data[0])<<8 | uint32(data[1])
	case 1:
		tail = uint32(data[0])
	}

	return sum ^ tail
}

// reader is a bounds-checked little-endian cursor; the first overrun sticks
// in err and every later read returns zero.
type reader struct {
	data   []byte
	offset int
	err    error
}

func (r *reader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}

	if n < 0 || r.offset > len(r.data)-n {
		r.err = fmt.Errorf("truncated at offset %d", r.offset)

		return nil
	}

	b := r.data[r.offset : r.offset+n]
	r.offset += n

	return b
}

func (r *reader) skip(n int) {
	r.bytes(n)
}

func (r *reader) uint8() uint8 {
	b := r.bytes(1)
	if b == nil {
		return 0
	}

	return b[0]
}

func (r *reader) uint16() uint16 {
	b := r.bytes(2)
	if b == nil {
		return 0
	}

	return binary.LittleEndian.Uint16(b)
}

func (r *reader) uint32() uint32 {
	b := r.bytes(4)
	if b == nil {
		return 0
	}

	return binary.LittleEndian.Uint32(b)
}

// cstring reads a NUL-terminated string.
func (r *reader) cstring() string {
	if r.err != nil {
		return ""
	}

	end := bytes.IndexByte(r.data[r.offset:], 0)
	if end < 0 {
		r.err = fmt.Errorf("unterminated string at offset %d", r.offset)

		return ""
	}

	s := string(r.data[r.offset : r.offset+end])
	r.offset += end + 1

	return s
}
