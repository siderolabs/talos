// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// EtcFileController creates /etc/hostname and /etc/resolv.conf files based on finalized network configuration.
type EtcFileController struct{}

// Name implements controller.Controller interface.
func (ctrl *EtcFileController) Name() string {
	return "network.EtcFileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcFileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.To(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverStatusType,
			ID:        pointer.To(network.ResolverID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.To(network.NodeAddressDefaultID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcFileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EtcFileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var cfgProvider talosconfig.Provider

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.(*config.MachineConfig).Config()
		}

		var resolverStatus *network.ResolverStatusSpec

		rStatus, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.ResolverStatusType, network.ResolverID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting resolver status: %w", err)
			}
		} else {
			resolverStatus = rStatus.(*network.ResolverStatus).TypedSpec()
		}

		var hostnameStatus *network.HostnameStatusSpec

		hStatus, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting hostname status: %w", err)
			}
		} else {
			hostnameStatus = hStatus.(*network.HostnameStatus).TypedSpec()
		}

		var nodeAddressStatus *network.NodeAddressSpec

		naStatus, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressDefaultID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting network address status: %w", err)
			}
		} else {
			nodeAddressStatus = naStatus.(*network.NodeAddress).TypedSpec()
		}

		if resolverStatus != nil {
			if err = r.Modify(ctx, files.NewEtcFileSpec(files.NamespaceName, "resolv.conf"),
				func(r resource.Resource) error {
					r.(*files.EtcFileSpec).TypedSpec().Contents = ctrl.renderResolvConf(resolverStatus, hostnameStatus, cfgProvider)
					r.(*files.EtcFileSpec).TypedSpec().Mode = 0o644

					return nil
				}); err != nil {
				return fmt.Errorf("error modifying resolv.conf: %w", err)
			}
		}

		if hostnameStatus != nil && nodeAddressStatus != nil {
			if err = r.Modify(ctx, files.NewEtcFileSpec(files.NamespaceName, "hosts"),
				func(r resource.Resource) error {
					r.(*files.EtcFileSpec).TypedSpec().Contents, err = ctrl.renderHosts(hostnameStatus, nodeAddressStatus, cfgProvider)
					r.(*files.EtcFileSpec).TypedSpec().Mode = 0o644

					return err
				}); err != nil {
				return fmt.Errorf("error modifying resolv.conf: %w", err)
			}
		}
	}
}

func (ctrl *EtcFileController) renderResolvConf(resolverStatus *network.ResolverStatusSpec, hostnameStatus *network.HostnameStatusSpec, cfgProvider talosconfig.Provider) []byte {
	var buf bytes.Buffer

	for i, resolver := range resolverStatus.DNSServers {
		if i >= 3 {
			// only use firt 3 nameservers, see MAXNS in https://linux.die.net/man/5/resolv.conf
			break
		}

		fmt.Fprintf(&buf, "nameserver %s\n", resolver)
	}

	var disableSearchDomain bool
	if cfgProvider != nil {
		disableSearchDomain = cfgProvider.Machine().Network().DisableSearchDomain()
	}

	if !disableSearchDomain && hostnameStatus != nil && hostnameStatus.Domainname != "" {
		fmt.Fprintf(&buf, "\nsearch %s\n", hostnameStatus.Domainname)
	}

	return buf.Bytes()
}

var hostsTemplate = template.Must(template.New("hosts").Parse(strings.TrimSpace(`
127.0.0.1       localhost
{{ .IP }}       {{ .Hostname }} {{ if ne .Hostname .Alias }}{{ .Alias }}{{ end }}
::1             localhost ip6-localhost ip6-loopback
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters

{{- with .ExtraHosts }}
{{ range . }}
{{ .IP }} {{ range .Aliases }}{{.}} {{ end -}}
{{ end -}}
{{ end -}}
`)))

func (ctrl *EtcFileController) renderHosts(hostnameStatus *network.HostnameStatusSpec, nodeAddressStatus *network.NodeAddressSpec, cfgProvider talosconfig.Provider) ([]byte, error) {
	var buf bytes.Buffer

	extraHosts := []talosconfig.ExtraHost{}

	if cfgProvider != nil {
		extraHosts = cfgProvider.Machine().Network().ExtraHosts()
	}

	data := struct {
		IP         string
		Hostname   string
		Alias      string
		ExtraHosts []talosconfig.ExtraHost
	}{
		IP:         nodeAddressStatus.Addresses[0].IP().String(),
		Hostname:   hostnameStatus.FQDN(),
		Alias:      hostnameStatus.Hostname,
		ExtraHosts: extraHosts,
	}

	if err := hostsTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
