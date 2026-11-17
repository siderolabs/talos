// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
	"github.com/siderolabs/talos/internal/pkg/firmware/jcat/jcattest"
	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

// buildCab encodes files as a single-folder, uncompressed cabinet.
func buildCab(files map[string][]byte) []byte {
	const (
		headerSize = 36
		folderSize = 8
		blockSize  = 32768
	)

	names := slices.Sorted(maps.Keys(files))

	var entries, contents bytes.Buffer

	for _, name := range names {
		entry := make([]byte, 16)
		binary.LittleEndian.PutUint32(entry[0:], uint32(len(files[name])))
		binary.LittleEndian.PutUint32(entry[4:], uint32(contents.Len()))
		entries.Write(entry)
		entries.WriteString(name)
		entries.WriteByte(0)

		contents.Write(files[name])
	}

	var blocks bytes.Buffer

	for start := 0; start < contents.Len(); start += blockSize {
		chunk := contents.Bytes()[start:min(start+blockSize, contents.Len())]

		header := make([]byte, 8) // checksum left zero: not verified
		binary.LittleEndian.PutUint16(header[4:], uint16(len(chunk)))
		binary.LittleEndian.PutUint16(header[6:], uint16(len(chunk)))
		blocks.Write(header)
		blocks.Write(chunk)
	}

	dataOffset := headerSize + folderSize + entries.Len()

	var out bytes.Buffer

	header := make([]byte, headerSize)
	copy(header, "MSCF")
	binary.LittleEndian.PutUint32(header[8:], uint32(dataOffset+blocks.Len()))
	binary.LittleEndian.PutUint32(header[16:], headerSize+folderSize)
	header[24], header[25] = 3, 1
	binary.LittleEndian.PutUint16(header[26:], 1)
	binary.LittleEndian.PutUint16(header[28:], uint16(len(names)))
	out.Write(header)

	folder := make([]byte, folderSize)
	binary.LittleEndian.PutUint32(folder[0:], uint32(dataOffset))
	binary.LittleEndian.PutUint16(folder[4:], uint16((contents.Len()+blockSize-1)/blockSize))
	out.Write(folder)

	out.Write(entries.Bytes())
	out.Write(blocks.Bytes())

	return out.Bytes()
}

