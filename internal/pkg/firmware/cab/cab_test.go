// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // the checksum is verified against specification vectors.
package cab

import (
	"bytes"
	"cmp"
	"compress/flate"
	"encoding/binary"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFile struct {
	name string
	data []byte

	// folderIndex and size override the computed CFFILE fields when non-nil.
	folderIndex *uint16
	size        *uint32
}

type testFolder struct {
	compression uint16
	files       []testFile
}

// testCab describes a cabinet for build.
type testCab struct {
	folders []testFolder

	headerReserve int
	folderReserve int
	blockReserve  int

	// flags is OR-ed into the header flags; the reserve flag is set
	// automatically when any reserve size is non-zero.
	flags uint16

	versionMinor uint8

	// zeroChecksums writes 0 in place of every CFDATA checksum.
	zeroChecksums bool
}

func file(name string, data []byte) testFile {
	return testFile{name: name, data: data}
}

func stored(files ...testFile) testFolder {
	return testFolder{compression: compressionNone, files: files}
}

func mszip(files ...testFile) testFolder {
	return testFolder{compression: compressionMSZIP, files: files}
}

func (c testCab) reserve() bool {
	return c.headerReserve != 0 || c.folderReserve != 0 || c.blockReserve != 0
}

// build encodes a cabinet the way gcab (the tool LVFS uses) lays it out:
// header, folders, files, then the data blocks of each folder in turn.
func build(t *testing.T, c testCab) []byte {
	t.Helper()

	var files, data bytes.Buffer

	folderStarts := make([]int, len(c.folders))
	fileCount := 0

	for i, folder := range c.folders {
		var contents []byte

		for _, f := range folder.files {
			files.Write(fileEntry(f, i, len(contents)))

			contents = append(contents, f.data...)
			fileCount++
		}

		folderStarts[i] = data.Len()

		writeBlocks(t, &data, folder.compression, contents, c.blockReserve, c.zeroChecksums)
	}

	headerLen := headerSize
	if c.reserve() {
		headerLen += 4 + c.headerReserve
	}

	foldersLen := len(c.folders) * (folderSize + c.folderReserve)
	filesOffset := headerLen + foldersLen
	dataOffset := filesOffset + files.Len()

	var out bytes.Buffer

	out.Write(header(c, dataOffset+data.Len(), filesOffset, fileCount))

	for i, folder := range c.folders {
		entry := make([]byte, folderSize)
		binary.LittleEndian.PutUint32(entry[0:], uint32(dataOffset+folderStarts[i]))
		binary.LittleEndian.PutUint16(entry[4:], uint16(blockCount(folder)))
		binary.LittleEndian.PutUint16(entry[6:], folder.compression)
		out.Write(entry)
		out.Write(bytes.Repeat([]byte{0xbb}, c.folderReserve))
	}

	out.Write(files.Bytes())
	out.Write(data.Bytes())

	return out.Bytes()
}

// fileEntry encodes a CFFILE entry for a file at an offset in its folder.
func fileEntry(f testFile, folder, offset int) []byte {
	size := uint32(len(f.data))
	if f.size != nil {
		size = *f.size
	}

	index := uint16(folder)
	if f.folderIndex != nil {
		index = *f.folderIndex
	}

	entry := make([]byte, fileSize, fileSize+len(f.name)+1)
	binary.LittleEndian.PutUint32(entry[0:], size)
	binary.LittleEndian.PutUint32(entry[4:], uint32(offset))
	binary.LittleEndian.PutUint16(entry[8:], index)

	entry = append(entry, f.name...)

	return append(entry, 0)
}

// header encodes CFHEADER with its optional reserve area.
func header(c testCab, total, filesOffset, fileCount int) []byte {
	out := make([]byte, headerSize, headerSize+4+c.headerReserve)
	copy(out, signature)
	binary.LittleEndian.PutUint32(out[8:], uint32(total))
	binary.LittleEndian.PutUint32(out[16:], uint32(filesOffset))
	out[24] = cmp.Or(c.versionMinor, versionMinor)
	out[25] = versionMajor
	binary.LittleEndian.PutUint16(out[26:], uint16(len(c.folders)))
	binary.LittleEndian.PutUint16(out[28:], uint16(fileCount))
	binary.LittleEndian.PutUint16(out[30:], c.flags)

	if !c.reserve() {
		return out
	}

	binary.LittleEndian.PutUint16(out[30:], c.flags|flagReservePresent)

	var r [4]byte

	binary.LittleEndian.PutUint16(r[0:], uint16(c.headerReserve))
	r[2] = uint8(c.folderReserve)
	r[3] = uint8(c.blockReserve)

	out = append(out, r[:]...)

	return append(out, bytes.Repeat([]byte{0xaa}, c.headerReserve)...)
}

func blockCount(folder testFolder) int {
	size := 0
	for _, f := range folder.files {
		size += len(f.data)
	}

	return (size + maxBlockSize - 1) / maxBlockSize
}

func writeBlocks(t *testing.T, w *bytes.Buffer, compression uint16, contents []byte, reserve int, zeroChecksums bool) {
	t.Helper()

	for start := 0; start < len(contents); start += maxBlockSize {
		chunk := contents[start:min(start+maxBlockSize, len(contents))]

		payload := chunk

		if compression == compressionMSZIP {
			dict := contents[max(0, start-maxBlockSize):start]

			var compressed bytes.Buffer

			compressed.WriteString(mszipSignature)

			fw, err := flate.NewWriterDict(&compressed, flate.BestCompression, dict)
			require.NoError(t, err)

			_, err = fw.Write(chunk)
			require.NoError(t, err)
			require.NoError(t, fw.Close())

			payload = compressed.Bytes()
		}

		header := make([]byte, blockSize)

		if !zeroChecksums {
			binary.LittleEndian.PutUint32(header[0:], blockChecksum(payload, uint16(len(payload)), uint16(len(chunk))))
		}

		binary.LittleEndian.PutUint16(header[4:], uint16(len(payload)))
		binary.LittleEndian.PutUint16(header[6:], uint16(len(chunk)))

		w.Write(header)
		w.Write(bytes.Repeat([]byte{0xcc}, reserve))
		w.Write(payload)
	}
}

// patterned returns data with repetition across block boundaries, so MSZIP
// blocks reference the history of the previous one.
func patterned(n int) []byte {
	out := make([]byte, n)

	for i := range out {
		out[i] = byte((i * 7) % 251)
	}

	return out
}

// firstBlock returns the offset of the first CFDATA block of the first folder
// in a cabinet built without reserve fields.
func firstBlock(data []byte) int {
	return int(binary.LittleEndian.Uint32(data[headerSize:]))
}

func TestChecksumVectors(t *testing.T) {
	t.Parallel()

	// computed with the reference algorithm from the cabinet specification
	for _, tc := range []struct {
		data  string
		plain uint32
		block uint32
	}{
		{"", 0x0, 0x0},
		{"a", 0x61, 0x10060},
		{"ab", 0x6162, 0x26160},
		{"abc", 0x616263, 0x626260},
		{"abcd", 0x64636261, 0x64676265},
		{"abcdefg", 0x64060406, 0x64010401},
		{"MSCF\x00\x01\x02", 0x4643524f, 0x46445248},
	} {
		assert.Equal(t, tc.plain, checksum([]byte(tc.data), 0), "checksum(%q)", tc.data)
		assert.Equal(t, tc.block, blockChecksum([]byte(tc.data), uint16(len(tc.data)), uint16(len(tc.data))), "blockChecksum(%q)", tc.data)
	}
}

func TestStored(t *testing.T) {
	t.Parallel()

	metainfo := []byte("<component><id>com.example.firmware</id></component>")
	image := patterned(70000)

	data := build(t, testCab{folders: []testFolder{
		stored(file("firmware.metainfo.xml", metainfo), file("firmware.bin", image)),
		stored(file("README.txt", []byte("readme"))),
	}})

	a, err := Parse(data)
	require.NoError(t, err)

	names := make([]string, 0, len(a.Files()))

	for _, f := range a.Files() {
		names = append(names, f.Name)
	}

	assert.Equal(t, []string{"firmware.metainfo.xml", "firmware.bin", "README.txt"}, names)
	assert.Equal(t, len(image), a.Files()[1].Size)

	got, err := a.Open("firmware.bin")
	require.NoError(t, err)
	assert.Equal(t, image, got)

	got, err = a.Open("firmware.metainfo.xml")
	require.NoError(t, err)
	assert.Equal(t, metainfo, got)

	got, err = a.Open("README.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("readme"), got)

	// Open hands out copies
	got[0] = 'X'

	got, err = a.Open("README.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("readme"), got)

	_, err = a.Open("missing")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestMSZIP(t *testing.T) {
	t.Parallel()

	image := patterned(100000)
	tail := []byte("trailer")

	data := build(t, testCab{folders: []testFolder{
		mszip(file("firmware.bin", image), file("tail", tail)),
	}})

	assert.Less(t, len(data), len(image), "patterned data should compress")

	a, err := Parse(data)
	require.NoError(t, err)

	got, err := a.Open("firmware.bin")
	require.NoError(t, err)
	assert.Equal(t, image, got)

	got, err = a.Open("tail")
	require.NoError(t, err)
	assert.Equal(t, tail, got)
}

func TestReserveFields(t *testing.T) {
	t.Parallel()

	image := patterned(40000)

	data := build(t, testCab{
		folders:       []testFolder{mszip(file("firmware.bin", image))},
		headerReserve: 20,
		folderReserve: 3,
		blockReserve:  5,
	})

	a, err := Parse(data)
	require.NoError(t, err)

	got, err := a.Open("firmware.bin")
	require.NoError(t, err)
	assert.Equal(t, image, got)
}

func TestZeroChecksumIsNotVerified(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{
		folders:       []testFolder{stored(file("firmware.bin", []byte("image")))},
		zeroChecksums: true,
	})

	a, err := Parse(data)
	require.NoError(t, err)

	got, err := a.Open("firmware.bin")
	require.NoError(t, err)
	assert.Equal(t, []byte("image"), got)
}

func TestChecksumMismatch(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{folders: []testFolder{stored(file("firmware.bin", []byte("image")))}})

	// flip a payload byte of the only block
	data[firstBlock(data)+blockSize] ^= 0xff

	a, err := Parse(data)
	require.NoError(t, err)

	_, err = a.Open("firmware.bin")
	require.ErrorContains(t, err, "folder 0 block 0: checksum mismatch")
}

func TestCorruptMSZIP(t *testing.T) {
	t.Parallel()

	image := patterned(1000)

	t.Run("signature", func(t *testing.T) {
		t.Parallel()

		data := build(t, testCab{
			folders:       []testFolder{mszip(file("firmware.bin", image))},
			zeroChecksums: true,
		})

		data[firstBlock(data)+blockSize] = 'X'

		a, err := Parse(data)
		require.NoError(t, err)

		_, err = a.Open("firmware.bin")
		require.ErrorContains(t, err, "missing MSZIP signature")
	})

	t.Run("short stream", func(t *testing.T) {
		t.Parallel()

		data := build(t, testCab{
			folders:       []testFolder{mszip(file("firmware.bin", image))},
			zeroChecksums: true,
		})

		// declare more uncompressed bytes than the stream holds
		binary.LittleEndian.PutUint16(data[firstBlock(data)+6:], uint16(len(image))+1)

		a, err := Parse(data)
		require.NoError(t, err)

		_, err = a.Open("firmware.bin")
		require.ErrorContains(t, err, "inflate")
	})

	t.Run("long stream", func(t *testing.T) {
		t.Parallel()

		data := build(t, testCab{
			folders:       []testFolder{mszip(file("firmware.bin", image))},
			zeroChecksums: true,
		})

		binary.LittleEndian.PutUint16(data[firstBlock(data)+6:], uint16(len(image))-1)

		a, err := Parse(data)
		require.NoError(t, err)

		_, err = a.Open("firmware.bin")
		require.ErrorContains(t, err, "larger than the declared")
	})
}

func TestStoredSizeMismatch(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{
		folders:       []testFolder{stored(file("firmware.bin", []byte("image")))},
		zeroChecksums: true,
	})

	binary.LittleEndian.PutUint16(data[firstBlock(data)+6:], 3)

	a, err := Parse(data)
	require.NoError(t, err)

	_, err = a.Open("firmware.bin")
	require.ErrorContains(t, err, "stored block of 5 bytes declares 3 uncompressed")
}

func TestShortFolder(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{folders: []testFolder{stored(file("firmware.bin", patterned(70000)))}})

	// drop the last of the three blocks
	binary.LittleEndian.PutUint16(data[headerSize+4:], 2)

	a, err := Parse(data)
	require.NoError(t, err)

	_, err = a.Open("firmware.bin")
	require.ErrorContains(t, err, "65536 bytes of data for 70000 bytes of files")
}

