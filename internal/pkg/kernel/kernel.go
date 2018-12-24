package kernel

import (
	"io/ioutil"
	"strings"
)

// ParseProcCmdline parses /proc/cmdline and returns a map reprentation of the
// kernel parameters.
func ParseProcCmdline() (cmdline map[string]string, err error) {
	cmdline = map[string]string{}
	cmdlineBytes, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return
	}
	line := strings.TrimSuffix(string(cmdlineBytes), "\n")
	arguments := strings.Split(line, " ")
	for _, a := range arguments {
		kv := strings.Split(a, "=")
		if len(kv) == 2 {
			cmdline[kv[0]] = kv[1]
		}
	}

	return cmdline, err
}
