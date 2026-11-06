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

	"github.com/containerd/containerd/v2/core/remotes"
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
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	ourcosign "github.com/siderolabs/talos/internal/pkg/containers/image/verify/internal/cosign"
)

// newLocalRegistry starts an in-memory registry and returns a repository name in it.
func newLocalRegistry(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)

	return strings.TrimPrefix(server.URL, "http://") + "/test/image"
}

// newPlainHTTPResolver returns a resolver which talks plain HTTP to the local registry.
func newPlainHTTPResolver() remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(docker.MatchAllHosts)),
	})
}

// pushRandomImage pushes a random image to the repository and returns its digest reference.
func pushRandomImage(t *testing.T, repository string) name.Digest {
	t.Helper()

	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	digest, err := img.Digest()
	require.NoError(t, err)

	ref, err := name.NewDigest(repository+"@"+digest.String(), name.Insecure)
	require.NoError(t, err)

	require.NoError(t, remote.Write(ref, img))

	return ref
}

// legacySignatureTag returns the legacy cosign signature tag (sha256-xxx.sig) for the image.
func legacySignatureTag(ref name.Digest) name.Tag {
	return ref.Context().Tag(strings.ReplaceAll(ref.DigestStr(), ":", "-") + ".sig")
}

// cosignPayload returns the simple signing payload for the image.
func cosignPayload(t *testing.T, ref name.Digest) []byte {
	t.Helper()

	sigPayload, err := (&payload.Cosign{Image: ref}).MarshalJSON()
	require.NoError(t, err)

	return sigPayload
}

// pushLegacySignature pushes a legacy cosign signature manifest for the image: a single layer with the
// signed payload as its content and the signature material in the layer annotations.
func pushLegacySignature(t *testing.T, ref name.Digest, signedPayload []byte, annotations map[string]string) {
	t.Helper()

	sigImage, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer:       static.NewLayer(signedPayload, types.MediaType("application/vnd.dev.cosign.simplesigning.v1+json")),
		Annotations: annotations,
	})
	require.NoError(t, err)

	sigImage = mutate.MediaType(sigImage, types.OCIManifestSchema1)
	sigImage = mutate.ConfigMediaType(sigImage, types.OCIConfigJSON)

	require.NoError(t, remote.Write(legacySignatureTag(ref), sigImage))
}

// verifyLocalImage verifies the image in the local registry.
func verifyLocalImage(t *testing.T, resolver remotes.Resolver, ref name.Digest, checkOpts ourcosign.CheckOpts) (*ourcosign.VerifyResult, error) {
	t.Helper()

	namedRef, err := reference.ParseDockerRef(ref.String())
	require.NoError(t, err)

	canonicalRef, ok := namedRef.(reference.Canonical)
	require.True(t, ok, "image reference must be digested")

	return ourcosign.VerifyImage(t.Context(), zaptest.NewLogger(t), resolver, nil, canonicalRef, checkOpts)
}

func TestVerifyLegacyRejectsSignatureForAnotherImage(t *testing.T) {
	t.Parallel()

	repository := newLocalRegistry(t)

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	signerVerifier, err := signature.LoadECDSASignerVerifier(privateKey, crypto.SHA256)
	require.NoError(t, err)

	signedRef := pushRandomImage(t, repository)
	otherRef := pushRandomImage(t, repository)

	// sign the first image with a legacy cosign signature
	sigPayload := cosignPayload(t, signedRef)

	rawSig, err := signerVerifier.SignMessage(bytes.NewReader(sigPayload))
	require.NoError(t, err)

	pushLegacySignature(t, signedRef, sigPayload, map[string]string{
		"dev.cosignproject.cosign/signature": base64.StdEncoding.EncodeToString(rawSig),
	})

	// copy the signature of the signed image to the signature tag of the other image
	sigDesc, err := remote.Get(legacySignatureTag(signedRef))
	require.NoError(t, err)
	require.NoError(t, remote.Put(legacySignatureTag(otherRef), sigDesc))

	resolver := newPlainHTTPResolver()

	checkOpts := ourcosign.NewPublicKeyCheckOpts(signerVerifier)

	result, err := verifyLocalImage(t, resolver, signedRef, checkOpts)
	require.NoError(t, err)
	assert.Equal(t, "verified via legacy signature (bundle verified false)", result.Message)

	_, err = verifyLocalImage(t, resolver, otherRef, checkOpts)
	require.ErrorContains(t, err, "invalid or missing digest in claim")

	// the signature must not verify with a different public key
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	otherVerifier, err := signature.LoadECDSAVerifier(&otherKey.PublicKey, crypto.SHA256)
	require.NoError(t, err)

	checkOpts = ourcosign.NewPublicKeyCheckOpts(otherVerifier)

	_, err = verifyLocalImage(t, resolver, signedRef, checkOpts)
	require.ErrorContains(t, err, "no valid legacy signature")
	require.ErrorContains(t, err, "could not verify message")
}
