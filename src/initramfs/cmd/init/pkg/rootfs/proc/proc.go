package proc

import (
	"io/ioutil"
	"path"
	"strings"
)

// SystemProperty represents a kernel system property.
type SystemProperty struct {
	Key   string
	Value string
}

// WriteSystemProperty writes a value to a key under /proc/sys.
func WriteSystemProperty(prop *SystemProperty) error {
	keyPath := path.Join("/proc/sys", strings.Replace(prop.Key, ".", "/", -1))
	return ioutil.WriteFile(keyPath, []byte(prop.Value), 0644)
}
