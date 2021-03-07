// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/talos-systems/talos/pkg/provision"
)

func (p *provisioner) ensureImageExists(ctx context.Context, image string, options *provision.Options) error {
	// In order to pull an image, the reference must be in canonical
	// format (e.g. domain/repo/image:tag).
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	image = ref.String()

	// To filter the images, we need a familiar name and a tag
	// (e.g. domain/repo/image:tag => repo/image:tag).
	familiarName := reference.FamiliarName(ref)
	tag := ""

	if tagged, isTagged := ref.(reference.Tagged); isTagged {
		tag = tagged.Tag()
	}

	filters := filters.NewArgs()
	filters.Add("reference", familiarName+":"+tag)

	images, err := p.client.ImageList(ctx, types.ImageListOptions{Filters: filters})
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Fprintln(options.LogWriter, "downloading", image)

		var reader io.ReadCloser

		if reader, err = p.client.ImagePull(ctx, image, types.ImagePullOptions{}); err != nil {
			return err
		}

		//nolint:errcheck
		defer reader.Close()

		if _, err = io.Copy(ioutil.Discard, reader); err != nil {
			return err
		}
	}

	return nil
}
