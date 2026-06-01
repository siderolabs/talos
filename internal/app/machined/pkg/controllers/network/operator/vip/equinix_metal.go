// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vip

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/packethost/packngo"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EquinixMetalHandler implements assignment and release of Virtual IPs using API.
type EquinixMetalHandler struct {
	client *packngo.Client

	logger *zap.Logger

	vip       string
	projectID string
	deviceID  string

	assignmentID string
}

// NewEquinixMetalHandler creates new EquinixMetalHandler.
func NewEquinixMetalHandler(logger *zap.Logger, vip string, spec network.VIPEquinixMetalSpec) *EquinixMetalHandler {
	return &EquinixMetalHandler{
		client: packngo.NewClientWithAuth("talos", spec.APIToken, nil),

		logger: logger,

		vip:       vip,
		projectID: spec.ProjectID,
		deviceID:  spec.DeviceID,
	}
}

// Acquire implements Handler interface.
func (handler *EquinixMetalHandler) Acquire(ctx context.Context) error {
	ips, _, err := handler.client.ProjectIPs.List(handler.projectID, &packngo.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing project IPs: %w", err)
	}

	// look up assignments for the VIP and unassign it
	for _, ip := range ips {
		if ip.Address != handler.vip {
			continue
		}

		for _, assignment := range ip.Assignments {
			assignmentID := path.Base(assignment.Href)

			if _, err = handler.client.DeviceIPs.Unassign(assignmentID); err != nil {
				return fmt.Errorf("error removing assignment %s: %w", assignment.String(), err)
			}

			handler.logger.Info("cleared previous Equinix Metal IP assignment", zap.String("assignment", assignmentID), zap.String("vip", handler.vip))
		}
	}

	// assign the VIP to this device
	assignment, _, err := handler.client.DeviceIPs.Assign(handler.deviceID, &packngo.AddressStruct{
		Address: handler.vip,
	})
	if err != nil {
		return fmt.Errorf("error assigning %q to %q: %w", handler.vip, handler.deviceID, err)
	}

	handler.logger.Info("assigned Equinix Metal IP", zap.String("vip", handler.vip), zap.String("device_id", handler.deviceID), zap.String("assignment", assignment.ID))
	handler.assignmentID = assignment.ID

	return nil
}

// Release implements Handler interface.
func (handler *EquinixMetalHandler) Release(ctx context.Context) error {
	if handler.assignmentID == "" {
		return nil
	}

	_, err := handler.client.DeviceIPs.Unassign(handler.assignmentID)
	if err != nil {
		return fmt.Errorf("error removing assignment %s: %w", handler.assignmentID, err)
	}

	handler.logger.Info("unassigned Equinix Metal IP", zap.String("assignment", handler.assignmentID), zap.String("vip", handler.vip))

	return nil
}

// EquinixMetalMetaDataEndpoint is the local endpoint for machine info like networking.
const EquinixMetalMetaDataEndpoint = "https://metadata.platformequinix.com/metadata"

// GetProjectAndDeviceIDs fills in parts of the spec based on the API token and instance metadata.
func GetProjectAndDeviceIDs(ctx context.Context, spec *network.VIPEquinixMetalSpec) error {
	metadataConfig, err := download.Download(ctx, EquinixMetalMetaDataEndpoint)
	if err != nil {
		return fmt.Errorf("error downloading metadata: %w", err)
	}

	type Metadata struct {
		ID string `json:"id"`
	}

	var unmarshalledMetadataConfig Metadata
	if err = json.Unmarshal(metadataConfig, &unmarshalledMetadataConfig); err != nil {
		return fmt.Errorf("error unmarshaling metadata: %w", err)
	}

	spec.DeviceID = unmarshalledMetadataConfig.ID

	client := packngo.NewClientWithAuth("talos", spec.APIToken, nil)

	device, _, err := client.Devices.Get(spec.DeviceID, &packngo.GetOptions{
		Includes: []string{"project"},
	})
	if err != nil {
		return fmt.Errorf("error getting device: %w", err)
	}

	spec.ProjectID = device.Project.ID

	return nil
}
