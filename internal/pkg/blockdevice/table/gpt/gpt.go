/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package gpt provides a library for working with GPT partitions.
package gpt

import (
	"encoding/binary"
	"os"
	"syscall"
	"unsafe"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/lba"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/table"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/table/gpt/header"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/internal/pkg/serde"
	"golang.org/x/sys/unix"
)

// GPT represents the GUID partition table.
type GPT struct {
	table      table.Table
	header     *header.Header
	partitions []table.Partition
	lba        *lba.LogicalBlockAddresser

	devname string
	f       *os.File
}

// NewGPT initializes and returns a GUID partition table.
func NewGPT(devname string, f *os.File, setters ...interface{}) *GPT {
	opts := NewDefaultOptions(setters...)

	lba := &lba.LogicalBlockAddresser{
		PhysicalBlockSize: opts.PhysicalBlockSize,
		LogicalBlockSize:  opts.LogicalBlockSize,
	}

	return &GPT{
		lba:     lba,
		devname: devname,
		f:       f,
	}
}

// Bytes returns the partition table as a byte slice.
func (gpt *GPT) Bytes() []byte {
	return gpt.table
}

// Type returns the partition type.
func (gpt *GPT) Type() table.Type {
	return table.GPT
}

// Header returns the header.
func (gpt *GPT) Header() table.Header {
	return gpt.header
}

// Partitions returns the partitions.
func (gpt *GPT) Partitions() []table.Partition {
	return gpt.partitions
}

// Read performs reads the partition table.
func (gpt *GPT) Read() error {
	primaryTable, err := gpt.readPrimary()
	if err != nil {
		return err
	}

	serializedHeader, err := gpt.serializeHeader(primaryTable)
	if err != nil {
		return err
	}

	serializedPartitions, err := gpt.serializePartitions(serializedHeader)
	if err != nil {
		return err
	}

	gpt.table = primaryTable
	gpt.header = serializedHeader
	gpt.partitions = serializedPartitions

	return nil
}

// Write writes the partition table to disk.
func (gpt *GPT) Write() error {
	partitions, err := gpt.deserializePartitions()
	if err != nil {
		return err
	}

	if err := gpt.writePrimary(partitions); err != nil {
		return errors.Errorf("failed to write primary table: %v", err)
	}

	if err := gpt.writeSecondary(partitions); err != nil {
		return errors.Errorf("failed to write secondary table: %v", err)
	}

	return gpt.Read()
}

// New creates a new partition table and writes it to disk.
func (gpt *GPT) New() (table.PartitionTable, error) {
	// Seek to the end to get the size.
	size, err := gpt.f.Seek(0, 2)
	if err != nil {
		return nil, err
	}
	// Reset and seek to the beginning.
	_, err = gpt.f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	h, err := gpt.newHeader(size)
	if err != nil {
		return nil, err
	}
	pmbr := gpt.newPMBR(h)

	gpt.header = h
	gpt.partitions = []table.Partition{}

	written, err := gpt.f.WriteAt(pmbr[446:], 446)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write the protective MBR")
	}
	if written != len(pmbr[446:]) {
		return nil, errors.Errorf("expected a write %d bytes, got %d", written, len(pmbr[446:]))
	}

	// Reset and seek to the beginning.
	_, err = gpt.f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return gpt, nil
}

func (gpt *GPT) newHeader(size int64) (*header.Header, error) {
	h := &header.Header{}
	h.Signature = "EFI PART"
	h.Revision = binary.LittleEndian.Uint32([]byte{0x00, 0x00, 0x01, 0x00})
	h.Size = header.HeaderSize
	h.Reserved = binary.LittleEndian.Uint32([]byte{0x00, 0x00, 0x00, 0x00})
	h.CurrentLBA = 1
	h.BackupLBA = uint64(size/int64(gpt.lba.PhysicalBlockSize) - 1)
	h.FirstUsableLBA = 34
	h.LastUsableLBA = h.BackupLBA - 33
	guuid, err := uuid.NewUUID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate UUID for new partition table")
	}
	h.GUUID = guuid
	h.PartitionEntriesStartLBA = 2
	h.NumberOfPartitionEntries = 128
	h.PartitionEntrySize = 128

	return h, nil
}

// See:
// - https://en.wikipedia.org/wiki/GUID_Partition_Table#Protective_MBR_(LBA_0)
// - https://www.syslinux.org/wiki/index.php?title=Doc/gpt
// - https://en.wikipedia.org/wiki/Master_boot_record
func (gpt *GPT) newPMBR(h *header.Header) []byte {
	pmbr := make([]byte, 512)

	// Boot signature.
	copy(pmbr[510:], []byte{0x55, 0xaa})
	// PMBR protective entry.
	b := pmbr[446 : 446+16]
	b[0] = 0x00
	// Partition type: EFI data partition.
	b[4] = 0xee
	// Partition start LBA.
	binary.LittleEndian.PutUint32(b[8:12], 1)
	// Partition length in sectors.
	binary.LittleEndian.PutUint32(b[12:16], uint32(h.BackupLBA))

	return pmbr
}

