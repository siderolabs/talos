// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/bytesize"
)

// DiskRequest is the configuration required for disk creation.
type DiskRequest struct {
	Driver string
	Size   bytesize.ByteSize
	Tag    string
	Serial string
}

// Drivers that support tag option.
var tagDrivers = []string{"virtiofs"}

// Drivers that support serial option.
var serialDrivers = []string{"virtio"}

// ParseDisksFlag parses the disks flag into a slice of DiskRequests.
func ParseDisksFlag(disks []string) ([]DiskRequest, error) {
	result := []DiskRequest{}

	if len(disks) == 0 {
		return nil, errors.New("at least one disk has to be specified")
	}

	for _, d := range disks {
		parts := strings.Split(d, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid disk format: %q", d)
		}

		size := bytesize.WithDefaultUnit("MiB")

		err := size.Set(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid size in disk spec: %q", d)
		}

		req := DiskRequest{
			Driver: parts[0],
			Size:   *size,
		}

		if len(parts) > 2 {
			for _, part := range parts[2:] {
				if err := parseKVParams(&req, part); err != nil {
					return nil, fmt.Errorf("%w: %q", err, d)
				}
			}
		}

		result = append(result, req)
	}

	return result, nil
}

func parseKVParams(req *DiskRequest, part string) error {
	kv := strings.SplitN(part, "=", 2)
	if len(kv) != 2 {
		return fmt.Errorf("invalid disk option in spec: %q", part)
	}

	switch kv[0] {
	case "tag":
		if !slices.Contains(tagDrivers, req.Driver) {
			return fmt.Errorf("tag option is only supported for %v drivers in spec", tagDrivers)
		}

		req.Tag = kv[1]
	case "serial":
		if !slices.Contains(serialDrivers, req.Driver) {
			return fmt.Errorf("serial option is only supported for %v drivers in spec", serialDrivers)
		}

		req.Serial = kv[1]
	default:
		return fmt.Errorf("unknown disk option %q in spec", kv[0])
	}

	return nil
}

// Disks implements pflag.Value for accumulating multiple DiskRequest entries.
// It accepts repeated uses of the flag (e.g., --disks a:1GiB --disks b:10GiB)
// and comma-separated lists (e.g., --disks a:1GiB,b:10GiB).
type Disks struct {
	requests []DiskRequest
}

// String returns a string representation suitable for flag printing.
func (f *Disks) String() string {
	if f == nil || len(f.requests) == 0 {
		return ""
	}

	parts := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		sb := strings.Builder{}

		sb.WriteString(r.Driver)
		sb.WriteString(":")
		sb.WriteString(r.Size.String())

		if r.Tag != "" {
			sb.WriteString(":tag=")
			sb.WriteString(r.Tag)
		}

		if r.Serial != "" {
			sb.WriteString(":serial=")
			sb.WriteString(r.Serial)
		}

		parts = append(parts, sb.String())
	}

	return strings.Join(parts, ",")
}

// Set parses and appends one or more disk specifications to the flag value.
// The input may contain a single spec ("driver:size") or a comma-separated list.
func (f *Disks) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("disk value must not be empty")
	}
	// Support comma-separated values in a single Set call.
	raw := strings.Split(value, ",")

	reqs, err := ParseDisksFlag(raw)
	if err != nil {
		return err
	}

	f.requests = reqs

	return nil
}

// Type returns the flag's value type name.
func (f *Disks) Type() string { return "disks" }

// Requests returns a defensive copy of the accumulated disk requests.
func (f *Disks) Requests() []DiskRequest {
	out := make([]DiskRequest, len(f.requests))
	copy(out, f.requests)

	return out
}
