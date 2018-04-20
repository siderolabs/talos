package etc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"text/template"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

const hostsTemplate = `
127.0.0.1       localhost
127.0.0.1       {{ .Hostname }}
{{ .IP }}       {{ .Hostname }}
::1             localhost ip6-localhost ip6-loopback
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters
`

const resolvConfTemplate = `
{{ range $_, $ip := . }}
nameserver {{ $ip }}
{{ end }}
`

func Hosts(s, hostname, ip string) error {
	data := struct {
		IP       string
		Hostname string
	}{
		IP:       ip,
		Hostname: hostname,
	}

	tmpl, err := template.New("").Parse(hostsTemplate)
	if err != nil {
		return err
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s, "/etc/hosts"), writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /etc/hosts: %s", err.Error())
	}

	return nil
}

func ResolvConf(s string, userdata userdata.UserData) error {
	tmpl, err := template.New("").Parse(resolvConfTemplate)
	if err != nil {
		return err
	}
	var buf []byte
	writer := bytes.NewBuffer(buf)
	err = tmpl.Execute(writer, userdata.Nameservers)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s, "/etc/resolv.conf"), writer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write /etc/resolv.conf: %s", err.Error())
	}

	return nil
}
