/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package azure

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	// AzureUserDataEndpoint is the local endpoint for the user data.
	// By specifying format=text and drilling down to the actual key we care about
	// we get a base64 encoded userdata response
	AzureUserDataEndpoint = "http://169.254.169.254/metadata/instance/compute/customData?api-version=2019-06-01&format=text"
	// AzureHostnameEndpoint is the local endpoint for the hostname.
	AzureHostnameEndpoint = "http://169.254.169.254/metadata/instance/compute/name?api-version=2019-06-01&format=text"
	// AzureInternalEndpoint is the Azure Internal Channel IP
	// https://blogs.msdn.microsoft.com/mast/2015/05/18/what-is-the-ip-address-168-63-129-16/
	AzureInternalEndpoint = "http://168.63.129.16"
)

// Azure is the concrete type that implements the platform.Platform interface.
type Azure struct{}

// Name implements the platform.Platform interface.
func (a *Azure) Name() string {
	return "Azure"
}

// UserData implements the platform.Platform interface.
func (a *Azure) UserData() (*userdata.UserData, error) {
	if err := linuxAgent(); err != nil {
		return nil, err
	}

	return userdata.Download(AzureUserDataEndpoint, userdata.WithHeaders(map[string]string{"Metadata": "true"}), userdata.WithFormat("base64"))
}

// Initialize implements the platform.Platform interface and handles additional system setup.
// nolint: dupl
func (a *Azure) Initialize(data *userdata.UserData) (err error) {
	var mountpoints *mount.Points
	mountpoints, err = owned.MountPointsFromLabels()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	hostnameBytes, err := hostname()
	if err != nil {
		return err
	}

	// Stub out networking
	if data.Networking == nil {
		data.Networking = &userdata.Networking{}
	}
	if data.Networking.OS == nil {
		data.Networking.OS = &userdata.OSNet{}
	}

	data.Networking.OS.Hostname = string(hostnameBytes)

	return err
}

func hostname() (hostname []byte, err error) {
	var req *http.Request
	var resp *http.Response

	req, err = http.NewRequest("GET", AzureHostnameEndpoint, nil)
	if err != nil {
		return
	}

	req.Header.Add("Metadata", "true")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return hostname, fmt.Errorf("failed to fetch hostname from metadata service: %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}
