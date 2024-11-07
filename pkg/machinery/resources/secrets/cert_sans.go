// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// CertSANType is type of CertSAN resource.
const CertSANType = resource.Type("CertSANs.secrets.talos.dev")

// CertSANAPIID is a resource ID of singleton instance for the Talos API.
const CertSANAPIID = resource.ID("api")

// CertSANMaintenanceID is a resource ID of singleton instance for the Talos Maintenance API.
const CertSANMaintenanceID = resource.ID("maintenance")

// CertSANKubernetesID is a resource ID of singleton instance for the Kubernetes API Server.
const CertSANKubernetesID = resource.ID("k8s")

// CertSAN contains certficiate subject alternative names.
type CertSAN = typed.Resource[CertSANSpec, CertSANExtension]

// CertSANSpec describes fields of the cert SANs.
//
//gotagsrewrite:gen
type CertSANSpec struct {
	IPs      []netip.Addr `yaml:"ips" protobuf:"1"`
	DNSNames []string     `yaml:"dnsNames" protobuf:"2"`
	FQDN     string       `yaml:"fqdn" protobuf:"3"`
}

// NewCertSAN initializes a Etc resource.
func NewCertSAN(namespace resource.Namespace, id resource.ID) *CertSAN {
	return typed.NewResource[CertSANSpec, CertSANExtension](
		resource.NewMetadata(namespace, CertSANType, id, resource.VersionUndefined),
		CertSANSpec{},
	)
}

// CertSANExtension is a resource data of CertSAN.
type CertSANExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CertSANExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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
		if ip, err := netip.ParseAddr(san); err == nil {
			spec.AppendIPs(ip)
		} else {
			spec.AppendDNSNames(san)
		}
	}
}

// AppendIPs skipping duplicates.
func (spec *CertSANSpec) AppendIPs(ips ...netip.Addr) {
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
	return xslices.Map(spec.IPs, func(ip netip.Addr) net.IP { return ip.AsSlice() })
}

// Sort the CertSANs.
func (spec *CertSANSpec) Sort() {
	slices.Sort(spec.DNSNames)
	slices.SortFunc(spec.IPs, func(a, b netip.Addr) int { return a.Compare(b) })
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CertSANSpec](CertSANType, &CertSAN{})
	if err != nil {
		panic(err)
	}
}
