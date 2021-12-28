// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
)

const notFoundError = "NotFoundError"

// AWS is the concrete type that implements the runtime.Platform interface.
type AWS struct {
	metadataClient *ec2metadata.EC2Metadata
}

// NewAWS initializes AWS platform building the IMDS client.
func NewAWS() (*AWS, error) {
	a := &AWS{}

	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, err
	}

	sess.Config.Credentials = ec2rolecreds.NewCredentials(sess)

	a.metadataClient = ec2metadata.New(sess)

	return a, nil
}

// Name implements the runtime.Platform interface.
func (a *AWS) Name() string {
	return "aws"
}

// Configuration implements the runtime.Platform interface.
func (a *AWS) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from AWS")

	userdata, err := a.metadataClient.GetUserDataWithContext(ctx)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == notFoundError {
				return nil, errors.ErrNoConfigSource
			}
		}

		return nil, fmt.Errorf("failed to fetch EC2 userdata: %w", err)
	}

	userdata = strings.TrimSpace(userdata)

	if userdata == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

// Mode implements the runtime.Platform interface.
func (a *AWS) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (a *AWS) Hostname(ctx context.Context) (hostname []byte, err error) {
	host, err := a.metadataClient.GetMetadataWithContext(ctx, "hostname")
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == notFoundError {
				return nil, nil
			}
		}

		return nil, fmt.Errorf("failed to fetch hostname from IMDS: %w", err)
	}

	return []byte(host), nil
}

// ExternalIPs implements the runtime.Platform interface.
func (a *AWS) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	publicIP, err := a.metadataClient.GetMetadataWithContext(ctx, "public-ipv4")
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == notFoundError {
				return nil, nil
			}
		}

		return nil, fmt.Errorf("failed to fetch public IPv4 from IMDS: %w", err)
	}

	if addr := net.ParseIP(publicIP); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (a *AWS) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
