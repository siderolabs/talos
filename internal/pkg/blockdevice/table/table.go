/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package table provides a library for working with block device partition tables.
package table

import "github.com/talos-systems/talos/internal/pkg/serde"

// Table represents a partition table.
type Table = []byte

// PartitionTable describes a partition table.
type PartitionTable interface {
	// Bytes returns the partition table as a byte slice.
	Bytes() Table
	// Read reades the partition table.
	Read() error
	// Write writes the partition table/.
	Write() error
	// Type returns the partition table type.
	Type() Type
	// Header returns the partition table header.
	Header() Header
	// Partitions returns a slice o partition table partitions.
	Partitions() []Partition
	// Repair repairs a partition table.
	Repair() error
	// New creates a new partition table.
	New() (PartitionTable, error)
	// Partitioner must be implemented by a partition table.
	Partitioner
}

// Type represents a partition table type.
type Type int

const (
	// MBR is the Master Boot Record artition table.
	MBR Type = iota
	// GPT is the GUID partition table.
	GPT
)

// Header describes a partition table header.
type Header interface {
	// Bytes returns the partition table header as a byte slice.
	Bytes() []byte
	serde.Serde
}

// Partition describes a partition.
type Partition interface {
	// Bytes returns the partition table partitions as a byte slice.
	Bytes() []byte
	// Start returns the partition's starting LBA.
	Start() int64
	// Length returns the partition's length in LBA.
	Length() int64
	// No returns the partition's number.
	No() int32
	serde.Serde
}

// Partitioner describes actions that can be taken on a partition.
type Partitioner interface {
	// Add adds a partition to the partition table.
	Add(uint64, ...interface{}) (Partition, error)
	// Resize resizes a partition table.
	Resize(Partition) error
	// Delete deletes a partition table.
	Delete(Partition) error
}
