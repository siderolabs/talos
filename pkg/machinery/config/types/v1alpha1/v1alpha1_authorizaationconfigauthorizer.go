// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"slices"
)

var allowedAuthorizationAuthorizerTypes = []string{"Node", "RBAC", "Webhook"}

// Type implements the config.AuthorizationConfig interface.
func (a *AuthorizationConfigAuthorizerConfig) Type() string {
	return a.AuthorizerType
}

// Name implements the config.AuthorizationConfig interface.
func (a *AuthorizationConfigAuthorizerConfig) Name() string {
	return a.AuthorizerName
}

// Webhook implements the config.AuthorizationConfig interface.
func (a *AuthorizationConfigAuthorizerConfig) Webhook() map[string]any {
	return a.AuthorizerWebhook.Object
}

// Validate validates the AuthorizationConfigAuthorizerConfig.
func (a *AuthorizationConfigAuthorizerConfig) Validate() error {
	if a.AuthorizerType == "" {
		return fmt.Errorf("authorizer type must be set")
	}

	if a.AuthorizerName == "" {
		return fmt.Errorf("authorizer name must be set")
	}

	if !slices.Contains(allowedAuthorizationAuthorizerTypes, a.AuthorizerType) {
		return fmt.Errorf("authorizer type %s is not allowed, allowed types are %v", a.AuthorizerType, allowedAuthorizationAuthorizerTypes)
	}

	return nil
}
