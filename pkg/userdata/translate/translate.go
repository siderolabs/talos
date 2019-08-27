/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package translate

import (
	"errors"

	"github.com/talos-systems/talos/pkg/userdata"
)

// Translator is the interface that will be implemented by all future machine config versions
type Translator interface {
	Translate() (*userdata.UserData, error)
}

// NewTranslator returns an instance of the translator depending on version
func NewTranslator(apiVersion string, nodeConfig string) (Translator, error) {
	switch apiVersion {
	case "v1alpha1":
		return &V1Alpha1Translator{nodeConfig: nodeConfig}, nil
	default:
		return nil, errors.New("unknown translator")
	}
}
