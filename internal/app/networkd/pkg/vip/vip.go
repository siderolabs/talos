// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vip

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/plunder-app/kube-vip/pkg/vip"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const campaignRetryInterval = time.Second

// A Controller provides a control interface for Virtual IP addressing.
type Controller interface {
	// Start activates the Virtual IP address controller.
	Start(ctx context.Context, logger *log.Logger, eg *errgroup.Group) error
}

type vipController struct {
	ip    net.IP
	iface *net.Interface
}

// New creates a new Virtual IP controller.
func New(ip, iface string) (Controller, error) {
	ipaddr := net.ParseIP(ip)
	if ipaddr == nil {
		return nil, fmt.Errorf("failed to parse ip %q as an IP address", ip)
	}

	netIf, err := net.InterfaceByName(iface)
	if err != nil || netIf == nil {
		return nil, fmt.Errorf("failed to find interface %s by name: %w", iface, err)
	}

	return &vipController{
		ip:    ipaddr,
		iface: netIf,
	}, nil
}

// Start implements the Controller interface.
func (c *vipController) Start(ctx context.Context, logger *log.Logger, eg *errgroup.Group) error {
	netController, err := vip.NewConfig(c.ip.String(), c.iface.Name, false)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		c.maintain(ctx, logger, netController)

		return nil
	})

	return nil
}

func (c *vipController) etcdElectionKey() string {
	return fmt.Sprintf("%s:vip:election:%s", constants.EtcdRootTalosKey, c.ip.String())
}

func (c *vipController) maintain(ctx context.Context, logger *log.Logger, netController vip.Network) {
	for ctx.Err() == nil {
		if err := c.campaign(ctx, logger, netController); err != nil {
			logger.Printf("campaign failure: %s", err)

			time.Sleep(campaignRetryInterval)

			continue
		}
	}
}

//nolint:gocyclo,cyclop
func (c *vipController) campaign(ctx context.Context, logger *log.Logger, netController vip.Network) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("refusing to join election without a hostname")
	}

	ec, err := etcd.NewLocalClient()
	if err != nil {
		return fmt.Errorf("failed to create local etcd client: %w", err)
	}

	defer ec.Close() //nolint:errcheck

	sess, err := concurrency.NewSession(ec.Client)
	if err != nil {
		return fmt.Errorf("failed to create concurrency session: %w", err)
	}
	defer sess.Close() //nolint:errcheck

	election := concurrency.NewElection(sess, c.etcdElectionKey())

	node, err := election.Leader(ctx)
	if err != nil {
		if err != concurrency.ErrElectionNoLeader {
			return fmt.Errorf("failed getting current leader: %w", err)
		}
	} else if string(node.Kvs[0].Value) == hostname {
		logger.Printf("vip: resigning from previous election")

		// we are still leader from the previous election, attempt to resign to force new election
		resumedElection := concurrency.ResumeElection(sess, c.etcdElectionKey(), string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)

		if err = resumedElection.Resign(ctx); err != nil {
			return fmt.Errorf("failed resigning from previous elections: %w", err)
		}
	}

	campaignErrCh := make(chan error)

	go func() {
		campaignErrCh <- election.Campaign(ctx, hostname)
	}()

	select {
	case err = <-campaignErrCh:
		if err != nil {
			return fmt.Errorf("failed to conduct campaign: %w", err)
		}
	case <-sess.Done():
		logger.Printf("vip: session closed")
	}

	defer func() {
		// use a new context to resign, as `ctx` might be canceled
		resignCtx, resignCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer resignCancel()

		election.Resign(resignCtx) //nolint:errcheck
	}()

	if err = netController.AddIP(); err != nil {
		return fmt.Errorf("failed to add VIP %q to local interface %q: %w", c.ip.String(), c.iface.Name, err)
	}

	defer func() {
		logger.Printf("vip: removing shared IP %q on interface %q", c.ip.String(), c.iface.Name)

		if err = netController.DeleteIP(); err != nil {
			logger.Printf("vip: error removing shared IP: %s", err)
		}
	}()

	// ARP is only supported for IPv4
	if c.ip.To4() != nil {
		// Send gratuitous ARP to announce the change
		if err = vip.ARPSendGratuitous(c.ip.String(), c.iface.Name); err != nil {
			return fmt.Errorf("failed to send gratuitous ARP after winning election: %w", err)
		}
	}

	logger.Printf("vip: enabled shared IP %q on interface %q", c.ip.String(), c.iface.Name)

	observe := election.Observe(ctx)

observeLoop:
	for {
		select {
		case <-sess.Done():
			logger.Printf("vip: session closed")

			break observeLoop
		case <-ctx.Done():
			break observeLoop
		case resp, ok := <-observe:
			if !ok {
				break observeLoop
			}

			if string(resp.Kvs[0].Value) != hostname {
				logger.Printf("vip: detected new leader %q", string(resp.Kvs[0].Value))

				break observeLoop
			}
		}
	}

	return nil
}
