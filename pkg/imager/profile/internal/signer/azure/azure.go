// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package azure implements SecureBoot/PCR signers via Azure Key Vault.
package azure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

type authenticationMethod string

const (
	unknownAuthenticationMethod     = "unknown"
	environmentAuthenticationMethod = "environment"
	cliAuthenticationMethod         = "cli"
)

const azureClientID = "AZURE_CLIENT_ID"

// getAuthenticationMethod returns an authenticationMethod to use to get an Azure Authorizer.
// If no environment variables are set, unknownAuthMethod will be used.
// If the environment variable 'AZURE_AUTH_METHOD' is set to either environment or cli, use it.
// If the environment variables 'AZURE_TENANT_ID', 'AZURE_CLIENT_ID' and 'AZURE_CLIENT_SECRET' are set, use environment.
func getAuthenticationMethod() authenticationMethod {
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	authMethod := os.Getenv("AZURE_AUTH_METHOD")

	if authMethod != "" {
		switch strings.ToLower(authMethod) {
		case "environment":
			return environmentAuthenticationMethod
		case "cli":
			return cliAuthenticationMethod
		}
	}

	if tenantID != "" && clientID != "" && clientSecret != "" {
		return environmentAuthenticationMethod
	}

	return unknownAuthenticationMethod
}

type azureCredential interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

func getAzClientOpts() azcore.ClientOptions {
	envName := os.Getenv("AZURE_ENVIRONMENT")
	switch envName {
	case "AZUREUSGOVERNMENT", "AZUREUSGOVERNMENTCLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzureGovernment}
	case "AZURECHINACLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzureChina}
	case "AZURECLOUD", "AZUREPUBLICCLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzurePublic}
	default:
		return azcore.ClientOptions{Cloud: cloud.AzurePublic}
	}
}

// getAzureCredential takes an authenticationMethod and returns an Azure credential or an error.
//
// If the method is unknown, Environment will be tested and if it returns an error CLI will be tested.
// If the method is specified, the specified method will be used and no other will be tested.
// This means the following default order of methods will be used if nothing else is defined:
// 1. Client credentials (FromEnvironment)
// 2. Client certificate (FromEnvironment)
// 3. Username password (FromEnvironment)
// 4. MSI (FromEnvironment)
// 5. CLI (FromCLI).
func getAzureCredential(method authenticationMethod) (azureCredential, error) {
	clientOpts := getAzClientOpts()

	switch method {
	case environmentAuthenticationMethod:
		envCred, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: clientOpts})
		if err == nil {
			return envCred, nil
		}

		o := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: clientOpts}
		if ID, ok := os.LookupEnv(azureClientID); ok {
			o.ID = azidentity.ClientID(ID)
		}

		msiCred, err := azidentity.NewManagedIdentityCredential(o)
		if err == nil {
			return msiCred, nil
		}

		return nil, fmt.Errorf("failed to create default azure credential from env auth method: %w", err)
	case cliAuthenticationMethod:
		cred, err := azidentity.NewAzureCLICredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default Azure credential from env auth method: %w", err)
		}

		return cred, nil
	case unknownAuthenticationMethod:
		break
	default:
		return nil, errors.New("you should never reach this")
	}

	envCreds, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: clientOpts})
	if err == nil {
		return envCreds, nil
	}

	cliCreds, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create default Azure credential from env auth method: %w", err)
	}

	return cliCreds, nil
}

type azureCredentialAndError struct {
	cred azureCredential
	err  error
}

var azureCredentialsOnce = sync.OnceValue(func() azureCredentialAndError {
	authMethod := getAuthenticationMethod()

	cred, err := getAzureCredential(authMethod)

	return azureCredentialAndError{cred, err}
})

func getKeysClient(vaultURL string) (*azkeys.Client, error) {
	credAndError := azureCredentialsOnce()
	if credAndError.err != nil {
		return nil, credAndError.err
	}

	return azkeys.NewClient(vaultURL, credAndError.cred, nil)
}

func getCertsClient(vaultURL string) (*azcertificates.Client, error) {
	credAndError := azureCredentialsOnce()
	if credAndError.err != nil {
		return nil, credAndError.err
	}

	return azcertificates.NewClient(vaultURL, credAndError.cred, nil)
}
