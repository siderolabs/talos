// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url

import (
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Variable represents a variable substitution in the download URL.
type Variable struct {
	// Key is the variable name.
	Key string
	// MatchOnArg is set for variables which are match on the arg name with empty value.
	//
	// Required to support legacy `?uuid=` style of the download URL.
	MatchOnArg bool
	// Value is the variable value.
	Value Value

	rOnce sync.Once
	r     *regexp.Regexp
}

// AllVariables is a map of all supported variables.
func AllVariables() map[string]*Variable {
	return map[string]*Variable{
		constants.UUIDKey: {
			Key:        constants.UUIDKey,
			MatchOnArg: true,
			Value:      UUIDValue(),
		},
		constants.SerialNumberKey: {
			Key:   constants.SerialNumberKey,
			Value: SerialNumberValue(),
		},
		constants.MacKey: {
			Key:   constants.MacKey,
			Value: MACValue(),
		},
		constants.HostnameKey: {
			Key:   constants.HostnameKey,
			Value: HostnameValue(),
		},
		constants.CodeKey: {
			Key:   constants.CodeKey,
			Value: CodeValue(),
		},
	}
}

func keyToVar(key string) string {
	return `${` + key + `}`
}

func (v *Variable) init() {
	v.rOnce.Do(func() {
		v.r = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(keyToVar(v.Key)))
	})
}

// Matches checks if the variable is present in the URL.
func (v *Variable) Matches(query url.Values) bool {
	v.init()

	for arg, values := range query {
		if v.MatchOnArg {
			if arg == v.Key && !(len(values) == 1 && strings.TrimSpace(values[0]) != "") {
				return true
			}
		}

		for _, value := range values {
			if v.r.MatchString(value) {
				return true
			}
		}
	}

	return false
}

// Replace modifies the URL query replacing the variable with the value.
func (v *Variable) Replace(query url.Values) {
	v.init()

	for arg, values := range query {
		if v.MatchOnArg {
			if arg == v.Key && !(len(values) == 1 && strings.TrimSpace(values[0]) != "") {
				query.Set(arg, v.Value.Get())

				continue
			}
		}

		for idx, value := range values {
			values[idx] = v.r.ReplaceAllString(value, v.Value.Get())
		}

		query[arg] = values
	}
}
