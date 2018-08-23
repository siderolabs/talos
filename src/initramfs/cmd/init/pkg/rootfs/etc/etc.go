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

	"github.com/autonomy/dianemo/src/initramfs/pkg/version"
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
PRETTY_NAME="{{ .Name }} ({{ .Version }}) by Autonomy"
HOME_URL="https://dianemo.autonomy.io/"
BUG_REPORT_URL="https://github.com/autonomy/dianemo/src/issues"
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

	if err := ioutil.WriteFile(path.Join(s, "/var/run/hosts"), writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /etc/hosts: %v", err)
	}
	if _, err := os.Lstat(path.Join(s, "/etc/hosts")); err == nil {
		if err = os.Remove(path.Join(s, "/etc/hosts")); err != nil {
			return fmt.Errorf("remove symlink /etc/hosts: %v", err)
		}
	}
	// The kubelet wants to manage /etc/hosts. Create a symlink there that
	// points to a writable file.
	if err := os.Symlink("/var/run/hosts", path.Join(s, "/etc/hosts")); err != nil {
		return fmt.Errorf("symlink /etc/hosts: %v", err)
	}

	return nil
}

// ResolvConf symlinks /proc/net/pnp to /etc/resolv.conf. See
// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt.
func ResolvConf(s string) (err error) {
	if _, err = os.Lstat(path.Join(s, "/etc/resolv.conf")); err == nil {
		if err = os.Remove(path.Join(s, "/etc/resolv.conf")); err != nil {
			return fmt.Errorf("remove symlink /etc/hosts: %v", err)
		}
	}
	if err = os.Symlink("/proc/net/pnp", path.Join(s, "/etc/resolv.conf")); err != nil {
		return
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
