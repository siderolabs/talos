package v1alpha2

import (
	"net/url"
	"time"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

var (
	_ config.Cluster           = (*Cluster)(nil)
	_ config.ClusterNetwork    = (*ClusterNetwork)(nil)
	_ config.Etcd              = (*Etcd)(nil)
	_ config.APIServer         = (*APIServer)(nil)
	_ config.ControllerManager = (*ControllerManager)(nil)
	_ config.Scheduler         = (*Scheduler)(nil)
	_ config.Proxy             = (*Proxy)(nil)
	_ config.CoreDNS           = (*CoreDNS)(nil)
	_ config.PodCheckpointer   = (*PodCheckpointer)(nil)
	_ config.CNI               = (*CNI)(nil)
	_ config.Token             = (*Token)(nil)
	_ config.AdminKubeconfig   = (*AdminKubeconfig)(nil)
)

type Cluster struct {
	provider *Provider
}

func (c *Cluster) Name() string {
	return ""
}

func (c *Cluster) Etcd() config.Etcd {
	return nil
}

func (c *Cluster) APIServer() config.APIServer {
	return &APIServer{c.provider}
}

func (c *Cluster) ControllerManager() config.ControllerManager {
	return &ControllerManager{c.provider}
}

func (c *Cluster) Scheduler() config.Scheduler {
	return &Scheduler{c.provider}
}

func (c *Cluster) Proxy() config.Proxy {
	return &Proxy{c.provider}
}

func (c *Cluster) CoreDNS() config.CoreDNS {
	return &CoreDNS{c.provider}
}

func (c *Cluster) PodCheckpointer() config.PodCheckpointer {
	return &PodCheckpointer{c.provider}
}

func (c *Cluster) Endpoint() *url.URL {
	return nil
}

func (c *Cluster) Token() config.Token {
	return &Token{c.provider}
}

func (c *Cluster) CertSANs() []string {
	return nil
}

func (c *Cluster) SetCertSANs([]string) {
	return
}

func (c *Cluster) CA() *x509.PEMEncodedCertificateAndKey {
	return nil
}

func (c *Cluster) AESCBCEncryptionSecret() string {
	return ""
}

func (c *Cluster) Config(machine.Type) (string, error) {
	return "", nil
}

func (c *Cluster) Network() config.ClusterNetwork {
	return &ClusterNetwork{c.provider}
}

func (c *Cluster) LocalAPIServerPort() int {
	return 6443
}

func (c *Cluster) ExtraManifestURLs() []string {
	return nil
}

func (c *Cluster) ExtraManifestHeaderMap() map[string]string {
	return nil
}

func (c *Cluster) AdminKubeconfig() config.AdminKubeconfig {
	return &AdminKubeconfig{c.provider}
}

// Etcd

type Etcd struct {
	provider *Provider
}

func (a *Etcd) Image() string {
	return ""
}

func (a *Etcd) ExtraArgs() map[string]string {
	return nil
}

func (a *Etcd) CA() *x509.PEMEncodedCertificateAndKey {
	return nil
}

// API Server

type APIServer struct {
	provider *Provider
}

func (a *APIServer) Image() string {
	return ""
}

func (a *APIServer) ExtraArgs() map[string]string {
	return nil
}

// Controller Manager

type ControllerManager struct {
	provider *Provider
}

func (c *ControllerManager) Image() string {
	return ""
}

func (c *ControllerManager) ExtraArgs() map[string]string {
	return nil
}

// Scheduler

type Scheduler struct {
	provider *Provider
}

func (c *Scheduler) Image() string {
	return ""
}

func (c *Scheduler) ExtraArgs() map[string]string {
	return nil
}

// Proxy

type Proxy struct {
	provider *Provider
}

func (c *Proxy) Image() string {
	return ""
}

func (c *Proxy) ExtraArgs() map[string]string {
	return nil
}

func (c *Proxy) Mode() string {
	return ""
}

// CoreDNS

type CoreDNS struct {
	provider *Provider
}

func (c *CoreDNS) Image() string {
	return ""
}

// PodCheckpointer

type PodCheckpointer struct {
	provider *Provider
}

func (c *PodCheckpointer) Image() string {
	return ""
}

// Cluster Network

type ClusterNetwork struct {
	provider *Provider
}

func (n *ClusterNetwork) CNI() config.CNI {
	return &CNI{n.provider}
}

func (n *ClusterNetwork) PodCIDR() string {
	return ""
}

func (n *ClusterNetwork) ServiceCIDR() string {
	return ""
}

func (n *ClusterNetwork) DNSDomain() string {
	return ""
}

type CNI struct {
	provider *Provider
}

func (c *CNI) Name() string {
	return ""
}

func (c *CNI) URLs() []string {
	return nil
}

type Token struct {
	provider *Provider
}

func (t *Token) ID() string {
	return ""
}

func (t *Token) Secret() string {
	return ""
}

type AdminKubeconfig struct {
	provider *Provider
}

func (a *AdminKubeconfig) CertLifetime() time.Duration {
	return time.Second
}
