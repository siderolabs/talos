// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cgroups provides functions to parse cgroup information.
package cgroups

import (
	"io"
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
)

// Tree represents a cgroup tree.
type Tree struct {
	Root *Node
}

// Find the node by directory path.
func (t *Tree) Find(directoryPath string) *Node {
	node := t.Root

	for _, component := range strings.Split(directoryPath, "/") {
		if component == "." || component == "" {
			return node
		}

		if node.Children == nil {
			node.Children = make(map[string]*Node)
		}

		child, ok := node.Children[component]
		if !ok {
			child = &Node{}
			node.Children[component] = child
		}

		node = child
	}

	return node
}

// ResolveNames resolves the names of the node and its children.
func (t *Tree) ResolveNames(nameMap map[string]string) {
	t.Root.ResolveNames(nameMap)
}

// Walk the tree.
func (t *Tree) Walk(fn func(*Node)) {
	t.Root.Walk(fn)
}

// SortedChildren returns the sorted children of the node.
func (n *Node) SortedChildren() []string {
	children := maps.Keys(n.Children)

	slices.Sort(children)

	return children
}

// ResolveNames resolves the names of the node and its children.
func (n *Node) ResolveNames(nameMap map[string]string) {
	for name, child := range n.Children {
		if resolvedName, ok := nameMap[name]; ok {
			delete(n.Children, name)
			n.Children[resolvedName] = child
		}

		child.ResolveNames(nameMap)
	}
}

// Walk the node.
func (n *Node) Walk(fn func(*Node)) {
	fn(n)

	for _, child := range n.Children {
		child.Walk(fn)
	}
}

// Node represents a cgroup node.
type Node struct {
	Children map[string]*Node

	CgroupEvents  FlatMap
	CgroupFreeze  Value
	CgroupProcs   Values
	CgroupStat    FlatMap
	CgroupThreads Values

	// Resolved externally into process names.
	CgroupProcsResolved []RawValue

	CPUIdle       Value
	CPUMax        Values
	CPUMaxBurst   Value
	CPUPressure   NestedKeyed
	CPUStat       FlatMap
	CPUStatLocal  FlatMap
	CPUWeight     Value
	CPUWeightNice Value

	CPUSetCPUs          RawValue
	CPUSetCPUsEffective RawValue
	CPUSetMems          RawValue
	CPUSetMemsEffective RawValue

	IOBFQWeight FlatMap
	IOMax       NestedKeyed
	IOPressure  NestedKeyed
	IOStat      NestedKeyed

	MemoryCurrent     Value
	MemoryEvents      FlatMap
	MemoryEventsLocal FlatMap
	MemoryHigh        Value
	MemoryLow         Value
	MemoryMax         Value
	MemoryMin         Value
	MemoryNUMAStat    NestedKeyed
	MemoryOOMGroup    Value
	MemoryPeak        Value
	MemoryPressure    NestedKeyed
	MemoryStat        FlatMap

	MemorySwapCurrent Value
	MemorySwapEvents  FlatMap
	MemorySwapHigh    Value
	MemorySwapMax     Value
	MemorySwapPeak    Value

	PidsCurrent Value
	PidsEvents  FlatMap
	PidsMax     Value
	PidsPeak    Value
}

func parseSingleValue(parser func(r io.Reader) (Values, error), out *Value, r io.Reader) error {
	values, err := parser(r)
	if err != nil {
		return err
	}

	if len(values) > 0 {
		*out = values[0]
	}

	return nil
}

// Parse the cgroup information by filename from the reader.
//
//nolint:gocyclo,cyclop
func (n *Node) Parse(filename string, r io.Reader) error {
	var err error

	switch filename {
	case "cgroup.events":
		n.CgroupEvents, err = ParseFlatMapValues(r)

		return err
	case "cgroup.freeze":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.CgroupFreeze, r)
	case "cgroup.procs":
		n.CgroupProcs, err = ParseNewlineSeparatedValues(r)

		return err
	case "cgroup.stat":
		n.CgroupStat, err = ParseFlatMapValues(r)

		return err
	case "cgroup.threads":
		n.CgroupThreads, err = ParseNewlineSeparatedValues(r)

		return err
	case "cpu.idle":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.CPUIdle, r)
	case "cpu.max":
		n.CPUMax, err = ParseSpaceSeparatedValues(r)

		return err
	case "cpu.max.burst":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.CPUMaxBurst, r)
	case "cpu.pressure":
		n.CPUPressure, err = ParseNestedKeyedValues(r)

		return err
	case "cpu.stat":
		n.CPUStat, err = ParseFlatMapValues(r)

		return err
	case "cpu.stat.local":
		n.CPUStatLocal, err = ParseFlatMapValues(r)

		return err
	case "cpu.weight":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.CPUWeight, r)
	case "cpu.weight.nice":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.CPUWeightNice, r)
	case "cpuset.cpus":
		n.CPUSetCPUs, err = ParseRawValue(r)

		return err
	case "cpuset.cpus.effective":
		n.CPUSetCPUsEffective, err = ParseRawValue(r)

		return err
	case "cpuset.mems":
		n.CPUSetMems, err = ParseRawValue(r)

		return err
	case "cpuset.mems.effective":
		n.CPUSetMemsEffective, err = ParseRawValue(r)

		return err
	case "io.bfq.weight":
		n.IOBFQWeight, err = ParseFlatMapValues(r)

		return err
	case "io.max":
		n.IOMax, err = ParseNestedKeyedValues(r)

		return err
	case "io.pressure":
		n.IOPressure, err = ParseNestedKeyedValues(r)

		return err
	case "io.stat":
		n.IOStat, err = ParseNestedKeyedValues(r)

		return err
	case "memory.current":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryCurrent, r)
	case "memory.events":
		n.MemoryEvents, err = ParseFlatMapValues(r)

		return err
	case "memory.events.local":
		n.MemoryEventsLocal, err = ParseFlatMapValues(r)

		return err
	case "memory.high":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryHigh, r)
	case "memory.low":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryLow, r)
	case "memory.max":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryMax, r)
	case "memory.min":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryMin, r)
	case "memory.numa_stat":
		n.MemoryNUMAStat, err = ParseNestedKeyedValues(r)

		return err
	case "memory.oom.group":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryOOMGroup, r)
	case "memory.peak":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemoryPeak, r)
	case "memory.pressure":
		n.MemoryPressure, err = ParseNestedKeyedValues(r)

		return err
	case "memory.stat":
		n.MemoryStat, err = ParseFlatMapValues(r)

		return err
	case "memory.swap.current":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemorySwapCurrent, r)
	case "memory.swap.events":
		n.MemorySwapEvents, err = ParseFlatMapValues(r)

		return err
	case "memory.swap.high":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemorySwapHigh, r)
	case "memory.swap.max":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemorySwapMax, r)
	case "memory.swap.peak":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.MemorySwapPeak, r)
	case "pids.current":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.PidsCurrent, r)
	case "pids.events":
		n.PidsEvents, err = ParseFlatMapValues(r)

		return err
	case "pids.max":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.PidsMax, r)
	case "pids.peak":
		return parseSingleValue(ParseNewlineSeparatedValues, &n.PidsPeak, r)
	}

	return nil
}
