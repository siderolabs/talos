// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"sync"

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
	var err error

	DefaultOptions.AWSRegions, err = GetAWSDefaultRegions()
	if err != nil {
		log.Printf("failed to get a list of enabled AWS regions: %s, ignored", err)
	}

	pflag.StringSliceVar(&DefaultOptions.Architectures, "architectures", DefaultOptions.Architectures, "list of architectures to process")
	pflag.StringVar(&DefaultOptions.ArtifactsPath, "artifacts-path", DefaultOptions.ArtifactsPath, "artifacts path")
	pflag.StringVar(&DefaultOptions.Tag, "tag", DefaultOptions.Tag, "tag (version) of the uploaded image")

	pflag.StringSliceVar(&DefaultOptions.AWSRegions, "aws-regions", DefaultOptions.AWSRegions, "list of AWS regions to upload to")

	pflag.Parse()

	seed := make([]byte, 8)
	if _, err := cryptorand.Read(seed); err != nil {
		log.Fatalf("error seeding rand: %s", err)
	}

	rand.Seed(int64(binary.LittleEndian.Uint64(seed)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		aws := AWSUploader{
			Options: DefaultOptions,
		}

		return aws.Upload(ctx)
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("failed: %s", err)
	}

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.Encode(&result) //nolint:errcheck
}