func TestUnsupportedCompression(t *testing.T) {
	t.Parallel()

	const lzx = 0x1203 // LZX with a 18-bit window

	data := build(t, testCab{folders: []testFolder{{compression: lzx, files: []testFile{file("firmware.bin", []byte("image"))}}}})

	_, err := Parse(data)
	require.ErrorContains(t, err, "folder 0: unsupported compression type 3")
}

func TestMultiCabinetRejected(t *testing.T) {
	t.Parallel()

	for _, flag := range []uint16{flagPrevCabinet, flagNextCabinet} {
		data := build(t, testCab{
			folders: []testFolder{stored(file("firmware.bin", []byte("image")))},
			flags:   flag,
		})

		_, err := Parse(data)
		require.ErrorContains(t, err, "multi-cabinet sets are not supported")
	}

	data := build(t, testCab{folders: []testFolder{stored(testFile{name: "firmware.bin", data: []byte("image"), folderIndex: new(uint16(0xfffe))})}})

	_, err := Parse(data)
	require.ErrorContains(t, err, `file 0: "firmware.bin" spans cabinets`)
}

func TestBadFolderIndex(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{folders: []testFolder{stored(testFile{name: "firmware.bin", data: []byte("image"), folderIndex: new(uint16(1))})}})

	_, err := Parse(data)
	require.ErrorContains(t, err, `"firmware.bin" references folder 1 of 1`)
}