// Write the primary table.
func (gpt *GPT) writePrimary(partitions []byte) error {
	header, err := gpt.deserializeHeader(partitions)
	if err != nil {
		return err
	}

	table, err := gpt.newTable(header, partitions, lba.Range{Start: 0, End: 1}, lba.Range{Start: 1, End: 33})
	if err != nil {
		return err
	}

	written, err := gpt.f.WriteAt(table, int64(gpt.PhysicalBlockSize()))
	if err != nil {
		return err
	}

	if written != len(table) {
		return errors.Errorf("expected a primary table write of %d bytes, got %d", len(table), written)
	}

	return nil
}

// Write the secondary table.
func (gpt *GPT) writeSecondary(partitions []byte) error {
	header, err := gpt.deserializeHeader(partitions, header.WithHeaderPrimary(false))
	if err != nil {
		return err
	}

	table, err := gpt.newTable(header, partitions, lba.Range{Start: 32, End: 33}, lba.Range{Start: 0, End: 32})
	if err != nil {
		return err
	}

	offset := int64((gpt.header.LastUsableLBA + 1))
	written, err := gpt.f.WriteAt(table, offset*int64(gpt.PhysicalBlockSize()))
	if err != nil {
		return err
	}

	if written != len(table) {
		return errors.Errorf("expected a secondary table write of %d bytes, got %d", len(table), written)
	}

	return nil
}

// Repair repairs the partition table.
func (gpt *GPT) Repair() error {
	// Seek to the end to get the size.
	size, err := gpt.f.Seek(0, 2)
	if err != nil {
		return err
	}
	// Reset and seek to the beginning.
	_, err = gpt.f.Seek(0, 0)
	if err != nil {
		return err
	}

	gpt.header.BackupLBA = uint64(size/int64(gpt.lba.PhysicalBlockSize) - 1)
	gpt.header.LastUsableLBA = gpt.header.BackupLBA - 33

	return nil
}

// Add adds a partition.
func (gpt *GPT) Add(size uint64, setters ...interface{}) (table.Partition, error) {
	opts := partition.NewDefaultOptions(setters...)

	var start, end uint64
	if len(gpt.partitions) == 0 {
		start = gpt.header.FirstUsableLBA
	} else {
		previous := gpt.partitions[len(gpt.partitions)-1]
		start = previous.(*partition.Partition).LastLBA + 1
	}
	end = start + size/uint64(gpt.PhysicalBlockSize())

	if end > gpt.header.LastUsableLBA {
		// TODO(andrewrynhard): This calculation is wrong, fix it.
		available := (gpt.header.LastUsableLBA - start) * uint64(gpt.PhysicalBlockSize())
		return nil, errors.Errorf("requested partition size %d is too big, largest available is %d", size, available)
	}

	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	partition := &partition.Partition{
		IsNew:    !opts.Test,
		Type:     opts.Type,
		ID:       uuid,
		FirstLBA: start,
		LastLBA:  end,
		Flags:    opts.Flags,
		Name:     opts.Name,
		Number:   int32(len(gpt.partitions) + 1),
	}

	gpt.partitions = append(gpt.partitions, partition)

	return partition, nil
}

// Resize resizes a partition.
// TODO(andrewrynhard): Verify that we can indeed grow this partition safely.
func (gpt *GPT) Resize(p table.Partition) error {
	partition, ok := p.(*partition.Partition)
	if !ok {
		return errors.Errorf("partition is not a GUID partition table partition")
	}

	// TODO(andrewrynhard): This should be a parameter.
	partition.LastLBA = gpt.header.LastUsableLBA

	index := partition.Number - 1
	if len(gpt.partitions) < int(index) {
		return errors.Errorf("unknown partition %d, only %d available", partition.Number, len(gpt.partitions))
	}

	partition.IsResized = true

	gpt.partitions[index] = partition

	return nil
}

// Delete deletes a partition.
func (gpt *GPT) Delete(partition table.Partition) error {
	return nil
}

// PhysicalBlockSize returns the physical block size.
func (gpt *GPT) PhysicalBlockSize() int {
	return gpt.lba.PhysicalBlockSize
}

func (gpt *GPT) readPrimary() ([]byte, error) {
	// LBA 34 is the first usable sector on the disk.
	table := gpt.lba.Make(34)
	read, err := gpt.f.ReadAt(table, 0)
	if err != nil {
		return nil, err
	}

	if read != len(table) {
		return nil, errors.Errorf("expected a read of %d bytes, got %d", len(table), read)
	}

	return table, nil
}

