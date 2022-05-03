// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/ordered"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// LinkSpecController applies network.LinkSpec to the actual interfaces.
type LinkSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *LinkSpecController) Name() string {
	return "network.LinkSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LinkSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LinkSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.LinkRefreshType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LinkSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(r, unix.RTMGRP_LINK)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	wgClient, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("error creating wireguard client: %w", err)
	}

	defer wgClient.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// list rtnetlink links (interfaces)
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// loop over links and make reconcile decision
		var multiErr *multierror.Error

		SortBonds(list.Items)

		for _, res := range list.Items {
			link := res.(*network.LinkSpec) //nolint:forcetypeassert,errcheck

			if err = ctrl.syncLink(ctx, r, logger, conn, wgClient, &links, link); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		}

		if err = multiErr.ErrorOrNil(); err != nil {
			return err
		}
	}
}

// SortBonds sort resources in increasing order, except it places slave interfaces right after the bond
// in proper order.
func SortBonds(items []resource.Resource) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Spec().(network.LinkSpecSpec)  //nolint:errcheck
		right := items[j].Spec().(network.LinkSpecSpec) //nolint:errcheck

		l := ordered.MakeTriple(left.Name, 0, "")
		if left.BondSlave.MasterName != "" {
			l = ordered.MakeTriple(left.BondSlave.MasterName, left.BondSlave.SlaveIndex, left.Name)
		}

		r := ordered.MakeTriple(right.Name, 0, "")
		if right.BondSlave.MasterName != "" {
			r = ordered.MakeTriple(right.BondSlave.MasterName, right.BondSlave.SlaveIndex, right.Name)
		}

		return l.LessThan(r)
	})
}

func findLink(links []rtnetlink.LinkMessage, name string) *rtnetlink.LinkMessage {
	for i, link := range links {
		if link.Attributes.Name == name {
			return &links[i]
		}
	}

	return nil
}

