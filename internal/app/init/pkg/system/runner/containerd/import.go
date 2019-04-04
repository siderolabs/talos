/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containerd

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
)

// ImportRequest represents an image import request.
type ImportRequest struct {
	Path    string
	Options []containerd.ImportOpt
}

// Import imports the images specified by the import requests.
func Import(namespace string, reqs ...*ImportRequest) (err error) {
	_, err = conditions.WaitForFileToExist(defaults.DefaultAddress)()
	if err != nil {
		return err
	}

	ctx := namespaces.WithNamespace(context.Background(), namespace)
	client, err := containerd.New(defaults.DefaultAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	var wg sync.WaitGroup

	wg.Add(len(reqs))

	for _, req := range reqs {
		go func(wg *sync.WaitGroup, r *ImportRequest) {
			defer wg.Done()

			tarball, err := os.Open(r.Path)
			if err != nil {
				panic(err)
			}

			imgs, err := client.Import(ctx, tarball, r.Options...)
			if err != nil {
				panic(err)
			}
			if err = tarball.Close(); err != nil {
				panic(err)
			}

			for _, img := range imgs {
				image := containerd.NewImage(client, img)
				log.Printf("unpacking %s (%s)\n", img.Name, img.Target.Digest)
				err = image.Unpack(ctx, containerd.DefaultSnapshotter)
				if err != nil {
					panic(err)
				}
			}
		}(&wg, req)
	}

	wg.Wait()

	return nil
}
