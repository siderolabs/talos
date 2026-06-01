// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubelet provides minimal client for the kubelet API.
package kubelet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Client is a kubelet API client.
//
// Client can only talk to the local kubelet on the same node.
type Client struct {
	client *rest.RESTClient
}

// NewClient creates new kubelet API client.
func NewClient(nodename string, clientCert, clientKey, caPEM []byte) (*Client, error) {
	config := &rest.Config{
		Host: fmt.Sprintf("https://127.0.0.1:%d/", constants.KubeletPort),
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs},
		},

		TLSClientConfig: rest.TLSClientConfig{
			CertData:   clientCert,
			KeyData:    clientKey,
			CAData:     caPEM,
			ServerName: nodename,
		},
	}

	kubeletCert, err := os.ReadFile(filepath.Join(constants.KubeletPKIDir, "kubelet.crt"))
	if err == nil {
		config.CAData = append(config.CAData, kubeletCert...)
	} else if err != nil {
		// ignore if file doesn't exist, assume cert isn't self-signed
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("error reading kubelet certificate: %w", err)
		}
	}

	client := &Client{}

	client.client, err = rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("error building REST client: %w", err)
	}

	return client, nil
}

// Pods returns list of pods running on the kubelet.
func (c *Client) Pods(ctx context.Context) (*PodList, error) {
	var podList PodList

	bytes, err := c.client.Get().AbsPath("/pods/").Timeout(30 * time.Second).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &podList)
	if err != nil {
		return nil, err
	}

	return &podList, nil
}

// PodList is a list of pods.
type PodList struct {
	Items []Pod `json:"items"`
}

// Pod returns pod details.
type Pod struct {
	Metadata Metadata     `json:"metadata"`
	Status   v1.PodStatus `json:"status"`
}

// Metadata is a pod metadata.
type Metadata struct {
	Name        string      `json:"name"`
	Namespace   string      `json:"namespace"`
	Annotations Annotations `json:"annotations"`
}

// Annotations are the annotations on a pod.
type Annotations struct {
	// ConfigMapSource indicates where the resource is coming from.
	// Its value is "file"/"http" for static pods and "api" for resources came from kube-apiserver.
	ConfigSource string `json:"kubernetes.io/config.source"`
}
