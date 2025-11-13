// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package merge

import (
	"fmt"
	"reflect"
	"strings"
)

// Merge two config trees together.
//
// Data in the left is replaced with data in the right unless it's zero value.
//
// This function is not supposed to be a generic merge function.
// It is specifically fine-tuned to merge Talos machine configuration.
//
// Rules:
//   - if it is a simple value (int, float, string, etc.), it's merged into the left unless it's zero value, but boolean false is always merged.
//   - if it is a pointer, merged dereferencing the pointer unless the right is nil
//   - if it is a slice, merged by concatenating the right to the left.
//   - if the `merge:"replace"` struct tag is defined, a slice is replaced with the value of the right (unless it's zero value.)
//   - slices of `[]byte` are always replaced
//   - if it is a map, for each key value is merged recursively.
//   - if it is a struct, merge is performed for each field of the struct.
//   - if the type implements 'merger' interface, Merge function is called to handle the merge process.
//   - merger interface should be implemented on the pointer to the type.
func Merge(left, right any) error {
	return merge(reflect.ValueOf(left), reflect.ValueOf(right), false, false)
}

type merger interface {
	Merge(other any) error
}

var (
	zeroValue  reflect.Value
	mergerType = reflect.TypeFor[merger]()
)

//nolint:gocyclo,cyclop
func merge(vl, vr reflect.Value, replace, overwrite bool) error {
	if vl == zeroValue && vr == zeroValue {
		return nil
	}

	tl, tr := vl.Type(), vr.Type()

	if tl != tr {
		return fmt.Errorf("merge type mismatch left %v right %v", tl, tr)
	}

	if reflect.PointerTo(tl).Implements(mergerType) {
		return vl.Addr().Interface().(merger).Merge(vr.Interface())
	}

	switch tl.Kind() { //nolint:exhaustive
	case reflect.Pointer, reflect.Interface:
		if vr.IsZero() {
			return nil
		}

		if vl.IsZero() {
			vl.Set(vr)

			return nil
		}

		return merge(vl.Elem(), vr.Elem(), replace, true)
	case reflect.Slice:
		if vr.IsZero() {
			return nil
		}

		if !vl.CanSet() {
			return fmt.Errorf("merge not possible, left %v is not settable", vl)
		}

		if replace || tl.Elem().Kind() == reflect.Uint8 {
			vl.Set(vr)

			return nil
		}

		if vl.IsNil() && vr.Len() == 0 {
			vl.Set(reflect.MakeSlice(tl, 0, 0))
		} else {
			vl.Set(reflect.AppendSlice(reflect.MakeSlice(tl, 0, 0), reflect.AppendSlice(vl, vr)))
		}
	case reflect.Map:
		if vr.IsZero() {
			return nil
		}

		if replace {
			vl.Set(vr)

			return nil
		}

		if vl.IsNil() {
			vl.Set(reflect.MakeMap(tl))
		}

		for _, k := range vr.MapKeys() {
			if !isNilOrZero(vl.MapIndex(k)) {
				valueType := tl.Elem()

				var v, rightV reflect.Value

				if valueType.Kind() == reflect.Interface { // special case for map[string]interface{}
					// here we have to instantiate the actual type behind in the `interface{}`, otherwise it's not settable
					valueType = vl.MapIndex(k).Elem().Type()

					if valueType != vr.MapIndex(k).Elem().Type() {
						return fmt.Errorf("merge type mismatch left %v right %v", valueType, vr.MapIndex(k).Elem().Type())
					}

					v = reflect.New(valueType).Elem()
					v.Set(vl.MapIndex(k).Elem())

					rightV = reflect.New(valueType).Elem()
					rightV.Set(vr.MapIndex(k).Elem())
				} else { // "normal" maps
					v = reflect.New(valueType).Elem()
					v.Set(vl.MapIndex(k))

					rightV = vr.MapIndex(k)
				}

				if err := merge(v, rightV, false, false); err != nil {
					return fmt.Errorf("merge map key %v[%v]: %w", tl, k, err)
				}

				vl.SetMapIndex(k, v)
			} else {
				vl.SetMapIndex(k, vr.MapIndex(k))
			}
		}
	case reflect.Struct:
		if replace {
			// if the right-hand struct is zero value, skip replacing the left-hand struct
			if vr.IsZero() {
				return nil
			}

			vl.Set(vr)

			return nil
		}

		for i := range tl.NumField() {
			var replace bool

			structTag := tl.Field(i).Tag.Get("merge")
			for value := range strings.SplitSeq(structTag, ",") {
				if value == "replace" {
					replace = true
				}
			}

			fl := vl.FieldByIndex(tl.Field(i).Index)
			fr := vr.FieldByIndex(tr.Field(i).Index)

			if err := merge(fl, fr, replace, false); err != nil {
				return fmt.Errorf("merge field %v.%v: %w", tl, tl.Field(i).Name, err)
			}
		}
	case
		reflect.String,
		reflect.Int,
		reflect.Uint,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Bool:
		if !vl.CanSet() {
			return fmt.Errorf("merge not possible, left %v is not settable", vl)
		}

		if tl.Kind() != reflect.Bool && vr.IsZero() && !overwrite {
			return nil
		}

		vl.Set(vr)
	default:
		return fmt.Errorf("merge not implemented for %v", tl.Kind())
	}

	return nil
}

// isNilOrZero returns true if the [reflect.Value] is zero [reflect.Value] or something that is nil.
// We need it because if map contains a key with `nil` value, simply comparing that result to the `zeroValue`
// is not enough.
func isNilOrZero(idx reflect.Value) bool {
	if idx == zeroValue {
		return true
	}

	switch idx.Kind() { //nolint:exhaustive
	case reflect.Interface, reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return idx.IsNil()
	default:
		return false
	}
}
