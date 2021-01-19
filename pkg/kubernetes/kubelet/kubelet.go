// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubelet provides minimal client for the kubelet API.
package kubelet

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Client is a kubelet API client.
//
// Client can only talk to the local kubelet on the same node.
type Client struct {
	httpClient *http.Client
	hostname   string
}

// NewClient creates new kubelet API client.
func NewClient(clientCert tls.Certificate) (*Client, error) {
	rootCAs := stdx509.NewCertPool()

	kubeletCert, err := ioutil.ReadFile(filepath.Join(constants.KubeletPKIDir, "kubelet.crt"))
	if err != nil {
		return nil, fmt.Errorf("error reading kubelet certificate: %w", err)
	}

	rootCAs.AppendCertsFromPEM(kubeletCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCAs,
	}

	client := &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}

	client.hostname, err = os.Hostname()
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Pods returns list of pods running on the kubelet.
func (c *Client) Pods(ctx context.Context) (*v1.PodList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s:%d/pods", c.hostname, constants.KubeletPort), nil)
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body) //nolint: errcheck
		resp.Body.Close()                  //nolint: errcheck
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status response %d", resp.StatusCode)
	}

	var podList v1.PodList

	if err = json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return nil, fmt.Errorf("error decoding JSON response: %w", err)
	}

	return &podList, nil
}