func metainfo(id, version, imageName string) []byte {
	checksum := ""
	if imageName != "" {
		checksum = fmt.Sprintf(`<checksum type="sha256" filename=%q target="content">unused</checksum>`, imageName)
	}

	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<component type="firmware"><id>%s</id><name>Test</name>
<provides><firmware type="flashed">%s</firmware></provides>
<releases><release version=%q timestamp="1">%s</release></releases>
</component>`, id, testGUID, version, checksum)
}

// catalogRelease is a release as the catalog describes it: the checksum of
// the image and, unless empty, its file name in the container.
func catalogRelease(version, imageName string, image []byte) *lvfs.Release {
	sum := sha256.Sum256(image)

	return &lvfs.Release{
		Version:   version,
		Checksums: []lvfs.Checksum{{Type: "sha256", Target: "content", Filename: imageName, Value: hex.EncodeToString(sum[:])}},
	}
}

func TestExtractPayload(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	image := bytes.Repeat([]byte("firmware"), 10000)
	info := metainfo("com.example.nvme", "2", "fw.bin")

	container := buildCab(map[string][]byte{
		"README.txt":            []byte("readme"),
		"firmware.metainfo.xml": info,
		"fw.bin":                image,
		"firmware.jcat": jcattest.Build(t,
			pki.Item(t, "fw.bin", nil, image),
			pki.Item(t, "firmware.metainfo.xml", nil, info),
		),
	})

	got, err := lvfs.ExtractPayload(container, "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.NoError(t, err)
	assert.Equal(t, image, got)

	_, err = lvfs.ExtractPayload(container, "com.example.other", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, `component "com.example.other" not found in container`)

	_, err = lvfs.ExtractPayload(container, "com.example.nvme", catalogRelease("2", "missing.bin", image), pki.Roots)
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = lvfs.ExtractPayload(container, "com.example.nvme", catalogRelease("2", "fw.bin", []byte("other")), pki.Roots)
	require.ErrorContains(t, err, "image fw.bin: SHA-256 mismatch")

	_, err = lvfs.ExtractPayload(container, "com.example.nvme", &lvfs.Release{Version: "2"}, pki.Roots)
	require.ErrorContains(t, err, "catalog release has no content checksum")

	_, err = lvfs.ExtractPayload(container, "com.example.nvme", catalogRelease("2", "fw.bin", image), jcattest.NewPKI(t).Roots)
	require.ErrorContains(t, err, "fw.bin signature: no valid PKCS7 signature")

	_, err = lvfs.ExtractPayload([]byte("not a cabinet"), "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, "error parsing container")
}

func TestExtractPayloadMultipleComponents(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	// one container, two components with their own images, as vendors ship
	// several capacities of a drive
	small, large := []byte("small image"), []byte("large image")
	smallInfo, largeInfo := metainfo("com.example.small", "7", "256GB.bin"), metainfo("com.example.large", "7", "512GB.bin")

	container := buildCab(map[string][]byte{
		"256GB.metainfo.xml": smallInfo,
		"256GB.bin":          small,
		"512GB.metainfo.xml": largeInfo,
		"512GB.bin":          large,
		"firmware.jcat": jcattest.Build(t,
			pki.Item(t, "256GB.bin", nil, small),
			pki.Item(t, "256GB.metainfo.xml", nil, smallInfo),
			pki.Item(t, "512GB.bin", nil, large),
			pki.Item(t, "512GB.metainfo.xml", nil, largeInfo),
		),
	})

	got, err := lvfs.ExtractPayload(container, "com.example.large", catalogRelease("7", "512GB.bin", large), pki.Roots)
	require.NoError(t, err)
	assert.Equal(t, large, got)

	got, err = lvfs.ExtractPayload(container, "com.example.small", catalogRelease("7", "256GB.bin", small), pki.Roots)
	require.NoError(t, err)
	assert.Equal(t, small, got)
}

// TestExtractPayloadImageName covers where the image's file name comes from:
// the catalog release, else the container's metainfo, else the default.
func TestExtractPayloadImageName(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	image := []byte("image")

	for _, tc := range []struct {
		name         string
		catalogName  string
		metainfoName string
		actualName   string
	}{
		{"catalog", "fw.bin", "other.bin", "fw.bin"},
		{"metainfo", "", "fw.bin", "fw.bin"},
		{"default", "", "", "firmware.bin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			info := metainfo("com.example.nvme", "2", tc.metainfoName)

			container := buildCab(map[string][]byte{
				"firmware.metainfo.xml": info,
				tc.actualName:           image,
				"firmware.jcat": jcattest.Build(t,
					pki.Item(t, tc.actualName, nil, image),
					pki.Item(t, "firmware.metainfo.xml", nil, info),
				),
			})

			got, err := lvfs.ExtractPayload(container, "com.example.nvme", catalogRelease("2", tc.catalogName, image), pki.Roots)
			require.NoError(t, err)
			assert.Equal(t, image, got)
		})
	}
}

func TestExtractPayloadUnsigned(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	image := []byte("image")
	info := metainfo("com.example.nvme", "2", "fw.bin")

	files := map[string][]byte{
		"firmware.metainfo.xml": info,
		"fw.bin":                image,
	}

	_, err := lvfs.ExtractPayload(buildCab(files), "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, "error reading signature")

	// the image is signed, the metainfo isn't
	files["firmware.jcat"] = jcattest.Build(t, pki.Item(t, "fw.bin", nil, image))

	_, err = lvfs.ExtractPayload(buildCab(files), "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, "firmware.metainfo.xml is not signed")

	// the image is signed, the metainfo's signature is for other content
	files["firmware.jcat"] = jcattest.Build(t, pki.Item(t, "fw.bin", nil, image), pki.Item(t, "firmware.metainfo.xml", nil, []byte("other")))

	_, err = lvfs.ExtractPayload(buildCab(files), "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, "firmware.metainfo.xml signature: SHA-256 checksum mismatch")

	// the image's signature carries no PKCS7 blob
	files["firmware.jcat"] = jcattest.Build(t, jcat.Item{ID: "fw.bin"}, pki.Item(t, "firmware.metainfo.xml", nil, info))

	_, err = lvfs.ExtractPayload(buildCab(files), "com.example.nvme", catalogRelease("2", "fw.bin", image), pki.Roots)
	require.ErrorContains(t, err, "fw.bin signature: ")
	require.ErrorContains(t, err, "no PKCS7 signatures found")
}

func TestContentChecksum(t *testing.T) {
	t.Parallel()

	release := &lvfs.Release{Checksums: []lvfs.Checksum{
		{Type: "sha1", Target: "content", Filename: "fw.bin", Value: "aa"},
		{Type: "sha256", Target: "container", Value: "bb"},
		{Type: "SHA256", Target: "content", Filename: "fw.bin", Value: " cc "},
	}}

	name, digest := release.ContentChecksum()
	assert.Equal(t, "fw.bin", name)
	assert.Equal(t, "cc", digest)

	name, digest = (&lvfs.Release{}).ContentChecksum()
	assert.Empty(t, name)
	assert.Empty(t, digest)
}
