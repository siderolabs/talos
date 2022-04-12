// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net"
	"sort"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// CertSANType is type of CertSAN resource.
const CertSANType = resource.Type("CertSANs.secrets.talos.dev")

// CertSANAPIID is a resource ID of singleton instance for the Talos API.
const CertSANAPIID = resource.ID("api")

// CertSANKubernetesID is a resource ID of singleton instance for the Kubernetes API Server.
const CertSANKubernetesID = resource.ID("k8s")

// CertSAN contains certficiate subject alternative names.
type CertSAN struct {
	md   resource.Metadata
	spec CertSANSpec
}

// CertSANSpec describes fields of the cert SANs.
type CertSANSpec struct {
	IPs      []netaddr.IP `yaml:"ips"`
	DNSNames []string     `yaml:"dnsNames"`
	FQDN     string       `yaml:"fqdn"`
}

// NewCertSAN initializes a Etc resource.
func NewCertSAN(namespace resource.Namespace, id resource.ID) *CertSAN {
	r := &CertSAN{
		md:   resource.NewMetadata(namespace, CertSANType, id, resource.VersionUndefined),
		spec: CertSANSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *CertSAN) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *CertSAN) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *CertSAN) DeepCopy() resource.Resource {
	return &CertSAN{
		md: r.md,
		spec: CertSANSpec{
			IPs:      append([]netaddr.IP(nil), r.spec.IPs...),
			DNSNames: append([]string(nil), r.spec.DNSNames...),
			FQDN:     r.spec.FQDN,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *CertSAN) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CertSANType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *CertSAN) TypedSpec() *CertSANSpec {
	return &r.spec
}

// Append list of SANs splitting into IPs/DNS names.
func (spec *CertSANSpec) Append(sans ...string) {
	for _, san := range sans {
		if ip, err := netaddr.ParseIP(san); err == nil {
			spec.AppendIPs(ip)
		} else {
			spec.AppendDNSNames(san)
		}
	}
}

// AppendIPs skipping duplicates.
func (spec *CertSANSpec) AppendIPs(ips ...netaddr.IP) {
	for _, ip := range ips {
		found := false

		for _, addr := range spec.IPs {
			if addr == ip {
				found = true

				break
			}
		}

		if !found {
			spec.IPs = append(spec.IPs, ip)
		}
	}
}

// AppendStdIPs is same as AppendIPs, but for net.IP.
func (spec *CertSANSpec) AppendStdIPs(ips ...net.IP) {
	for _, ip := range ips {
		if nip, ok := netaddr.FromStdIP(ip); ok {
			spec.AppendIPs(nip)
		}
	}
}

// AppendDNSNames skipping duplicates.
func (spec *CertSANSpec) AppendDNSNames(dnsNames ...string) {
	for _, dnsName := range dnsNames {
		found := false

		for _, name := range spec.DNSNames {
			if name == dnsName {
				found = true

				break
			}
		}

		if !found {
			spec.DNSNames = append(spec.DNSNames, dnsName)
		}
	}
}

// StdIPs returns a list of converted std.IPs.
func (spec *CertSANSpec) StdIPs() []net.IP {
	result := make([]net.IP, len(spec.IPs))

	for i := range spec.IPs {
		result[i] = spec.IPs[i].IPAddr().IP
	}

	return result
}

// Sort the CertSANs.
func (spec *CertSANSpec) Sort() {
	sort.Strings(spec.DNSNames)
	sort.Slice(spec.IPs, func(i, j int) bool { return spec.IPs[i].Compare(spec.IPs[j]) < 0 })
}
