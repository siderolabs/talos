// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oauth2 implements OAuth2 Device Flow to authenticate machine config download.
package oauth2

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/mdp/qrterminal/v3"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/oauth2"

	metalurl "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Config represents the OAuth2 configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	Audience     string
	Scopes       []string

	ExtraVariables []string

	DeviceAuthURL string
	TokenURL      string

	extraHeaders map[string]string
}

// NewConfig returns a new Config from cmdline.
//
// If OAuth2 is not configured, it returns os.ErrNotExist.
//
//nolint:gocyclo
func NewConfig(cmdline *procfs.Cmdline, downloadURL string) (*Config, error) {
	var cfg Config

	clientID := cmdline.Get(constants.KernelParamConfigOAuthClientID).First()

	if clientID == nil {
		return nil, os.ErrNotExist
	}

	cfg.ClientID = *clientID

	if clientSecret := cmdline.Get(constants.KernelParamConfigOAuthClientSecret).First(); clientSecret != nil {
		cfg.ClientSecret = *clientSecret
	}

	if audience := cmdline.Get(constants.KernelParamConfigOAuthAudience).First(); audience != nil {
		cfg.Audience = *audience
	}

	for i := 0; ; i++ {
		scope := cmdline.Get(constants.KernelParamConfigOAuthScope).Get(i)

		if scope == nil {
			break
		}

		cfg.Scopes = append(cfg.Scopes, *scope)
	}

	for i := 0; ; i++ {
		extra := cmdline.Get(constants.KernelParamConfigOAuthExtraVariable).Get(i)

		if extra == nil {
			break
		}

		cfg.ExtraVariables = append(cfg.ExtraVariables, *extra)
	}

	if deviceAuthURL := cmdline.Get(constants.KernelParamConfigOAuthDeviceAuthURL).First(); deviceAuthURL != nil {
		cfg.DeviceAuthURL = *deviceAuthURL
	} else {
		u, err := url.Parse(downloadURL)
		if err != nil {
			return nil, err
		}

		u.Path = "/device/code"

		cfg.DeviceAuthURL = u.String()
	}

	if tokenURL := cmdline.Get(constants.KernelParamConfigOAuthTokenURL).First(); tokenURL != nil {
		cfg.TokenURL = *tokenURL
	} else {
		u, err := url.Parse(downloadURL)
		if err != nil {
			return nil, err
		}

		u.Path = "/token"

		cfg.TokenURL = u.String()
	}

	return &cfg, nil
}

// DeviceAuthFlow represents the device auth flow response.
func (c *Config) DeviceAuthFlow(ctx context.Context, st state.State) error {
	transport := httpdefaults.PatchTransport(cleanhttp.DefaultTransport())

	client := &http.Client{
		Transport: transport,
	}

	// register the HTTP client with OAuth2 flow
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	cfg := oauth2.Config{
		ClientID: c.ClientID,
		Scopes:   c.Scopes,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: c.DeviceAuthURL,
			TokenURL:      c.TokenURL,
		},
	}

	log.Printf("[OAuth] starting the authentication device flow with the following settings:")
	log.Printf("[OAuth]  - client ID: %q", c.ClientID)
	log.Printf("[OAuth]  - device auth URL: %q", c.DeviceAuthURL)
	log.Printf("[OAuth]  - token URL: %q", c.TokenURL)
	log.Printf("[OAuth]  - extra variables: %q", c.ExtraVariables)

	// acquire device variables
	variables, err := c.getVariableValues(ctx, st)
	if err != nil {
		return fmt.Errorf("failed to get variable values: %w", err)
	}

	var deviceAuthOptions []oauth2.AuthCodeOption //nolint:prealloc

	if c.Audience != "" {
		deviceAuthOptions = append(deviceAuthOptions, oauth2.SetAuthURLParam("audience", c.Audience))
	}

	for k, v := range variables {
		deviceAuthOptions = append(deviceAuthOptions, oauth2.SetAuthURLParam(k, v))
	}

	deviceAuthResponse, err := cfg.DeviceAuth(ctx, deviceAuthOptions...)
	if err != nil {
		return fmt.Errorf("failed to get device auth response: %w", err)
	}

	log.Printf("[OAuth] please visit the URL %s and enter the code %s", deviceAuthResponse.VerificationURI, deviceAuthResponse.UserCode)

	if deviceAuthResponse.VerificationURIComplete != "" {
		var qrBuf bytes.Buffer

		qrterminal.GenerateHalfBlock(deviceAuthResponse.VerificationURIComplete, qrterminal.L, &qrBuf)

		log.Printf("[OAuth] or scan the following QR code:\n%s", qrBuf.String())
	}

	log.Printf("[OAuth] waiting for the device to be authorized (expires at %s)...", deviceAuthResponse.Expiry.Format("15:04:05"))

	if c.ClientSecret != "" {
		deviceAuthOptions = append(deviceAuthOptions, oauth2.SetAuthURLParam("client_secret", c.ClientSecret))
	}

	token, err := cfg.DeviceAccessToken(ctx, deviceAuthResponse, deviceAuthOptions...)
	if err != nil {
		return fmt.Errorf("failed to get device access token: %w", err)
	}

	log.Printf("[OAuth] device authorized successfully")

	c.extraHeaders = map[string]string{
		"Authorization": token.Type() + " " + token.AccessToken,
	}

	return nil
}

// getVariableValues returns the variable values to include in the device auth request.
func (c *Config) getVariableValues(ctx context.Context, st state.State) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.ConfigLoadAttemptTimeout/2)
	defer cancel()

	return metalurl.MapValues(ctx, st, c.ExtraVariables)
}

// ExtraHeaders returns the extra headers to include in the download request.
func (c *Config) ExtraHeaders() map[string]string {
	return c.extraHeaders
}
