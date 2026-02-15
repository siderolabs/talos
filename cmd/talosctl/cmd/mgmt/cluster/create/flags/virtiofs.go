// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags

import (
	"errors"
	"fmt"
	"strings"
)

// VirtiofsRequest is the configuration required for virtiofs share creation.
type VirtiofsRequest struct {
	SharedDir  string
	SocketPath string
}

// ParseVirtiofsFlag parses the virtiofs flag into a slice of VirtiofsRequest.
func ParseVirtiofsFlag(disks []string) ([]VirtiofsRequest, error) {
	result := []VirtiofsRequest{}

	if len(disks) == 0 {
		return nil, errors.New("at least one disk has to be specified")
	}

	for _, d := range disks {
		parts := strings.SplitN(d, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid disk format: %q", d)
		}

		result = append(result, VirtiofsRequest{
			SharedDir:  parts[0],
			SocketPath: parts[1],
		})
	}

	return result, nil
}

// Virtiofs implements pflag.Value for accumulating multiple VirtiofsRequest entries.
type Virtiofs struct {
	requests []VirtiofsRequest
}

// String returns a string representation suitable for flag printing.
func (f *Virtiofs) String() string {
	if f == nil || len(f.requests) == 0 {
		return ""
	}

	parts := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		parts = append(parts, fmt.Sprintf("%s:%s", r.SharedDir, r.SocketPath))
	}

	return strings.Join(parts, ",")
}

// Set parses and appends one or more disk specifications to the flag value.
// The input may contain a single spec ("sharedDir:socketPath") or a comma-separated list.
func (f *Virtiofs) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("virtiofs value must not be empty")
	}
	// Support comma-separated values in a single Set call.
	raw := strings.Split(value, ",")

	reqs, err := ParseVirtiofsFlag(raw)
	if err != nil {
		return err
	}

	f.requests = reqs

	return nil
}

// Type returns the flag's value type name.
func (f *Virtiofs) Type() string { return "virtiofs" }

// Requests returns a defensive copy of the accumulated virtiofs share requests.
func (f *Virtiofs) Requests() []VirtiofsRequest {
	out := make([]VirtiofsRequest, len(f.requests))
	copy(out, f.requests)

	return out
}