func (gpt *GPT) newTable(header, partitions []byte, headerRange, paritionsRange lba.Range) ([]byte, error) {
	table := gpt.lba.Make(33)

	if _, err := gpt.lba.Copy(table, header, headerRange); err != nil {
		return nil, errors.Errorf("failed to copy header data: %v", err)
	}

	if _, err := gpt.lba.Copy(table, partitions, paritionsRange); err != nil {
		return nil, errors.Errorf("failed to copy partition data: %v", err)
	}

	return table, nil
}

func (gpt *GPT) serializeHeader(table []byte) (*header.Header, error) {
	// GPT header is in LBA 1.
	data, err := gpt.lba.From(table, lba.Range{Start: 1, End: 1})
	if err != nil {
		return nil, err
	}

	hdr := header.NewHeader(data, gpt.lba)

	opts := header.NewDefaultOptions(header.WithHeaderTable(table))
	if err := serde.Ser(hdr, hdr.Bytes(), 0, opts); err != nil {
		return nil, errors.Errorf("failed to serialize the header: %v", err)
	}

	return hdr, nil
}

func (gpt *GPT) deserializeHeader(partitions []byte, setters ...interface{}) ([]byte, error) {
	data := gpt.lba.Make(1)
	setters = append(setters, header.WithHeaderArrayBytes(partitions))
	opts := header.NewDefaultOptions(setters...)
	if err := serde.De(gpt.header, data, 0, opts); err != nil {
		return nil, errors.Errorf("failed to deserialize the header: %v", err)
	}

	return data, nil
}

func (gpt *GPT) serializePartitions(header *header.Header) ([]table.Partition, error) {
	partitions := make([]table.Partition, 0, header.NumberOfPartitionEntries)

	for i := uint32(0); i < header.NumberOfPartitionEntries; i++ {
		offset := i * header.PartitionEntrySize
		data := header.ArrayBytes()[offset : offset+header.PartitionEntrySize]
		prt := partition.NewPartition(data)

		if err := serde.Ser(prt, header.ArrayBytes(), offset, nil); err != nil {
			return nil, errors.Errorf("failed to serialize the partitions: %v", err)
		}

		// The first LBA of the partition cannot start before the first usable
		// LBA specified in the header.
		if prt.FirstLBA >= header.FirstUsableLBA {
			prt.Number = int32(i) + 1
			partitions = append(partitions, prt)
		}
	}

	return partitions, nil
}

func (gpt *GPT) deserializePartitions() ([]byte, error) {
	// TODO(andrewrynhard): Should this be a method on the Header struct?
	data := make([]byte, gpt.header.NumberOfPartitionEntries*gpt.header.PartitionEntrySize)

	for j, p := range gpt.partitions {
		i := uint32(j)
		partition, ok := p.(*partition.Partition)
		if !ok {
			return nil, errors.Errorf("partition is not a GUID partition table partition")
		}
		if err := serde.De(partition, data, i*gpt.header.PartitionEntrySize, nil); err != nil {
			return nil, errors.Errorf("failed to deserialize the partitions: %v", err)
		}
	}

	return data, nil
}

// InformKernelOfAdd invokes the BLKPG_ADD_PARTITION ioctl.
func (gpt *GPT) InformKernelOfAdd(partition table.Partition) error {
	return inform(gpt.f.Fd(), partition, unix.BLKPG_ADD_PARTITION, int64(gpt.lba.PhysicalBlockSize))
}

// InformKernelOfResize invokes the BLKPG_RESIZE_PARTITION ioctl.
func (gpt *GPT) InformKernelOfResize(partition table.Partition) error {
	return inform(gpt.f.Fd(), partition, unix.BLKPG_RESIZE_PARTITION, int64(gpt.lba.PhysicalBlockSize))
}

// InformKernelOfDelete invokes the BLKPG_DEL_PARTITION ioctl.
func (gpt *GPT) InformKernelOfDelete(partition table.Partition) error {
	return inform(gpt.f.Fd(), partition, unix.BLKPG_DEL_PARTITION, int64(gpt.lba.PhysicalBlockSize))
}

func inform(fd uintptr, partition table.Partition, op int32, blocksize int64) error {
	arg := &unix.BlkpgIoctlArg{
		Op: op,
		Data: (*byte)(unsafe.Pointer(&unix.BlkpgPartition{
			Start:  partition.Start() * blocksize,
			Length: partition.Length() * blocksize,
			Pno:    partition.No(),
		})),
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		unix.BLKPG,
		uintptr(unsafe.Pointer(arg)),
	)

	if errno != 0 {
		return errno
	}

	return nil
}
