// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"

	"github.com/siderolabs/talos/pkg/download"
)

const (
	// AzureMetadata documentation
	// ref: https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
	// ref: https://github.com/Azure/azure-rest-api-specs/blob/main/specification/imds/data-plane/Microsoft.InstanceMetadataService/stable/2023-07-01/examples/GetInstanceMetadata.json

	// AzureVersion is the version of the Azure metadata service.
	AzureVersion = "2021-12-13"

	// AzureVersionFallback is the fallback version of the Azure metadata service (e.g. Azure Stack Hub).
	AzureVersionFallback = "2019-06-01"

	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
	// AzureMetadataEndpoint is the local endpoint for the metadata.
	AzureMetadataEndpoint = "http://169.254.169.254/metadata/instance/compute?api-version=%s&format=json"
	// AzureInterfacesEndpoint is the local endpoint to get external IPs.
	AzureInterfacesEndpoint = "http://169.254.169.254/metadata/instance/network/interface?api-version=%s&format=json"
	// AzureLoadbalancerEndpoint is the local endpoint for load balancer config.
	AzureLoadbalancerEndpoint = "http://169.254.169.254/metadata/loadbalancer?api-version=%s&format=json"

	mnt = "/mnt"
)

// ComputeMetadata represents metadata compute information.
type ComputeMetadata struct {
	Environment string `json:"azEnvironment,omitempty"`
	SKU         string `json:"sku,omitempty"`
	Name        string `json:"name,omitempty"`
	Zone        string `json:"zone,omitempty"`
	VMSize      string `json:"vmSize,omitempty"`
	OSType      string `json:"osType,omitempty"`
	OSProfile   struct {
		ComputerName string `json:"computerName,omitempty"`
	} `json:"osProfile,omitempty"`
	Location               string `json:"location,omitempty"`
	FaultDomain            string `json:"platformFaultDomain,omitempty"`
	PlatformSubFaultDomain string `json:"platformSubFaultDomain,omitempty"`
	UpdateDomain           string `json:"platformUpdateDomain,omitempty"`
	ResourceGroup          string `json:"resourceGroupName,omitempty"`
	ResourceID             string `json:"resourceId,omitempty"`
	VMScaleSetName         string `json:"vmScaleSetName,omitempty"`
	SubscriptionID         string `json:"subscriptionId,omitempty"`
	EvictionPolicy         string `json:"evictionPolicy,omitempty"`
}

func (a *Azure) getMetadata(ctx context.Context) (*ComputeMetadata, string, error) {
	apiVersion := AzureVersion
	errBadRequest := stderrors.New("bad request")

	metadataEndpoint := fmt.Sprintf(AzureMetadataEndpoint, apiVersion)

	log.Printf("fetching azure instance config from: %q", metadataEndpoint)

	metadataDl, err := download.Download(ctx, metadataEndpoint,
		download.WithHeaders(map[string]string{"Metadata": "true"}),
		download.WithErrorOnBadRequest(errBadRequest),
	)
	if err != nil && stderrors.Is(err, errBadRequest) {
		apiVersion = AzureVersionFallback
		metadataEndpoint = fmt.Sprintf(AzureMetadataEndpoint, apiVersion)

		log.Printf("fetching azure instance config from: %q", metadataEndpoint)

		metadataDl, err = download.Download(ctx, metadataEndpoint,
			download.WithHeaders(map[string]string{"Metadata": "true"}),
		)
	}

	if err != nil {
		return nil, "", fmt.Errorf("error fetching metadata: %w", err)
	}

	var metadata ComputeMetadata

	if err = json.Unmarshal(metadataDl, &metadata); err != nil {
		return nil, "", fmt.Errorf("failed to parse compute metadata: %w", err)
	}

	return &metadata, apiVersion, nil
}
