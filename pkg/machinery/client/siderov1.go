// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/browser"
	authcli "github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp/client"
	"google.golang.org/grpc"
)

type authInterceptorConfig struct {
	provider    *client.KeyProvider
	contextName string
	identity    string
}

func newAuthInterceptorConfig(contextName, identity string) *authInterceptorConfig {
	return &authInterceptorConfig{
		provider:    client.NewKeyProvider("talos/keys"),
		contextName: contextName,
		identity:    identity,
	}
}

func (c *authInterceptorConfig) Interceptor() *interceptor.Signature {
	signerFunc := func(ctx context.Context, cc *grpc.ClientConn) (message.Signer, error) {
		return c.provider.ReadValidKey(c.contextName, c.identity)
	}

	renewSignerFunc := func(ctx context.Context, cc *grpc.ClientConn) (message.Signer, error) {
		return c.authenticate(ctx, cc)
	}

	authEnabledFunc := func(ctx context.Context, cc *grpc.ClientConn) (bool, error) {
		return true, nil
	}

	return interceptor.NewSignature(c.identity, signerFunc, renewSignerFunc, authEnabledFunc)
}

func (c *authInterceptorConfig) authenticate(ctx context.Context, cc *grpc.ClientConn) (*client.Key, error) {
	ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

	authCli := authcli.NewClient(cc)

	err := c.provider.DeleteKey(c.contextName, c.identity)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	pgpKey, err := c.provider.GenerateKey(c.contextName, c.identity, "Talos")
	if err != nil {
		return nil, err
	}

	publicKey, err := pgpKey.ArmorPublic()
	if err != nil {
		return nil, err
	}

	loginURL, err := authCli.RegisterPGPPublicKey(ctx, c.identity, []byte(publicKey))
	if err != nil {
		return nil, err
	}

	savePath, err := c.provider.WriteKey(pgpKey)
	if err != nil {
		return nil, err
	}

	err = browser.OpenURL(loginURL)
	if err != nil {
		fmt.Printf("Please visit this page to authenticate: %s\n", loginURL)
	}

	publicKeyID := pgpKey.Key.Fingerprint()

	err = authCli.AwaitPublicKeyConfirmation(ctx, publicKeyID)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Public key %s is now registered for user %s\n", publicKeyID, c.identity)

	fmt.Printf("PGP key saved to %s\n", savePath)

	return pgpKey, nil
}
