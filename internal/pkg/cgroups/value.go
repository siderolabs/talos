// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Value represents a cgroup value.
//
// Value might represent 'max' value.
type Value struct {
	Val   int64
	Frac  int
	IsMax bool
	IsSet bool
}

// String returns the string representation of the cgroup value.
func (v Value) String() string {
	switch {
	case !v.IsSet:
		return "unset"
	case v.IsMax:
		return "max"
	default:
		s := strconv.FormatInt(v.Val, 10)

		if v.Frac == 0 {
			return s
		}

		if len(s) < v.Frac+1 {
			s = strings.Repeat("0", (v.Frac+1)-len(s)) + s
		}

		return s[:len(s)-v.Frac] + "." + s[len(s)-v.Frac:]
	}
}

// HumanizeIBytes returns the humanized bytes representation of the cgroup value.
func (v Value) HumanizeIBytes() string {
	if !v.IsSet || v.IsMax || v.Frac > 0 {
		return v.String()
	}

	return humanize.IBytes(uint64(v.Val))
}

// DivideBy returns the value divided by another value in percentage.
//
// a.DivideBy(b) = a / b * 100.
func (v Value) DivideBy(other Value) Value {
	switch {
	case !v.IsSet || !other.IsSet:
		// if either value is unset, return unset
		return Value{}
	case other.IsMax && !v.IsMax:
		// if other is max and v is not, return 0.00%
		return Value{IsSet: true, Frac: 2}
	case v.IsMax && other.IsMax:
		// if both are max, return 100.00%
		return Value{Val: 10000, IsSet: true, Frac: 2}
	case other.Val == 0 || v.IsMax:
		// if other is 0, return max
		return Value{IsMax: true, IsSet: true}
	default:
		return Value{Val: int64(math.Round(float64(v.Val) / float64(other.Val) * 100 * 100)), IsSet: true, Frac: 2}
	}
}

// UsecToDuration returns the duration representation of the cgroup value in microseconds.
func (v Value) UsecToDuration() string {
	if !v.IsSet || v.IsMax {
		return v.String()
	}

	return (time.Duration(v.Val) * time.Microsecond).String()
}

// Values represents a list of cgroup values.
type Values []Value

// FlatMap returns the flat map of the cgroup values.
type FlatMap map[string]Value

// NestedKeyed returns the nested keyed map of the cgroup values.
type NestedKeyed map[string]FlatMap

// ParseValue parses the cgroup value from the string.
func ParseValue(s string) (Value, error) {
	if s == "max" {
		return Value{IsMax: true, IsSet: true}, nil
	}

	var frac int

	l, r, ok := strings.Cut(s, ".")
	if ok {
		frac = len(r)

		s = l + r
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Value{}, err
	}

	return Value{Val: val, Frac: frac, IsSet: true}, nil
}

// ParseNewlineSeparatedValues parses the cgroup values from the newline separated string.
//
// New-line separated values
// (when only one value can be written at once)
//
// VAL0\n
// VAL1\n
// ...
func ParseNewlineSeparatedValues(r io.Reader) (Values, error) {
	scanner := bufio.NewScanner(r)

	var values Values

	for scanner.Scan() {
		val, err := ParseValue(scanner.Text())
		if err != nil {
			return nil, err
		}

		values = append(values, val)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

// ParseSpaceSeparatedValues parses the cgroup values from the space separated string.
//
// Space separated values
// (when read-only or multiple values can be written at once)
//
// VAL0 VAL1 ...\n.
func ParseSpaceSeparatedValues(r io.Reader) (Values, error) {
	scanner := bufio.NewScanner(r)

	if !scanner.Scan() {
		return nil, nil
	}

	line := scanner.Text()
	parts := strings.Fields(line)

	values := make(Values, 0, len(parts))

	for _, s := range parts {
		val, err := ParseValue(s)
		if err != nil {
			return nil, err
		}

		values = append(values, val)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

// ParseFlatMapValues parses the cgroup values from the flat map.
//
// Flat keyed:
//
// KEY0 VAL0\n
// KEY1 VAL1\n
// ...
func ParseFlatMapValues(r io.Reader) (FlatMap, error) {
	scanner := bufio.NewScanner(r)

	flatMap := FlatMap{}

	for scanner.Scan() {
		line := scanner.Text()

		key, value, ok := strings.Cut(line, " ")
		if !ok {
			return nil, errors.New("invalid format")
		}

		val, err := ParseValue(value)
		if err != nil {
			return nil, err
		}

		flatMap[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return flatMap, nil
}

// ParseNestedKeyedValues parses the cgroup values from the nested keyed map.
//
// Nested keyed:
//
// KEY0 SUB_KEY0=VAL00 SUB_KEY1=VAL01...
// KEY1 SUB_KEY0=VAL10 SUB_KEY1=VAL11...
// ...
func ParseNestedKeyedValues(r io.Reader) (NestedKeyed, error) {
	scanner := bufio.NewScanner(r)

	nestedKeyed := NestedKeyed{}

	for scanner.Scan() {
		line := scanner.Text()

		key, values, ok := strings.Cut(line, " ")
		if !ok {
			return nil, errors.New("invalid format")
		}

		flatMap := FlatMap{}

		for _, pair := range strings.Fields(values) {
			subKey, value, ok := strings.Cut(pair, "=")
			if !ok {
				return nil, errors.New("invalid format")
			}

			val, err := ParseValue(value)
			if err != nil {
				return nil, err
			}

			flatMap[subKey] = val
		}

		nestedKeyed[key] = flatMap
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nestedKeyed, nil
}