// syncLink syncs kernel state with the LinkSpec link.
//
// This method is really long, but it's hard to break it down in multiple pieces, are those pieces and steps are inter-dependent, so, instead,
// I'm going to provide high-level flow of the method here to help understand it:
//
// First of all, if the spec is being torn down - remove the link from the kernel, done.
// If the link spec is not being torn down, start the sync process:
//
//  * for physical links, there's not much we can sync - only MTU and 'UP' flag
//  * for logical links, controller handles creation and sync of the settings depending on the interface type
//
// If the logical link kind or type got changed (for example, "link0" was a bond, and now it's wireguard interface), the link
// is dropped and replaced with the new one.
// Same replace flow is used for VLAN links, as VLAN settings can't be changed on the fly.
//
// For bonded links, there are two sync steps applied:
//
//  * bond slave interfaces are enslaved to be part of the bond (by changing MasterIndex)
//  * bond master link settings are synced with the spec: some settings can't be applied on UP bond and a bond which has slaves,
//    so slaves are removed and bond is brought down (these settings are going to be reconciled back in the next sync cycle)
//
// For wireguard links, only settings are synced with the diff generated by the WireguardSpec.
//
//nolint:gocyclo,cyclop
func (ctrl *LinkSpecController) syncLink(ctx context.Context, r controller.Runtime, logger *zap.Logger, conn *rtnetlink.Conn, wgClient *wgctrl.Client,
	links *[]rtnetlink.LinkMessage, link *network.LinkSpec,
) error {
	logger = logger.With(zap.String("link", link.TypedSpec().Name))

	switch link.Metadata().Phase() {
	case resource.PhaseTearingDown:
		// TODO: should we bring link down if it's physical and the spec was torn down?
		if link.TypedSpec().Logical {
			existing := findLink(*links, link.TypedSpec().Name)

			if existing != nil {
				if err := conn.Link.Delete(existing.Index); err != nil {
					return fmt.Errorf("error deleting link %q: %w", link.TypedSpec().Name, err)
				}

				logger.Info("deleted link")

				// refresh links as the link list got changed
				var err error

				*links, err = conn.Link.List()
				if err != nil {
					return fmt.Errorf("error listing links: %w", err)
				}
			}
		}

		// now remove finalizer as link was deleted
		if err := r.RemoveFinalizer(ctx, link.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}
	case resource.PhaseRunning:
		existing := findLink(*links, link.TypedSpec().Name)

		// check if type/kind matches for the existing logical link
		if existing != nil && link.TypedSpec().Logical {
			replace := false

			// if type/kind doesn't match, recreate the link to change it
			if existing.Type != uint16(link.TypedSpec().Type) || existing.Attributes.Info.Kind != link.TypedSpec().Kind {
				logger.Info("replacing logical link",
					zap.String("old_kind", existing.Attributes.Info.Kind),
					zap.String("new_kind", link.TypedSpec().Kind),
					zap.Stringer("old_type", nethelpers.LinkType(existing.Type)),
					zap.Stringer("new_type", link.TypedSpec().Type),
				)

				replace = true
			}

			// sync VLAN spec, as it can't be modified on the fly
			if !replace && link.TypedSpec().Kind == network.LinkKindVLAN {
				var existingVLAN network.VLANSpec

				if err := networkadapter.VLANSpec(&existingVLAN).Decode(existing.Attributes.Info.Data); err != nil {
					return fmt.Errorf("error decoding VLAN properties on %q: %w", link.TypedSpec().Name, err)
				}

				if existingVLAN != link.TypedSpec().VLAN {
					logger.Info("replacing VLAN link",
						zap.Uint16("old_id", existingVLAN.VID),
						zap.Uint16("new_id", link.TypedSpec().VLAN.VID),
						zap.Stringer("old_protocol", existingVLAN.Protocol),
						zap.Stringer("new_protocol", link.TypedSpec().VLAN.Protocol),
					)

					replace = true
				}
			}

			if replace {
				if err := conn.Link.Delete(existing.Index); err != nil {
					return fmt.Errorf("error deleting link %q: %w", link.TypedSpec().Name, err)
				}

				// not refreshing links, as the link is set to be re-created

				existing = nil
			}
		}

		if existing == nil {
			if !link.TypedSpec().Logical {
				// physical interface doesn't exist yet, nothing to be done
				return nil
			}

			// create logical interface
			var (
				parentIndex uint32
				data        []byte
				err         error
			)

			// VLAN settings should be set on interface creation (parent + VLAN settings)
			if link.TypedSpec().ParentName != "" {
				parent := findLink(*links, link.TypedSpec().ParentName)
				if parent == nil {
					// parent doesn't exist yet, skip it
					return nil
				}

				parentIndex = parent.Index
			}

			if link.TypedSpec().Kind == network.LinkKindVLAN {
				data, err = networkadapter.VLANSpec(&link.TypedSpec().VLAN).Encode()
				if err != nil {
					return fmt.Errorf("error encoding VLAN attributes for link %q: %w", link.TypedSpec().Name, err)
				}
			}

			if err = conn.Link.New(&rtnetlink.LinkMessage{
				Type: uint16(link.TypedSpec().Type),
				Attributes: &rtnetlink.LinkAttributes{
					Name: link.TypedSpec().Name,
					Type: parentIndex,
					Info: &rtnetlink.LinkInfo{
						Kind: link.TypedSpec().Kind,
						Data: data,
					},
				},
			}); err != nil {
				return fmt.Errorf("error creating logical link %q: %w", link.TypedSpec().Name, err)
			}

			logger.Info("created new link", zap.String("kind", link.TypedSpec().Kind))

			// refresh links as the link list got changed
			*links, err = conn.Link.List()
			if err != nil {
				return fmt.Errorf("error listing links: %w", err)
			}

			existing = findLink(*links, link.TypedSpec().Name)
			if existing == nil {
				return fmt.Errorf("created link %q not found in the link list", link.TypedSpec().Name)
			}
		}

		// sync bond settings
		if link.TypedSpec().Kind == network.LinkKindBond {
			var existingBond network.BondMasterSpec

			if err := networkadapter.BondMasterSpec(&existingBond).Decode(existing.Attributes.Info.Data); err != nil {
				return fmt.Errorf("error parsing bond attributes for %q: %w", link.TypedSpec().Name, err)
			}

			if existingBond != link.TypedSpec().BondMaster {
				logger.Debug("updating bond settings",
					zap.String("old", fmt.Sprintf("%+v", existingBond)),
					zap.String("new", fmt.Sprintf("%+v", link.TypedSpec().BondMaster)),
				)

				data, err := networkadapter.BondMasterSpec(&link.TypedSpec().BondMaster).Encode()
				if err != nil {
					return fmt.Errorf("error encoding bond attributes for %q: %w", link.TypedSpec().Name, err)
				}

				// bring bond down
				if err = conn.Link.Set(&rtnetlink.LinkMessage{
					Family: existing.Family,
					Type:   existing.Type,
					Index:  existing.Index,
					Flags:  0,
					Change: unix.IFF_UP,
				}); err != nil {
					return fmt.Errorf("error changing flags for %q: %w", link.TypedSpec().Name, err)
				}

				// unslave all slaves
				for i, slave := range *links {
					if slave.Attributes.Master != nil && *slave.Attributes.Master == existing.Index {
						if err = conn.Link.Set(&rtnetlink.LinkMessage{
							Family: slave.Family,
							Type:   slave.Type,
							Index:  slave.Index,
							Attributes: &rtnetlink.LinkAttributes{
								Master: pointer.To[uint32](0),
							},
						}); err != nil {
							return fmt.Errorf("error unslaving link %q under %q: %w", slave.Attributes.Name, link.TypedSpec().BondSlave.MasterName, err)
						}

						(*links)[i].Attributes.Master = nil
					}
				}

				// update settings
				if err = conn.Link.Set(&rtnetlink.LinkMessage{
					Family: existing.Family,
					Type:   existing.Type,
					Index:  existing.Index,
					Attributes: &rtnetlink.LinkAttributes{
						Info: &rtnetlink.LinkInfo{
							Kind: existing.Attributes.Info.Kind,
							Data: data,
						},
					},
				}); err != nil {
					return fmt.Errorf("error updating bond settings for %q: %w", link.TypedSpec().Name, err)
				}

				logger.Info("updated bond settings")
			}
		}

		// sync wireguard settings
		if link.TypedSpec().Kind == network.LinkKindWireguard {
			wgDev, err := wgClient.Device(link.TypedSpec().Name)
			if err != nil {
				return fmt.Errorf("error getting wireguard settings for %q: %w", link.TypedSpec().Name, err)
			}

			var existingSpec network.WireguardSpec

			networkadapter.WireguardSpec(&existingSpec).Decode(wgDev, false)
			existingSpec.Sort()

			link.TypedSpec().Wireguard.Sort()

			// order here is important: we allow listenPort to be zero in the configuration
			if !existingSpec.Equal(&link.TypedSpec().Wireguard) {
				config, err := networkadapter.WireguardSpec(&link.TypedSpec().Wireguard).Encode(&existingSpec)
				if err != nil {
					return fmt.Errorf("error creating wireguard config patch for %q: %w", link.TypedSpec().Name, err)
				}

				if err = wgClient.ConfigureDevice(link.TypedSpec().Name, *config); err != nil {
					return fmt.Errorf("error configuring wireguard device %q: %w", link.TypedSpec().Name, err)
				}

				logger.Info("reconfigured wireguard link", zap.Int("peers", len(link.TypedSpec().Wireguard.Peers)))

				// notify link status controller, as wireguard updates can't be watched via netlink API
				if err = r.Modify(ctx, network.NewLinkRefresh(network.NamespaceName, network.LinkKindWireguard), func(r resource.Resource) error {
					r.(*network.LinkRefresh).TypedSpec().Bump()

					return nil
				}); err != nil {
					return fmt.Errorf("error bumping link refresh")
				}
			}
		}

		// sync UP flag
		existingUp := existing.Flags&unix.IFF_UP == unix.IFF_UP
		if existingUp != link.TypedSpec().Up {
			flags := uint32(0)

			if link.TypedSpec().Up {
				flags = unix.IFF_UP
			}

			if err := conn.Link.Set(&rtnetlink.LinkMessage{
				Family: existing.Family,
				Type:   existing.Type,
				Index:  existing.Index,
				Flags:  flags,
				Change: unix.IFF_UP,
			}); err != nil {
				return fmt.Errorf("error changing flags for %q: %w", link.TypedSpec().Name, err)
			}

			logger.Debug("brought link up/down", zap.Bool("up", link.TypedSpec().Up))
		}

		// sync MTU if it's set in the spec
		if link.TypedSpec().MTU != 0 && existing.Attributes.MTU != link.TypedSpec().MTU {
			if err := conn.Link.Set(&rtnetlink.LinkMessage{
				Family: existing.Family,
				Type:   existing.Type,
				Index:  existing.Index,
				Attributes: &rtnetlink.LinkAttributes{
					MTU: link.TypedSpec().MTU,
				},
			}); err != nil {
				return fmt.Errorf("error setting MTU for %q: %w", link.TypedSpec().Name, err)
			}

			existing.Attributes.MTU = link.TypedSpec().MTU

			logger.Info("changed MTU for the link", zap.Uint32("mtu", link.TypedSpec().MTU))
		}

		// sync master index (for links which are bond slaves)
		var masterIndex uint32

		if link.TypedSpec().BondSlave.MasterName != "" {
			if master := findLink(*links, link.TypedSpec().BondSlave.MasterName); master != nil {
				masterIndex = master.Index
			}
		}

		if (existing.Attributes.Master == nil && masterIndex != 0) || (existing.Attributes.Master != nil && *existing.Attributes.Master != masterIndex) {
			if err := conn.Link.Set(&rtnetlink.LinkMessage{
				Family: existing.Family,
				Type:   existing.Type,
				Index:  existing.Index,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Master: pointer.To(masterIndex),
				},
			}); err != nil {
				return fmt.Errorf("error enslaving/unslaving link %q under %q: %w", link.TypedSpec().Name, link.TypedSpec().BondSlave.MasterName, err)
			}

			existing.Attributes.Master = pointer.To(masterIndex)

			logger.Info("enslaved/unslaved link", zap.String("parent", link.TypedSpec().BondSlave.MasterName))
		}
	}

	return nil
}
