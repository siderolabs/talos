package proc

import (
	"io/ioutil"
	"path"
	"strings"
)

// WriteSystemProperty writes a value to a key under /proc/sys.
func WriteSystemProperty(key, value string) error {
	keyPath := strings.Replace(key, ".", "/", -1)
	return ioutil.WriteFile(path.Join("/proc/sys", keyPath), []byte(value), 0644)
}
