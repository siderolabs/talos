/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package etc

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/talos-systems/talos/internal/pkg/version"
)

const hostsTemplate = `
127.0.0.1       localhost
127.0.0.1       {{ .Hostname }}
{{ .IP }}       {{ .Hostname }}
::1             localhost ip6-localhost ip6-loopback
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters
`

const osReleaseTemplate = `
NAME="{{ .Name }}"
ID={{ .ID }}
VERSION_ID={{ .Version }}
PRETTY_NAME="{{ .Name }} ({{ .Version }})"
HOME_URL="https://docs.talos-systems.com/"
BUG_REPORT_URL="https://github.com/talos-systems/talos/issues"
`

// Hosts renders a valid /etc/hosts file and writes it to disk.
func Hosts(s, hostname, ip string) (err error) {
	data := struct {
		IP       string
		Hostname string
	}{
		IP:       ip,
		Hostname: hostname,
	}

	tmpl, err := template.New("").Parse(hostsTemplate)
	if err != nil {
		return
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, data)
	if err != nil {
		return
	}

	if err := ioutil.WriteFile("/run/hosts", writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /etc/hosts: %v", err)
	}

	// The kubelet wants to manage /etc/hosts. Create a symlink there that
	// points to a writable file.
	return createSymlink("/run/hosts", path.Join(s, "/etc/hosts"))
}

// ResolvConf copies the resolv.conf generated in the early boot to the new
// root.
func ResolvConf(s string) (err error) {
	source, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join(s, "/etc/resolv.conf"), source, 0644)
}

//
func createSymlink(source string, target string) (err error) {
	if _, err = os.Lstat(target); err == nil {
		if err = os.Remove(target); err != nil {
			return fmt.Errorf("remove symlink %s: %v", target, err)
		}
	}
	if err = os.Symlink(source, target); err != nil {
		return fmt.Errorf("symlink %s -> %s: %v", target, source, err)
	}

	return nil

}

// OSRelease renders a valid /etc/os-release file and writes it to disk. The
// node's OS Image field is reported by the node from /etc/os-release.
func OSRelease(s string) (err error) {
	var v string
	switch version.Tag {
	case "none":
		v = version.SHA
	default:
		v = version.Tag
	}
	data := struct {
		Name    string
		ID      string
		Version string
	}{
		Name:    version.Name,
		ID:      strings.ToLower(version.Name),
		Version: v,
	}

	tmpl, err := template.New("").Parse(osReleaseTemplate)
	if err != nil {
		return
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, data)
	if err != nil {
		return
	}

	if err := ioutil.WriteFile(path.Join(s, "/etc/os-release"), writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /etc/os-release: %v", err)
	}

	return nil
}

// DefaultGateway parses /proc/net/route for the IP address of the default
// gateway.
func DefaultGateway() (s string, err error) {
	handle, err := os.Open("/proc/net/route")
	if err != nil {
		return
	}
	// nolint: errcheck
	defer handle.Close()
	scanner := bufio.NewScanner(handle)

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 3 {
			return s, fmt.Errorf("expected at least 3 fields from /proc/net/route, got %d", len(parts))
		}
		// Skip the header.
		if parts[0] == "Iface" {
			continue
		}
		destination := parts[1]
		gateway := parts[2]
		// We are looking for the default gateway.
		if destination == "00000000" {
			var decoded []byte
			decoded, err = hex.DecodeString(gateway)
			if err != nil {
				return
			}
			s = fmt.Sprintf("%v.%v.%v.%v", decoded[3], decoded[2], decoded[1], decoded[0])
			break
		}
	}

	return s, nil
}
