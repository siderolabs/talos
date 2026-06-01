// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package aws implements SecureBoot/PCR signers via AWS Key Management Service.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

func getKmsClient(ctx context.Context, awsRegion string) (*kms.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		return nil, fmt.Errorf("error initializing AWS default config: %w", err)
	}

	return kms.NewFromConfig(awsCfg), nil
}

func getAcmClient(ctx context.Context, awsRegion string) (*acm.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		return nil, fmt.Errorf("error initializing AWS default config: %w", err)
	}

	return acm.NewFromConfig(awsCfg), nil
}
