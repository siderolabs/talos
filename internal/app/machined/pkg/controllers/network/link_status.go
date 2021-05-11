// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/ethtool"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/network/nethelpers"
)

// LinkStatusController manages secrets.Etcd based on configuration.
type LinkStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *LinkStatusController) Name() string {
	return "network.LinkStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LinkStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *LinkStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.LinkStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LinkStatusController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	// create watch connections to rtnetlink and ethtool via genetlink
	// these connections are used only to join multicast groups and receive notifications on changes
	// other connections are used to send requests and receive responses, as we can't mix the notifications and request/responses
	watchCh := make(chan struct{})

	rtnetlinkWatcher, err := watch.NewRtNetlink(ctx, watchCh, unix.RTMGRP_LINK)
	if err != nil {
		return err
	}

	defer rtnetlinkWatcher.Done()

	ethtoolWatcher, err := watch.NewEthtool(ctx, watchCh)
	if err != nil {
		return err
	}

	defer ethtoolWatcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	ethClient, err := ethtool.New()
	if err != nil {
		return fmt.Errorf("error dialing ethtool socket: %w", err)
	}

	defer ethClient.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-watchCh:
		}

		if err = ctrl.reconcile(ctx, r, conn, ethClient); err != nil {
			return err
		}
	}
}

// reconcile function runs for every reconciliation loop querying the netlink state and updating resources.
//
//nolint:gocyclo,cyclop
func (ctrl *LinkStatusController) reconcile(ctx context.Context, r controller.Runtime, conn *rtnetlink.Conn, ethClient *ethtool.Client) error {
	// list the existing LinkStatus resources and mark them all to be deleted, as the actual link is discovered via netlink, resource ID is removed from the list
	list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	itemsToDelete := map[resource.ID]struct{}{}

	for _, r := range list.Items {
		itemsToDelete[r.Metadata().ID()] = struct{}{}
	}

	links, err := conn.Link.List()
	if err != nil {
		return fmt.Errorf("error listing links: %w", err)
	}

	// for every rtnetlink discovered link
	for _, link := range links {
		link := link

		var (
			ethState *ethtool.LinkState
			ethInfo  *ethtool.LinkInfo
			ethMode  *ethtool.LinkMode
		)

		// query additional information via ethtool (if supported)
		ethState, err = ethClient.LinkState(ethtool.Interface{
			Index: int(link.Index),
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error querying ethtool link state for %q: %w", link.Attributes.Name, err)
		}

		// skip if previous call failed (e.g. not supported)
		if err == nil {
			ethInfo, err = ethClient.LinkInfo(ethtool.Interface{
				Index: int(link.Index),
			})
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error querying ethtool link info for %q: %w", link.Attributes.Name, err)
			}
		}

		// skip if previous call failed (e.g. not supported)
		if err == nil {
			ethMode, err = ethClient.LinkMode(ethtool.Interface{
				Index: int(link.Index),
			})
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error querying ethtool link mode for %q: %w", link.Attributes.Name, err)
			}
		}

		if err = r.Modify(ctx, network.NewLinkStatus(network.NamespaceName, link.Attributes.Name), func(r resource.Resource) error {
			status := r.(*network.LinkStatus).Status()

			status.Index = link.Index
			status.HardwareAddr = nethelpers.HardwareAddr(link.Attributes.Address)
			status.BroadcastAddr = nethelpers.HardwareAddr(link.Attributes.Broadcast)
			status.LinkIndex = link.Attributes.Type
			status.Flags = nethelpers.LinkFlags(link.Flags)
			status.Type = nethelpers.LinkType(link.Type)
			status.QueueDisc = link.Attributes.QueueDisc
			status.MTU = link.Attributes.MTU
			if link.Attributes.Master != nil {
				status.MasterIndex = *link.Attributes.Master
			} else {
				status.MasterIndex = 0
			}
			status.OperationalState = nethelpers.OperationalState(link.Attributes.OperationalState)
			if link.Attributes.Info != nil {
				status.Kind = link.Attributes.Info.Kind
				status.SlaveKind = link.Attributes.Info.SlaveKind
			} else {
				status.Kind = ""
				status.SlaveKind = ""
			}

			if ethState != nil {
				status.LinkState = ethState.Link
			} else {
				status.LinkState = false
			}

			if ethInfo != nil {
				status.Port = nethelpers.Port(ethInfo.Port)
			} else {
				status.Port = nethelpers.Port(ethtool.Other)
			}

			if ethMode != nil {
				status.SpeedMegabits = ethMode.SpeedMegabits
				status.Duplex = nethelpers.Duplex(ethMode.Duplex)
			} else {
				status.SpeedMegabits = 0
				status.Duplex = nethelpers.Duplex(ethtool.Unknown)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		delete(itemsToDelete, link.Attributes.Name)
	}

	for id := range itemsToDelete {
		if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, id, resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error deleting link status %q: %w", id, err)
		}
	}

	return nil
}
