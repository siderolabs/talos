/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/base64"
	"os"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/userdata/translate"
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
	trans, err := translate.NewTranslator("v1", string(decoded))
	if err != nil {
		return nil, err
	}
	data, err = trans.Translate()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Mode implements the platform.Platform interface.
func (c *Container) Mode() runtime.Mode {
	return runtime.Container
}

// Hostname implements the platform.Platform interface.
func (c *Container) Hostname() (hostname []byte, err error) {
	return nil, nil
}
