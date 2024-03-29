// Code generated by "enumer -type=RouteFlag -linecomment -text"; DO NOT EDIT.

package nethelpers

import (
	"fmt"
	"strings"
)

const (
	_RouteFlagName_0      = "notify"
	_RouteFlagLowerName_0 = "notify"
	_RouteFlagName_1      = "cloned"
	_RouteFlagLowerName_1 = "cloned"
	_RouteFlagName_2      = "equalize"
	_RouteFlagLowerName_2 = "equalize"
	_RouteFlagName_3      = "prefix"
	_RouteFlagLowerName_3 = "prefix"
	_RouteFlagName_4      = "lookup_table"
	_RouteFlagLowerName_4 = "lookup_table"
	_RouteFlagName_5      = "fib_match"
	_RouteFlagLowerName_5 = "fib_match"
	_RouteFlagName_6      = "offload"
	_RouteFlagLowerName_6 = "offload"
	_RouteFlagName_7      = "trap"
	_RouteFlagLowerName_7 = "trap"
)

var (
	_RouteFlagIndex_0 = [...]uint8{0, 6}
	_RouteFlagIndex_1 = [...]uint8{0, 6}
	_RouteFlagIndex_2 = [...]uint8{0, 8}
	_RouteFlagIndex_3 = [...]uint8{0, 6}
	_RouteFlagIndex_4 = [...]uint8{0, 12}
	_RouteFlagIndex_5 = [...]uint8{0, 9}
	_RouteFlagIndex_6 = [...]uint8{0, 7}
	_RouteFlagIndex_7 = [...]uint8{0, 4}
)

func (i RouteFlag) String() string {
	switch {
	case i == 256:
		return _RouteFlagName_0
	case i == 512:
		return _RouteFlagName_1
	case i == 1024:
		return _RouteFlagName_2
	case i == 2048:
		return _RouteFlagName_3
	case i == 4096:
		return _RouteFlagName_4
	case i == 8192:
		return _RouteFlagName_5
	case i == 16384:
		return _RouteFlagName_6
	case i == 32768:
		return _RouteFlagName_7
	default:
		return fmt.Sprintf("RouteFlag(%d)", i)
	}
}

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the stringer command to generate them again.
func _RouteFlagNoOp() {
	var x [1]struct{}
	_ = x[RouteNotify-(256)]
	_ = x[RouteCloned-(512)]
	_ = x[RouteEqualize-(1024)]
	_ = x[RoutePrefix-(2048)]
	_ = x[RouteLookupTable-(4096)]
	_ = x[RouteFIBMatch-(8192)]
	_ = x[RouteOffload-(16384)]
	_ = x[RouteTrap-(32768)]
}

var _RouteFlagValues = []RouteFlag{RouteNotify, RouteCloned, RouteEqualize, RoutePrefix, RouteLookupTable, RouteFIBMatch, RouteOffload, RouteTrap}

var _RouteFlagNameToValueMap = map[string]RouteFlag{
	_RouteFlagName_0[0:6]:       RouteNotify,
	_RouteFlagLowerName_0[0:6]:  RouteNotify,
	_RouteFlagName_1[0:6]:       RouteCloned,
	_RouteFlagLowerName_1[0:6]:  RouteCloned,
	_RouteFlagName_2[0:8]:       RouteEqualize,
	_RouteFlagLowerName_2[0:8]:  RouteEqualize,
	_RouteFlagName_3[0:6]:       RoutePrefix,
	_RouteFlagLowerName_3[0:6]:  RoutePrefix,
	_RouteFlagName_4[0:12]:      RouteLookupTable,
	_RouteFlagLowerName_4[0:12]: RouteLookupTable,
	_RouteFlagName_5[0:9]:       RouteFIBMatch,
	_RouteFlagLowerName_5[0:9]:  RouteFIBMatch,
	_RouteFlagName_6[0:7]:       RouteOffload,
	_RouteFlagLowerName_6[0:7]:  RouteOffload,
	_RouteFlagName_7[0:4]:       RouteTrap,
	_RouteFlagLowerName_7[0:4]:  RouteTrap,
}

var _RouteFlagNames = []string{
	_RouteFlagName_0[0:6],
	_RouteFlagName_1[0:6],
	_RouteFlagName_2[0:8],
	_RouteFlagName_3[0:6],
	_RouteFlagName_4[0:12],
	_RouteFlagName_5[0:9],
	_RouteFlagName_6[0:7],
	_RouteFlagName_7[0:4],
}

// RouteFlagString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func RouteFlagString(s string) (RouteFlag, error) {
	if val, ok := _RouteFlagNameToValueMap[s]; ok {
		return val, nil
	}

	if val, ok := _RouteFlagNameToValueMap[strings.ToLower(s)]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to RouteFlag values", s)
}

// RouteFlagValues returns all values of the enum
func RouteFlagValues() []RouteFlag {
	return _RouteFlagValues
}

// RouteFlagStrings returns a slice of all String values of the enum
func RouteFlagStrings() []string {
	strs := make([]string, len(_RouteFlagNames))
	copy(strs, _RouteFlagNames)
	return strs
}

// IsARouteFlag returns "true" if the value is listed in the enum definition. "false" otherwise
func (i RouteFlag) IsARouteFlag() bool {
	for _, v := range _RouteFlagValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalText implements the encoding.TextMarshaler interface for RouteFlag
func (i RouteFlag) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for RouteFlag
func (i *RouteFlag) UnmarshalText(text []byte) error {
	var err error
	*i, err = RouteFlagString(string(text))
	return err
}
