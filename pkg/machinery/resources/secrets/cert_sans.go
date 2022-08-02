// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net"
	"sort"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

// CertSANType is type of CertSAN resource.
const CertSANType = resource.Type("CertSANs.secrets.talos.dev")

// CertSANAPIID is a resource ID of singleton instance for the Talos API.
const CertSANAPIID = resource.ID("api")

// CertSANKubernetesID is a resource ID of singleton instance for the Kubernetes API Server.
const CertSANKubernetesID = resource.ID("k8s")

// CertSAN contains certficiate subject alternative names.
type CertSAN = typed.Resource[CertSANSpec, CertSANRD]

// CertSANSpec describes fields of the cert SANs.
//
//gotagsrewrite:gen
type CertSANSpec struct {
	IPs      []netaddr.IP `yaml:"ips" protobuf:"1"`
	DNSNames []string     `yaml:"dnsNames" protobuf:"2"`
	FQDN     string       `yaml:"fqdn" protobuf:"3"`
}

// NewCertSAN initializes a Etc resource.
func NewCertSAN(namespace resource.Namespace, id resource.ID) *CertSAN {
	return typed.NewResource[CertSANSpec, CertSANRD](
		resource.NewMetadata(namespace, CertSANType, id, resource.VersionUndefined),
		CertSANSpec{},
	)
}

// CertSANRD is a resource data of CertSAN.
type CertSANRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CertSANRD) ResourceDefinition(resource.Metadata, CertSANSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CertSANType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// Reset the list of SANs.
func (spec *CertSANSpec) Reset() {
	spec.DNSNames = nil
	spec.IPs = nil
	spec.FQDN = ""
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
	return slices.Map(spec.IPs, func(ip netaddr.IP) net.IP { return ip.IPAddr().IP })
}

// Sort the CertSANs.
func (spec *CertSANSpec) Sort() {
	sort.Strings(spec.DNSNames)
	sort.Slice(spec.IPs, func(i, j int) bool { return spec.IPs[i].Compare(spec.IPs[j]) < 0 })
}
