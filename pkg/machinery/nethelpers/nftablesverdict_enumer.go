// Code generated by "enumer -type=NfTablesVerdict -linecomment -text"; DO NOT EDIT.

package nethelpers

import (
	"fmt"
	"strings"
)

const _NfTablesVerdictName = "dropaccept"

var _NfTablesVerdictIndex = [...]uint8{0, 4, 10}

const _NfTablesVerdictLowerName = "dropaccept"

func (i NfTablesVerdict) String() string {
	if i < 0 || i >= NfTablesVerdict(len(_NfTablesVerdictIndex)-1) {
		return fmt.Sprintf("NfTablesVerdict(%d)", i)
	}
	return _NfTablesVerdictName[_NfTablesVerdictIndex[i]:_NfTablesVerdictIndex[i+1]]
}

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the stringer command to generate them again.
func _NfTablesVerdictNoOp() {
	var x [1]struct{}
	_ = x[VerdictDrop-(0)]
	_ = x[VerdictAccept-(1)]
}

var _NfTablesVerdictValues = []NfTablesVerdict{VerdictDrop, VerdictAccept}

var _NfTablesVerdictNameToValueMap = map[string]NfTablesVerdict{
	_NfTablesVerdictName[0:4]:       VerdictDrop,
	_NfTablesVerdictLowerName[0:4]:  VerdictDrop,
	_NfTablesVerdictName[4:10]:      VerdictAccept,
	_NfTablesVerdictLowerName[4:10]: VerdictAccept,
}

var _NfTablesVerdictNames = []string{
	_NfTablesVerdictName[0:4],
	_NfTablesVerdictName[4:10],
}

// NfTablesVerdictString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func NfTablesVerdictString(s string) (NfTablesVerdict, error) {
	if val, ok := _NfTablesVerdictNameToValueMap[s]; ok {
		return val, nil
	}

	if val, ok := _NfTablesVerdictNameToValueMap[strings.ToLower(s)]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to NfTablesVerdict values", s)
}

// NfTablesVerdictValues returns all values of the enum
func NfTablesVerdictValues() []NfTablesVerdict {
	return _NfTablesVerdictValues
}

// NfTablesVerdictStrings returns a slice of all String values of the enum
func NfTablesVerdictStrings() []string {
	strs := make([]string, len(_NfTablesVerdictNames))
	copy(strs, _NfTablesVerdictNames)
	return strs
}

// IsANfTablesVerdict returns "true" if the value is listed in the enum definition. "false" otherwise
func (i NfTablesVerdict) IsANfTablesVerdict() bool {
	for _, v := range _NfTablesVerdictValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalText implements the encoding.TextMarshaler interface for NfTablesVerdict
func (i NfTablesVerdict) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for NfTablesVerdict
func (i *NfTablesVerdict) UnmarshalText(text []byte) error {
	var err error
	*i, err = NfTablesVerdictString(string(text))
	return err
}
