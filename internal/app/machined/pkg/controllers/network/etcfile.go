// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	efiles "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EtcFileController creates /etc/hostname and /etc/resolv.conf files based on finalized network configuration.
type EtcFileController struct {
	PodResolvConfPath string
	V1Alpha1Mode      runtime.Mode
}

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
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverStatusType,
			ID:        optional.Some(network.ResolverID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.NodeAddressDefaultID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostDNSConfigType,
			ID:        optional.Some(network.HostDNSConfigID),
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
//nolint:gocyclo,cyclop
func (ctrl *EtcFileController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var cfgProvider talosconfig.Config

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.Config()
		}

		hostnameStatus, err := safe.ReaderGetByID[*network.HostnameStatus](ctx, r, network.HostnameID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting hostname status: %w", err)
			}
		}

		nodeAddressStatus, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.NodeAddressDefaultID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting network address status: %w", err)
			}
		}

		resolverStatus, err := safe.ReaderGetByID[*network.ResolverStatus](ctx, r, network.ResolverID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error resolver status: %w", err)
			}
		}

		hostDNSCfg, err := safe.ReaderGetByID[*network.HostDNSConfig](ctx, r, network.HostDNSConfigID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting host dns config: %w", err)
			}
		}

		var hostnameStatusSpec *network.HostnameStatusSpec
		if hostnameStatus != nil {
			hostnameStatusSpec = hostnameStatus.TypedSpec()
		}

		if resolverStatus != nil && hostDNSCfg != nil && !ctrl.V1Alpha1Mode.InContainer() {
			// in container mode, keep the original resolv.conf to use the resolvers supplied by the container runtime
			if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "resolv.conf"),
				func(r *files.EtcFileSpec) error {
					r.TypedSpec().Contents = renderResolvConf(pickNameservers(hostDNSCfg, resolverStatus), hostnameStatusSpec, cfgProvider)
					r.TypedSpec().Mode = 0o644

					return nil
				}); err != nil {
				return fmt.Errorf("error modifying resolv.conf: %w", err)
			}
		}

		if resolverStatus != nil && hostDNSCfg != nil {
			dnsServers := xslices.FilterInPlace(
				[]netip.Addr{hostDNSCfg.TypedSpec().ServiceHostDNSAddress},
				netip.Addr.IsValid,
			)

			if len(dnsServers) == 0 {
				dnsServers = resolverStatus.TypedSpec().DNSServers
			}

			conf := renderResolvConf(slices.All(dnsServers), hostnameStatusSpec, cfgProvider)

			if err = os.MkdirAll(filepath.Dir(ctrl.PodResolvConfPath), 0o755); err != nil {
				return fmt.Errorf("error creating pod resolv.conf dir: %w", err)
			}

			err = efiles.UpdateFile(ctrl.PodResolvConfPath, conf, 0o644)
			if err != nil {
				return fmt.Errorf("error writing pod resolv.conf: %w", err)
			}
		}

		if hostnameStatus != nil && nodeAddressStatus != nil {
			if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "hosts"),
				func(r *files.EtcFileSpec) error {
					r.TypedSpec().Contents, err = ctrl.renderHosts(hostnameStatus.TypedSpec(), nodeAddressStatus.TypedSpec(), cfgProvider)
					r.TypedSpec().Mode = 0o644

					return err
				}); err != nil {
				return fmt.Errorf("error modifying hosts: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

var localDNS = xiter.Single2(0, netip.MustParseAddr("127.0.0.53"))

func pickNameservers(hostDNSCfg *network.HostDNSConfig, resolverStatus *network.ResolverStatus) iter.Seq2[int, netip.Addr] {
	if hostDNSCfg.TypedSpec().Enabled {
		// local dns resolve cache enabled, route host dns requests to 127.0.0.1
		return localDNS
	}

	return slices.All(resolverStatus.TypedSpec().DNSServers)
}

func renderResolvConf(nameservers iter.Seq2[int, netip.Addr], hostnameStatus *network.HostnameStatusSpec, cfgProvider talosconfig.Config) []byte {
	var buf bytes.Buffer

	for i, ns := range nameservers {
		if i >= 3 {
			// only use first 3 nameservers, see MAXNS in https://linux.die.net/man/5/resolv.conf
			break
		}

		fmt.Fprintf(&buf, "nameserver %s\n", ns)
	}

	var disableSearchDomain bool
	if cfgProvider != nil && cfgProvider.Machine() != nil {
		disableSearchDomain = cfgProvider.Machine().Network().DisableSearchDomain()
	}

	if !disableSearchDomain && hostnameStatus != nil && hostnameStatus.Domainname != "" {
		fmt.Fprintf(&buf, "\nsearch %s\n", hostnameStatus.Domainname)
	}

	return buf.Bytes()
}

func (ctrl *EtcFileController) renderHosts(hostnameStatus *network.HostnameStatusSpec, nodeAddressStatus *network.NodeAddressSpec, cfgProvider talosconfig.Config) ([]byte, error) {
	var buf bytes.Buffer

	tabW := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)

	write := func(s string) { tabW.Write([]byte(s)) } //nolint:errcheck

	write("127.0.0.1\tlocalhost\n")

	write(fmt.Sprintf("%s\t%s", nodeAddressStatus.Addresses[0].Addr(), hostnameStatus.FQDN()))

	if hostnameStatus.Hostname != hostnameStatus.FQDN() {
		write(" " + hostnameStatus.Hostname)
	}

	write("\n")

	write("::1\tlocalhost ip6-localhost ip6-loopback\n")
	write("ff02::1\tip6-allnodes\n")
	write("ff02::2\tip6-allrouters\n")

	if cfgProvider != nil && cfgProvider.Machine() != nil {
		for _, extraHost := range cfgProvider.Machine().Network().ExtraHosts() {
			write(fmt.Sprintf("%s\t%s\n", extraHost.IP(), strings.Join(extraHost.Aliases(), " ")))
		}
	}

	if err := tabW.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
