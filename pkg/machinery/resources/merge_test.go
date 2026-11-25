// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resources_test

import (
	"net/netip"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestMergeCoversAllFields verifies that every Merge method carries over every field of its spec,
// so that adding a field to a spec without handling it in Merge fails this test.
func TestMergeCoversAllFields(t *testing.T) {
	t.Parallel()

	t.Run("AffiliateSpec", func(t *testing.T) {
		t.Parallel()

		testMergeCoversAllFields(t, func(dst, src *cluster.AffiliateSpec) { dst.Merge(src) },
			"NodeID", // merge key
		)
	})

	t.Run("EthernetSpecSpec", func(t *testing.T) {
		t.Parallel()

		testMergeCoversAllFields(t, func(dst, src *network.EthernetSpecSpec) { dst.Merge(src) },
			"LinkName",     // merge key
			"FromSelector", // describes the source spec, only used to order the merge
		)
	})

	t.Run("LinkSpecSpec", func(t *testing.T) {
		t.Parallel()

		testMergeCoversAllFields(t, func(dst, src *network.LinkSpecSpec) { require.NoError(t, dst.Merge(src)) },
			"Name", // merge key
		)
	})

	t.Run("WireguardSpec", func(t *testing.T) {
		t.Parallel()

		testMergeCoversAllFields(t, func(dst, src *network.WireguardSpec) { dst.Merge(*src) },
			"PublicKey", // only set on LinkStatus, never on a spec
		)
	})
}

// testMergeCoversAllFields merges a spec with every field set into a zero spec and checks that nothing is lost.
//
// Fields listed in skip are intentionally not merged, so they are left out of the comparison.
func testMergeCoversAllFields[T any](t *testing.T, merge func(dst, src *T), skip ...string) {
	var src, dst T

	fillNonZero(t, reflect.ValueOf(&src).Elem())

	merge(&dst, &src)

	dstValue := reflect.ValueOf(dst)

	for field, srcField := range reflect.ValueOf(src).Fields() {
		if slices.Contains(skip, field.Name) {
			continue
		}

		assert.Equal(t, srcField.Interface(), dstValue.FieldByIndex(field.Index).Interface(), "field %s is not merged", field.Name)
	}
}

// fillNonZero sets every value reachable from v to a non-zero value.
//
//nolint:gocyclo
func fillNonZero(t *testing.T, v reflect.Value) {
	t.Helper()

	switch v.Type() {
	case reflect.TypeFor[netip.Addr]():
		v.Set(reflect.ValueOf(netip.MustParseAddr("10.0.0.1")))

		return
	case reflect.TypeFor[netip.Prefix]():
		v.Set(reflect.ValueOf(netip.MustParsePrefix("10.0.0.0/8")))

		return
	case reflect.TypeFor[netip.AddrPort]():
		v.Set(reflect.ValueOf(netip.MustParseAddrPort("10.0.0.1:1")))

		return
	}

	switch v.Kind() { //nolint:exhaustive
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.String:
		v.SetString("x")
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillNonZero(t, v.Elem())
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillNonZero(t, v.Index(0))
	case reflect.Map:
		key, elem := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fillNonZero(t, key)
		fillNonZero(t, elem)

		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(key, elem)
	case reflect.Struct:
		for _, field := range v.Fields() {
			if field.CanSet() {
				fillNonZero(t, field)
			}
		}
	}

	if v.IsZero() {
		t.Fatalf("fillNonZero can't populate %s, extend it", v.Type())
	}
}
