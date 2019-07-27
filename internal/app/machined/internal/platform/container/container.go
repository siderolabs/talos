/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/base64"
	"os"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/userdata"

	"gopkg.in/yaml.v2"
)

// Container is a platform for installing Talos via an Container image.
type Container struct{}

// Name implements the platform.Platform interface.
func (c *Container) Name() string {
	return "Container"
}

// UserData implements the platform.Platform interface.
func (c *Container) UserData() (data *userdata.UserData, err error) {
	s, ok := os.LookupEnv("USERDATA")
	if !ok {
		return nil, errors.New("missing USERDATA environment variable")
	}
	var decoded []byte
	if decoded, err = base64.StdEncoding.DecodeString(s); err != nil {
		return nil, err
	}
	data = &userdata.UserData{}
	if err = yaml.Unmarshal(decoded, data); err != nil {
		return nil, err
	}

	return data, nil
}

// Prepare implements the platform.Platform interface.
func (c *Container) Prepare(data *userdata.UserData) (err error) {
	return nil
}

// Install implements the platform.Platform interface.
func (c *Container) Install(data *userdata.UserData) error {
	return nil
}
