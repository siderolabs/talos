// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etc

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"text/template"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/version"

	"golang.org/x/sys/unix"
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
func Hosts(hostname string) (err error) {
	ip := ip()

	// If no hostname, set it to `talos-<ip>`, talos-1-2-3-4
	if hostname == "" {
		hostname = fmt.Sprintf("%s-%s", "talos", strings.ReplaceAll(ip, ".", "-"))
	}

	if err = unix.Sethostname([]byte(hostname)); err != nil {
		return err
	}

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

	if err = ioutil.WriteFile("/run/system/etc/hosts", writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /run/hosts: %w", err)
	}

	if err = unix.Mount("/run/system/etc/hosts", "/etc/hosts", "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for /etc/hosts: %w", err)
	}

	return nil
}

// ResolvConf copies the resolv.conf generated in the early boot to the new
// root.
func ResolvConf() (err error) {
	option := kernel.ProcCmdline().Get("ip").First()
	switch option {
	case nil:
		target := "/run/system/etc/resolv.conf"

		var f *os.File

		if f, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
			return err
		}

		// nolint: errcheck
		defer f.Close()

		if err = unix.Mount("/run/system/etc/resolv.conf", "/etc/resolv.conf", "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("failed to create bind mount for /etc/resolv.conf: %w", err)
		}
	default:
		if _, err = os.Stat("/proc/net/pnp"); err != nil {
			return errors.New("failed to symlink /etc/resolv.conf to /proc/net/pnp")
		}

		if err = os.Symlink("/etc/resolv.conf", "/proc/net/pnp"); err != nil {
			return err
		}
	}

	return nil
}

// OSRelease renders a valid /etc/os-release file and writes it to disk. The
// node's OS Image field is reported by the node from /etc/os-release.
func OSRelease() (err error) {
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

	if err = ioutil.WriteFile("/run/system/etc/os-release", writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /run/system/etc/os-release: %w", err)
	}

	if err = unix.Mount("/run/system/etc/os-release", "/etc/os-release", "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for /etc/os-release: %w", err)
	}

	return nil
}

func ip() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}
