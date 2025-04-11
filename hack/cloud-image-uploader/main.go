// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main implements Talos cloud image uploader.
package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

// Result of the upload process.
type Result []CloudImage

// CloudImage is the record official cloud image.
type CloudImage struct {
	Cloud  string `json:"cloud"`
	Tag    string `json:"version"`
	Region string `json:"region"`
	Arch   string `json:"arch"`
	Type   string `json:"type"`
	ID     string `json:"id"`
}

var (
	result   Result
	resultMu sync.Mutex
)

func pushResult(image CloudImage) {
	resultMu.Lock()
	defer resultMu.Unlock()

	result = append(result, image)
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("%s", err)
	}
}

//nolint:gocyclo
func run() error {
	var err error

	pflag.StringSliceVar(&DefaultOptions.TargetClouds, "target-clouds", DefaultOptions.TargetClouds, "cloud targets to upload to")
	pflag.StringSliceVar(&DefaultOptions.Architectures, "architectures", DefaultOptions.Architectures, "list of architectures to process")
	pflag.StringVar(&DefaultOptions.ArtifactsPath, "artifacts-path", DefaultOptions.ArtifactsPath, "artifacts path")
	pflag.StringVar(&DefaultOptions.Tag, "tag", DefaultOptions.Tag, "tag (version) of the uploaded image")
	pflag.StringVar(&DefaultOptions.NamePrefix, "name-prefix", DefaultOptions.NamePrefix, "prefix for the name of the uploaded image")

	pflag.StringSliceVar(&DefaultOptions.AWSRegions, "aws-regions", DefaultOptions.AWSRegions, "list of AWS regions to upload to")
	pflag.BoolVar(&DefaultOptions.AWSForceBIOS, "aws-force-bios", DefaultOptions.AWSForceBIOS, "force BIOS boot mode for AWS images")

	pflag.Parse()

	seed := make([]byte, 8)
	if _, err = cryptorand.Read(seed); err != nil {
		log.Fatalf("error seeding rand: %s", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	for _, target := range DefaultOptions.TargetClouds {
		switch target {
		case "aws":
			g.Go(func() error {
				if len(DefaultOptions.AWSRegions) == 0 {
					DefaultOptions.AWSRegions, err = GetAWSDefaultRegions(ctx)
					if err != nil {
						log.Printf("failed to get a list of enabled AWS regions: %s, ignored", err)
					}
				}

				aws := AWSUploader{
					Options: DefaultOptions,
				}

				return aws.Upload(ctx)
			})
		case "gcp":
			g.Go(func() error {
				gcp, err := NewGCPUploder(DefaultOptions)
				if err != nil {
					return fmt.Errorf("failed to create GCP uploader: %w", err)
				}

				return gcp.Upload(ctx)
			})
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}

	if err = g.Wait(); err != nil {
		return fmt.Errorf("failed: %w", err)
	}

	f, err := os.Create(filepath.Join(DefaultOptions.ArtifactsPath, "cloud-images.json"))
	if err != nil {
		return fmt.Errorf("failed: %w", err)
	}

	defer f.Close() //nolint:errcheck

	e := json.NewEncoder(io.MultiWriter(os.Stdout, f))
	e.SetIndent("", "  ")

	return e.Encode(&result)
}
