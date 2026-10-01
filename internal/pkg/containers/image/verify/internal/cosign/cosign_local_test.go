// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cosign_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	ourcosign "github.com/siderolabs/talos/internal/pkg/containers/image/verify/internal/cosign"
)

func TestVerifyLegacyRejectsSignatureForAnotherImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)

	repository := strings.TrimPrefix(server.URL, "http://") + "/test/image"

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	signerVerifier, err := signature.LoadECDSASignerVerifier(privateKey, crypto.SHA256)
	require.NoError(t, err)

	pushImage := func() name.Digest {
		img, pushErr := random.Image(1024, 1)
		require.NoError(t, pushErr)

		digest, pushErr := img.Digest()
		require.NoError(t, pushErr)

		ref, pushErr := name.NewDigest(repository+"@"+digest.String(), name.Insecure)
		require.NoError(t, pushErr)

		require.NoError(t, remote.Write(ref, img))

		return ref
	}

	signatureTag := func(ref name.Digest) name.Tag {
		return ref.Context().Tag(strings.ReplaceAll(ref.DigestStr(), ":", "-") + ".sig")
	}

	signedRef := pushImage()
	otherRef := pushImage()

	// sign the first image with a legacy cosign signature
	sigPayload, err := (&payload.Cosign{Image: signedRef}).MarshalJSON()
	require.NoError(t, err)

	rawSig, err := signerVerifier.SignMessage(bytes.NewReader(sigPayload))
	require.NoError(t, err)

	sigImage, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: static.NewLayer(sigPayload, types.MediaType("application/vnd.dev.cosign.simplesigning.v1+json")),
		Annotations: map[string]string{
			"dev.cosignproject.cosign/signature": base64.StdEncoding.EncodeToString(rawSig),
		},
	})
	require.NoError(t, err)

	sigImage = mutate.MediaType(sigImage, types.OCIManifestSchema1)
	sigImage = mutate.ConfigMediaType(sigImage, types.OCIConfigJSON)

	require.NoError(t, remote.Write(signatureTag(signedRef), sigImage))

	// copy the signature of the signed image to the signature tag of the other image
	sigDesc, err := remote.Get(signatureTag(signedRef))
	require.NoError(t, err)
	require.NoError(t, remote.Put(signatureTag(otherRef), sigDesc))

	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchAllHosts)),
	})

	checkOpts := cosign.CheckOpts{
		Offline:     true,
		IgnoreTlog:  true,
		SigVerifier: signerVerifier,
	}

	verifyImage := func(ref name.Digest) (*ourcosign.VerifyResult, error) {
		namedRef, parseErr := reference.ParseDockerRef(ref.String())
		require.NoError(t, parseErr)

		canonicalRef, ok := namedRef.(reference.Canonical)
		require.True(t, ok, "image reference must be digested")

		return ourcosign.VerifyImage(t.Context(), zaptest.NewLogger(t), resolver, nil, canonicalRef, checkOpts)
	}

	result, err := verifyImage(signedRef)
	require.NoError(t, err)
	assert.Equal(t, "verified via legacy signature (bundle verified false)", result.Message)

	_, err = verifyImage(otherRef)
	require.ErrorContains(t, err, "invalid or missing digest in claim")
}
