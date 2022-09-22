// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcp

import (
	"context"
	"strings"

	"cloud.google.com/go/compute/metadata"
)

const (
	gcpResolverServer = "169.254.169.254"
	gcpTimeServer     = "metadata.google.internal"
)

// MetadataConfig holds meta info.
type MetadataConfig struct {
	ProjectID    string `json:"project-id"`
	Name         string `json:"name,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Zone         string `json:"zone,omitempty"`
	InstanceType string `json:"machine-type"`
	InstanceID   string `json:"id"`
	PublicIPv4   string `json:"external-ip"`
}

func (g *GCP) getMetadata(context.Context) (*MetadataConfig, error) {
	var (
		meta MetadataConfig
		err  error
	)

	if meta.ProjectID, err = metadata.ProjectID(); err != nil {
		return nil, err
	}

	if meta.Name, err = metadata.InstanceName(); err != nil {
		return nil, err
	}

	instanceType, err := metadata.Get("instance/machine-type")
	if err != nil {
		return nil, err
	}

	meta.InstanceType = strings.TrimSpace(instanceType[strings.LastIndex(instanceType, "/")+1:])

	if meta.InstanceID, err = metadata.InstanceID(); err != nil {
		return nil, err
	}

	if meta.Hostname, err = metadata.Hostname(); err != nil {
		return nil, err
	}

	if meta.Zone, err = metadata.Zone(); err != nil {
		return nil, err
	}

	if meta.PublicIPv4, err = metadata.ExternalIP(); err != nil {
		if _, ok := err.(metadata.NotDefinedError); !ok {
			return nil, err
		}
	}

	return &meta, nil
}
