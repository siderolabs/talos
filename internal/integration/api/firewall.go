// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-cleanhttp"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// FirewallSuite ...
type FirewallSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *FirewallSuite) SuiteName() string {
	return "api.FirewallSuite"
}

// SetupTest ...
func (suite *FirewallSuite) SetupTest() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state can't guarantee availability of kubelet IPs")
	}

	// make sure we abort at some point in time, but give enough room for Resets
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)
}

// TearDownTest ...
func (suite *FirewallSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestKubeletAccess verifies that without firewall kubelet API is available, and not available otherwise.
func (suite *FirewallSuite) TestKubeletAccess() {
	allNodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	_, err := safe.StateGetByID[*network.NfTablesChain](client.WithNode(suite.ctx, allNodes[0]), suite.Client.COSI, "ingress")
	firewallEnabled := err == nil

	eg, ctx := errgroup.WithContext(suite.ctx)

	transport := cleanhttp.DefaultTransport()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := http.Client{
		Transport: transport,
	}

	for _, node := range allNodes {
		eg.Go(func() error {
			attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(
				attemptCtx,
				http.MethodGet,
				fmt.Sprintf("https://%s/healthz", net.JoinHostPort(node, strconv.Itoa(constants.KubeletPort))),
				nil,
			)
			if err != nil {
				return err
			}

			resp, err := client.Do(req)

			if resp != nil {
				resp.Body.Close() //nolint:errcheck
			}

			if firewallEnabled {
				if err == nil {
					return errors.New("kubelet API should not be available")
				}

				if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("unexpected error: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("kubelet API should be available: %w", err)
			}

			return nil
		})
	}

	suite.Require().NoError(eg.Wait())
}

//go:embed testdata/nodeport.yaml
var nodePortServiceYAML []byte

// TestNodePortAccess verifies that without firewall NodePort is available, and not available otherwise.
//
//nolint:gocyclo
func (suite *FirewallSuite) TestNodePortAccess() {
	allNodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	chain, err := safe.StateGetByID[*network.NfTablesChain](client.WithNode(suite.ctx, allNodes[0]), suite.Client.COSI, "ingress")
	firewallEnabled := err == nil
	firewallDefaultBlock := firewallEnabled && chain.TypedSpec().Policy == nethelpers.VerdictDrop

	// our blocking only works with kube-proxy, so we need to make sure it's running
	out, err := suite.Clientset.CoreV1().Pods("kube-system").List(suite.ctx, metav1.ListOptions{LabelSelector: "k8s-app=kube-proxy"})
	suite.Require().NoError(err)

	if len(out.Items) == 0 {
		suite.T().Skip("kube-proxy not running")
	}

	// create a deployment with a NodePort service
	localPathStorage := suite.ParseManifests(nodePortServiceYAML)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, localPathStorage)
	})

	suite.ApplyManifests(suite.ctx, localPathStorage)

	// fetch the NodePort service
	// read back Service to figure out the ports
	svc, err := suite.Clientset.CoreV1().Services("default").Get(suite.ctx, "test-nginx", metav1.GetOptions{})
	suite.Require().NoError(err)

	var nodePort int

	for _, portSpec := range svc.Spec.Ports {
		nodePort = int(portSpec.NodePort)
	}

	suite.Require().NotZero(nodePort)

	suite.T().Log("sleeping for 5 seconds to allow kube-proxy to update nftables")

	time.Sleep(5 * time.Second)

	eg, ctx := errgroup.WithContext(suite.ctx)

	for _, node := range allNodes {
		eg.Go(func() error {
			attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			var d net.Dialer

			conn, err := d.DialContext(attemptCtx, "tcp", net.JoinHostPort(node, strconv.Itoa(nodePort)))
			if conn != nil {
				conn.Close() //nolint:errcheck
			}

			if firewallDefaultBlock {
				if err == nil {
					return errors.New("nodePort API should not be available")
				}

				if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("unexpected error: %w", err)
				}
			} else if err != nil {
				// ignore connection refused, as it's not firewall, but rather service proxy not ready yet
				if !strings.Contains(err.Error(), "connection refused") {
					return fmt.Errorf("nodePort API should be available: %w", err)
				}
			}

			return nil
		})
	}

	suite.Require().NoError(eg.Wait())
}

func init() {
	allSuites = append(allSuites, new(FirewallSuite))
}
