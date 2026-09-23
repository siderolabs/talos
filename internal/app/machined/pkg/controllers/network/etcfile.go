// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"iter"
	"net/netip"
	"os"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	efiles "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/etcrender"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/xfs"
)

// EtcFileController creates /etc/hostname and /etc/resolv.conf files based on finalized network configuration.
type EtcFileController struct {
	V1Alpha1Mode runtime.Mode

	EtcRoot          xfs.Root
	BindMountTarget  string
	bindMountCreated bool
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
			ID:        optional.Some(config.ActiveID),
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
func (ctrl *EtcFileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var cfgProvider talosconfig.Config

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
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

		if resolverStatus != nil && hostDNSCfg != nil && !ctrl.V1Alpha1Mode.InContainer() {
			// in container mode, keep the original resolv.conf to use the resolvers supplied by the container runtime
			if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "resolv.conf"),
				func(r *files.EtcFileSpec) error {
					r.TypedSpec().Contents = etcrender.ResolvConf(
						pickNameservers(hostDNSCfg, resolverStatus),
						resolverStatus.TypedSpec().SearchDomains,
					)
					r.TypedSpec().Mode = 0o644
					r.TypedSpec().SelinuxLabel = constants.EtcSelinuxLabel

					return nil
				}); err != nil {
				return fmt.Errorf("error modifying resolv.conf: %w", err)
			}
		}

		if resolverStatus != nil && hostDNSCfg != nil {
			dnsServers := xslices.FilterInPlace(
				[]netip.Addr{hostDNSCfg.TypedSpec().ServiceHostDNSAddress, hostDNSCfg.TypedSpec().ServiceHostDNSAddressV6},
				netip.Addr.IsValid,
			)

			if len(dnsServers) == 0 {
				dnsServers = xslices.Map(
					xslices.Filter(
						resolverStatus.TypedSpec().NameServers,
						func(ns network.NameServerSpec) bool {
							// without HostDNS support only plain DNS protocol
							return ns.Protocol == nethelpers.DNSProtocolDefault
						},
					),
					func(ns network.NameServerSpec) netip.Addr { return ns.Addr },
				)
			}

			src := "resolv.conf"
			dst := filepath.Join(ctrl.BindMountTarget, src)

			conf := etcrender.ResolvConf(slices.All(dnsServers), resolverStatus.TypedSpec().SearchDomains)

			if err := efiles.UpdateFile(ctrl.EtcRoot, src, conf, 0o644, constants.EtcSelinuxLabel); err != nil {
				return fmt.Errorf("error writing pod resolv.conf: %w", err)
			}

			if ctrl.EtcRoot.FSType() != "os" {
				if !ctrl.bindMountCreated {
					if err := createBindMountFileFd(ctrl.EtcRoot, src, dst, 0o644); err != nil {
						return fmt.Errorf("failed to create shadow bind mount %q -> %q: %w", src, dst, err)
					}

					ctrl.bindMountCreated = true
				}
			}
		}

		if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "hosts"),
			func(r *files.EtcFileSpec) error {
				r.TypedSpec().Contents, err = etcrender.Hosts(hostnameStatus, nodeAddressStatus, cfgProvider)
				r.TypedSpec().Mode = 0o644
				r.TypedSpec().SelinuxLabel = constants.EtcSelinuxLabel

				return err
			}); err != nil {
			return fmt.Errorf("error modifying hosts: %w", err)
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

	return slices.All(
		xslices.Map(
			xslices.Filter(
				resolverStatus.TypedSpec().NameServers,
				func(ns network.NameServerSpec) bool {
					return ns.Protocol == nethelpers.DNSProtocolDefault
				},
			),
			func(ns network.NameServerSpec) netip.Addr {
				return ns.Addr
			},
		),
	)
}

// createBindMountFileFd creates a common way to create a writable source file with a
// bind mounted destination.
func createBindMountFileFd(root xfs.Root, src, dst string, mode os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("error creating bind mount dir for resolv.conf: %w", err)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("error creating bind mount target %q for resolv.conf: %w", dst, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("error closing bind mount target %q for resolv.conf: %w", dst, err)
	}

	if err = xfs.MkdirAll(root, filepath.Dir(src), 0o755); err != nil {
		return err
	}

	fsrc, err := xfs.OpenFile(root, src, os.O_WRONLY|os.O_CREATE, mode)
	if err != nil {
		return err
	}
	defer fsrc.Close() //nolint:errcheck

	return mount.BindReadonlyFd(int(fsrc.Fd()), dst)
}
