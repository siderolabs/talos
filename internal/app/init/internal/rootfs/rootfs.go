/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"io/ioutil"
	"log"
	stdlibnet "net"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/cni"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/etc"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/proc"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	yaml "gopkg.in/yaml.v2"
)

func ip() string {
	addrs, err := stdlibnet.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*stdlibnet.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

// Prepare creates the files required by the installed binaries and libraries.
// nolint: gocyclo
func Prepare(s string, inContainer bool, data *userdata.UserData) (err error) {
	if !inContainer {
		// Enable IP forwarding.
		if err = proc.WriteSystemProperty(&proc.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
			return
		}
		// Kernel Self Protection Project recommended settings.
		if err = kernelHardening(); err != nil {
			return
		}
		// Create /etc/hosts.
		var hostname string
		if hostname, err = os.Hostname(); err != nil {
			return
		}
		ip := ip()
		if err = etc.Hosts(s, hostname, ip); err != nil {
			return
		}
		// Create /etc/resolv.conf.
		if err = etc.ResolvConf(s); err != nil {
			return
		}
	}

	// Create /etc/os-release.
	if err = etc.OSRelease(s); err != nil {
		return
	}
	// Setup directories required by the CNI plugin.
	if err = cni.Setup(s, data); err != nil {
		return
	}
	// Generate the identity certificate.
	if err = generatePKI(data); err != nil {
		return
	}
	// Save the user data to disk.
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(constants.UserDataPath, dataBytes, 0400); err != nil {
		return
	}

	return nil
}

func generatePKI(data *userdata.UserData) (err error) {
	log.Println("generating node identity PKI")
	if data.IsBootstrap() {
		log.Println("generating PKI locally")
		var csr *x509.CertificateSigningRequest
		if csr, err = data.NewIdentityCSR(); err != nil {
			return err
		}
		var crt *x509.Certificate
		crt, err = x509.NewCertificateFromCSRBytes(data.Security.OS.CA.Crt, data.Security.OS.CA.Key, csr.X509CertificateRequestPEM, x509.NotAfter(time.Now().Add(time.Duration(8760)*time.Hour)))
		if err != nil {
			return err
		}
		data.Security.OS.Identity.Crt = crt.X509CertificatePEM
		return nil
	}

	log.Println("generating PKI from trustd")
	var generator *gen.Generator
	generator, err = gen.NewGenerator(data, constants.TrustdPort)
	if err != nil {
		return errors.Wrap(err, "failed to create trustd client")
	}
	if err = generator.Identity(data); err != nil {
		return errors.Wrap(err, "failed to generate identity")
	}

	return nil
}

// We can ignore setting kernel.kexec_load_disabled = 1 because modules are
// disabled in the kernel config.
func kernelHardening() (err error) {
	props := []*proc.SystemProperty{
		{
			Key:   "kernel.kptr_restrict",
			Value: "1",
		},
		{
			Key:   "kernel.dmesg_restrict",
			Value: "1",
		},
		{
			Key:   "kernel.perf_event_paranoid",
			Value: "3",
		},
		// {
		// 	Key:   "kernel.kexec_load_disabled",
		// 	Value: "1",
		// },
		{
			Key:   "kernel.yama.ptrace_scope",
			Value: "1",
		},
		{
			Key:   "user.max_user_namespaces",
			Value: "0",
		},
		// {
		// 	Key:   "kernel.unprivileged_bpf_disabled",
		// 	Value: "1",
		// },
		// {
		// 	Key:   "net.core.bpf_jit_harden",
		// 	Value: "2",
		// },
	}

	for _, prop := range props {
		if err = proc.WriteSystemProperty(prop); err != nil {
			return
		}
	}

	return nil
}
