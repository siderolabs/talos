/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kernel

import (
	"io/ioutil"
	"strings"
	"sync"
)

// NewDefaultCmdline returns a set of kernel parameters that serve as the base
// for all Talos installations.
// nolint: golint
func NewDefaultCmdline() *Cmdline {
	cmdline := NewCmdline("")
	cmdline.Append("page_poison", "1")
	cmdline.Append("slab_nomerge", "")
	cmdline.Append("pti", "on")
	// TODO(andrewrynhard): Add slub_debug=P. See https://github.com/talos-systems/talos/pull/157.
	cmdline.Append("consoleblank", "0")
	// AWS recommends setting the nvme_core.io_timeout to the highest value possible.
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html.
	cmdline.Append("nvme_core.io_timeout", "4294967295")
	cmdline.Append("random.trust_cpu", "on")
	// Disable rate limited printk
	cmdline.Append("printk.devkmsg", "on")
	// Enable early kernel message logging
	cmdline.Append("earlyprintk", "ttyS0,115200")
	// NB: We make console=tty0 the last device on the list since the last
	// device will be used when you open /dev/console.
	// AWS: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/UserProvidedKernels.html
	// VMWare: https://kb.vmware.com/s/article/2009269
	// GCP: https://cloud.google.com/compute/docs/instances/interacting-with-serial-console
	// Azure: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/create-upload-generic#general-linux-system-requirements
	cmdline.Append("console", "tty0")
	cmdline.Append("console", "tty1")
	cmdline.Append("console", "ttyS0,115200n8")

	return cmdline
}

// Key represents a key in a kernel parameter key-value pair.
type Key = string

// Parameter represents a value in a kernel parameter key-value pair.
type Parameter struct {
	key    Key
	values []string
}

// NewParameter initializes and returns a Parameter.
func NewParameter(k string) *Parameter {
	return &Parameter{
		key:    k,
		values: []string{},
	}
}

// Append appends a string to a value's internal representation.
func (v *Parameter) Append(s string) *Parameter {
	v.values = append(v.values, s)

	return v
}

// First attempts to return the first string of a value's internal
// representation.
func (v *Parameter) First() *string {
	switch {
	case v == nil:
		return nil
	case v.values == nil:
		return nil
	case len(v.values) > 0:
		return &v.values[0]
	default:
		return nil
	}
}

// Get attempts to get a string from a value's internal representation.
func (v *Parameter) Get(idx int) *string {
	if len(v.values) > idx {
		return &v.values[idx]
	}

	return nil
}

// Contains returns a boolean indicating the existence of a value.
func (v *Parameter) Contains(s string) (ok bool) {
	for _, value := range v.values {
		if ok = s == value; ok {
			return ok
		}
	}

	return ok
}

// Key returns the value's key.
func (v *Parameter) Key() string {
	return v.key
}

// Parameters represents /proc/cmdline.
type Parameters []*Parameter

// String returns a string representation of all parameters.
func (p Parameters) String() string {
	s := ""
	for _, v := range p {
		for _, val := range v.values {
			if val == "" {
				s += v.key + " "
			} else {
				s += v.key + "=" + val + " "
			}
		}
	}

	return strings.TrimRight(s, " ")
}

// Cmdline represents a set of kernel parameters.
type Cmdline struct {
	sync.Mutex
	Parameters
}

var instance *Cmdline
var once sync.Once

// Cmdline returns a representation of /proc/cmdline.
// nolint: golint
func ProcCmdline() *Cmdline {
	once.Do(func() {
		var err error
		if instance, err = read(); err != nil {
			panic(err)
		}
	})

	return instance
}

// NewCmdline initializes and returns a representation of the cmdline values
// specified by `parameters`.
// nolint: golint
func NewCmdline(parameters string) *Cmdline {
	parsed := parse(parameters)
	c := &Cmdline{sync.Mutex{}, parsed}

	return c
}

// Get gets a kernel parameter.
func (c *Cmdline) Get(k string) (value *Parameter) {
	c.Lock()
	defer c.Unlock()
	for _, value := range c.Parameters {
		if value.key == k {
			return value
		}
	}

	return nil
}

// Set sets a kernel parameter.
func (c *Cmdline) Set(k string, v *Parameter) {
	c.Lock()
	defer c.Unlock()
	for i, value := range c.Parameters {
		if value.key == k {
			c.Parameters = append(c.Parameters[:i], append([]*Parameter{v}, c.Parameters[i:]...)...)
			return
		}
	}
}

// Append appends a kernel parameter.
func (c *Cmdline) Append(k string, v string) {
	c.Lock()
	defer c.Unlock()
	for _, value := range c.Parameters {
		if value.key == k {
			value.Append(v)
			return
		}
	}
	insert(&c.Parameters, k, v)
}

// AppendAll appends a set of kernel parameters.
func (c *Cmdline) AppendAll(args []string) error {
	parameters := parse(strings.Join(args, " "))
	c.Parameters = append(c.Parameters, parameters...)

	return nil
}

// Bytes returns the byte slice representation of the cmdline struct.
func (c *Cmdline) Bytes() []byte {
	return []byte(c.String())
}

func insert(values *Parameters, key, value string) {
	for _, v := range *values {
		if v.key == key {
			v.Append(value)
			return
		}
	}
	*values = append(*values, &Parameter{key: key, values: []string{value}})
}

func parse(parameters string) (parsed Parameters) {
	line := strings.TrimSuffix(parameters, "\n")
	fields := strings.Fields(line)
	parsed = make(Parameters, 0)
	for _, arg := range fields {
		kv := strings.SplitN(arg, "=", 2)
		switch len(kv) {
		case 1:
			insert(&parsed, kv[0], "")
		case 2:
			insert(&parsed, kv[0], kv[1])
		}
	}

	return parsed
}

func read() (c *Cmdline, err error) {
	var parameters []byte
	parameters, err = ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return nil, err
	}

	parsed := parse(string(parameters))
	c = &Cmdline{sync.Mutex{}, parsed}

	return c, nil
}
