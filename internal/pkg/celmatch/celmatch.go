// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package celmatch provides CEL match expressions for volumes and disks used by the controllers.
//
// The expressions are built as an AST (no string formatting and parsing, so any value is safe to embed),
// and each distinct expression is built only once and then shared.
//
// Sharing the expression is important for the controllers which rebuild resources on each reconcile:
// COSI compares the old and the new resource specs with reflect.DeepEqual, which is cheap for the same
// AST pointer, but walks the whole AST otherwise. The shared AST also hits the compiled program cache
// (see cel.Expression.EvalBool).
package celmatch

import (
	"fmt"
	"sync"

	celgo "cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// metaPartitionSize is the size of the META partition.
const metaPartitionSize = 1048576

type kind int

const (
	kindVolumePartitionLabel kind = iota
	kindVolumePartitionLabelNonEmpty
	kindVolumePartitionIndex
	kindDiskDevPath
)

type cacheKey struct {
	str  string
	num  uint64
	kind kind
}

// cache holds parameterized expressions.
//
// The keys come from the machine configuration and the disks present in the system,
// so the number of entries stays small.
var cache sync.Map // map[cacheKey]cel.Expression

func cached(key cacheKey, env *celgo.Env, build func(b *builder) ast.Expr) cel.Expression {
	if expr, ok := cache.Load(key); ok {
		return expr.(cel.Expression) //nolint:forcetypeassert
	}

	expr, _ := cache.LoadOrStore(key, buildExpression(env, build))

	return expr.(cel.Expression) //nolint:forcetypeassert
}

func buildExpression(env *celgo.Env, build func(b *builder) ast.Expr) cel.Expression {
	b := &builder{Builder: cel.NewBuilder(env)}

	expr, err := b.ToBooleanExpression(build(b))
	if err != nil {
		// the expressions are static, so the error is a programming error
		panic(fmt.Sprintf("failed to build CEL expression: %s", err))
	}

	return *expr
}

func once(env func() *celgo.Env, build func(b *builder) ast.Expr) func() cel.Expression {
	return sync.OnceValue(func() cel.Expression {
		return buildExpression(env(), build)
	})
}

// builder adds a few helpers on top of cel.Builder.
type builder struct {
	*cel.Builder
}

// field returns `ident.field`.
func (b *builder) field(ident, field string) ast.Expr {
	return b.NewSelect(b.NextID(), b.NewIdent(b.NextID(), ident), field)
}

func (b *builder) str(s string) ast.Expr {
	return b.NewLiteral(b.NextID(), types.String(s))
}

func (b *builder) uint(n uint64) ast.Expr {
	return b.NewLiteral(b.NextID(), types.Uint(n))
}

func (b *builder) call(op string, args ...ast.Expr) ast.Expr {
	return b.NewCall(b.NextID(), op, args...)
}

func (b *builder) list(elems ...ast.Expr) ast.Expr {
	return b.NewList(b.NextID(), elems, nil)
}

// NoMatch returns an expression which never matches: `false`.
var NoMatch = once(celenv.Empty, func(b *builder) ast.Expr {
	return b.NewLiteral(b.NextID(), types.False)
})

// SystemDisk returns an expression which matches the system disk: `system_disk`.
var SystemDisk = once(celenv.DiskLocator, func(b *builder) ast.Expr {
	return b.NewIdent(b.NextID(), "system_disk")
})

// Meta returns an expression which matches the META partition:
//
//	volume.partition_label == "META" && volume.name in ["", "talosmeta"] && volume.size == 1048576u
var Meta = once(celenv.VolumeLocator, func(b *builder) ast.Expr {
	return b.call(operators.LogicalAnd,
		b.call(operators.LogicalAnd,
			b.call(operators.Equals, b.field("volume", "partition_label"), b.str(constants.MetaPartitionLabel)),
			b.call(operators.In, b.field("volume", "name"), b.list(b.str(""), b.str("talosmeta"))),
		),
		b.call(operators.Equals, b.field("volume", "size"), b.uint(metaPartitionSize)),
	)
})

// ImageCacheISO returns an expression which matches the image cache ISO:
//
//	volume.name in ["iso9660", "vfat"] && volume.label.startsWith("TALOS_")
var ImageCacheISO = once(celenv.VolumeLocator, func(b *builder) ast.Expr {
	return b.call(operators.LogicalAnd,
		b.call(operators.In, b.field("volume", "name"), b.list(b.str("iso9660"), b.str("vfat"))),
		b.NewMemberCall(b.NextID(), "startsWith", b.field("volume", "label"), b.str("TALOS_")),
	)
})

// VolumePartitionLabel returns an expression which matches a volume by the partition label:
//
//	volume.partition_label == "<label>"
func VolumePartitionLabel(label string) cel.Expression {
	return cached(cacheKey{kind: kindVolumePartitionLabel, str: label}, celenv.VolumeLocator(), func(b *builder) ast.Expr {
		return b.call(operators.Equals, b.field("volume", "partition_label"), b.str(label))
	})
}

// VolumePartitionLabelNonEmpty returns an expression which matches a volume with a filesystem (or other recognized content) by the partition label:
//
//	volume.partition_label == "<label>" && volume.name != ""
func VolumePartitionLabelNonEmpty(label string) cel.Expression {
	return cached(cacheKey{kind: kindVolumePartitionLabelNonEmpty, str: label}, celenv.VolumeLocator(), func(b *builder) ast.Expr {
		return b.call(operators.LogicalAnd,
			b.call(operators.Equals, b.field("volume", "partition_label"), b.str(label)),
			b.call(operators.NotEquals, b.field("volume", "name"), b.str("")),
		)
	})
}

// VolumePartitionIndex returns an expression which matches a partition by the parent device path and the partition index:
//
//	volume.parent_dev_path == "<parentDevPath>" && volume.partition_index == <index>u
func VolumePartitionIndex(parentDevPath string, index uint64) cel.Expression {
	return cached(cacheKey{kind: kindVolumePartitionIndex, str: parentDevPath, num: index}, celenv.VolumeLocator(), func(b *builder) ast.Expr {
		return b.call(operators.LogicalAnd,
			b.call(operators.Equals, b.field("volume", "parent_dev_path"), b.str(parentDevPath)),
			b.call(operators.Equals, b.field("volume", "partition_index"), b.uint(index)),
		)
	})
}

// DiskDevPath returns an expression which matches a disk by the device path:
//
//	disk.dev_path == "<devPath>"
func DiskDevPath(devPath string) cel.Expression {
	return cached(cacheKey{kind: kindDiskDevPath, str: devPath}, celenv.DiskLocator(), func(b *builder) ast.Expr {
		return b.call(operators.Equals, b.field("disk", "dev_path"), b.str(devPath))
	})
}