func TestFolderSizeLimit(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{folders: []testFolder{stored(testFile{name: "firmware.bin", data: []byte("image"), size: new(uint32(MaxFolderSize + 1))})}})

	_, err := Parse(data)
	require.ErrorContains(t, err, "limit is")
}

func TestDeclaredSizeBeyondData(t *testing.T) {
	t.Parallel()

	// the declared size is within the limit but nowhere near the data present:
	// it must fail on the data, not reserve the declared size up front
	data := build(t, testCab{folders: []testFolder{stored(testFile{name: "firmware.bin", data: []byte("image"), size: new(uint32(MaxFolderSize))})}})

	a, err := Parse(data)
	require.NoError(t, err)

	_, err = a.Open("firmware.bin")
	require.ErrorContains(t, err, "5 bytes of data for 1073741824 bytes of files")
}

func TestDataPastFilesIsNotDecoded(t *testing.T) {
	t.Parallel()

	image := patterned(70000)

	data := build(t, testCab{
		folders:       []testFolder{stored(testFile{name: "firmware.bin", data: image, size: new(uint32(10))})},
		zeroChecksums: true,
	})

	// the second block is invalid, but no file extends into it
	binary.LittleEndian.PutUint16(data[firstBlock(data)+blockSize+maxBlockSize+6:], 0)

	a, err := Parse(data)
	require.NoError(t, err)

	got, err := a.Open("firmware.bin")
	require.NoError(t, err)
	assert.Equal(t, image[:10], got)
}

func TestUnsupportedVersion(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{
		folders:      []testFolder{stored(file("firmware.bin", []byte("image")))},
		versionMinor: 2,
	})

	_, err := Parse(data)
	require.ErrorContains(t, err, "unsupported format version 1.2")
}

func TestNotCabinet(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte("MSCF"))
	require.ErrorContains(t, err, "truncated header")

	_, err = Parse(bytes.Repeat([]byte{'x'}, headerSize))
	require.ErrorContains(t, err, "not a cabinet file")
}

func TestTruncated(t *testing.T) {
	t.Parallel()

	data := build(t, testCab{
		folders: []testFolder{
			mszip(file("firmware.bin", patterned(40000))),
			stored(file("README.txt", []byte("readme"))),
		},
		headerReserve: 2,
		folderReserve: 1,
		blockReserve:  1,
	})

	// every proper prefix must fail cleanly: at Parse, or at Open when only
	// data blocks are missing
	for n := range data {
		a, err := Parse(data[:n])
		if err != nil {
			continue
		}

		_, err = a.Open("README.txt")
		require.Error(t, err, "prefix of %d bytes", n)
	}
}
