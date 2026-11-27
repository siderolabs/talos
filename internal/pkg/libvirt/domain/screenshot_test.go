// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

var _ libvirtdomain.ScreenshotRPC = (*fakeScreenshotRPC)(nil)

var pngImage = []byte("\x89PNG\r\n\x1a\nrest-of-image")

type fakeScreenshotRPC struct {
	domain libvirt.Domain
	active int32
	xml    string
	image  []byte
	mime   libvirt.OptString
	err    error

	captured     bool
	disconnected bool
}

func (f *fakeScreenshotRPC) ConnectToURI(libvirt.ConnectURI) error { return nil }

func (f *fakeScreenshotRPC) Disconnect() error {
	f.disconnected = true

	return nil
}

func (f *fakeScreenshotRPC) DomainLookupByName(string) (libvirt.Domain, error) {
	return f.domain, nil
}

func (f *fakeScreenshotRPC) DomainIsActive(libvirt.Domain) (int32, error) { return f.active, nil }

func (f *fakeScreenshotRPC) DomainGetXMLDesc(libvirt.Domain, libvirt.DomainXMLFlags) (string, error) {
	return f.xml, nil
}

func (f *fakeScreenshotRPC) DomainScreenshot(_ libvirt.Domain, w io.Writer, screen, flags uint32) (libvirt.OptString, error) {
	f.captured = true

	if screen != 0 || flags != 0 {
		return nil, errors.New("unexpected screen or flags")
	}

	// Deliver in chunks as libvirt stream packets would.
	for chunk := range slicesChunk(f.image, 4) {
		if _, err := w.Write(chunk); err != nil {
			return nil, err
		}
	}

	return f.mime, f.err
}

func slicesChunk(b []byte, n int) func(func([]byte) bool) {
	return func(yield func([]byte) bool) {
		for len(b) > 0 {
			k := min(n, len(b))
			if !yield(b[:k]) {
				return
			}

			b = b[k:]
		}
	}
}

func screenshotXML(d libvirtdomain.Domain, devices string) string {
	return fmt.Sprintf(`<domain><name>%s</name><uuid>%s</uuid>
<metadata><definition xmlns="https://talos.dev/libvirt/domain">digest</definition></metadata>
<devices>%s</devices></domain>`, d.Name, d.UUID, devices)
}

//nolint:gocyclo // Table cases share one fake.
func TestScreenshot(t *testing.T) {
	t.Parallel()

	d := libvirtdomain.Domain{Name: "guest", UUID: uuid.New()}
	video := `<graphics type="vnc"/><video><model type="vga"/></video>`
	owned := libvirt.Domain{Name: d.Name, UUID: libvirt.UUID(d.UUID), ID: 3}
	decodeErr := errors.New("xdr:DecodeUint: EOF while decoding 4 bytes - read: '[]'")

	for _, tt := range []struct {
		name    string
		rpc     fakeScreenshotRPC
		mime    string
		image   []byte
		wantErr func(*testing.T, error)
		capture bool
	}{
		{
			name:    "reported MIME",
			rpc:     fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), image: pngImage, mime: libvirt.OptString{"image/png"}},
			mime:    libvirtdomain.ScreenshotMIMEPNG,
			image:   pngImage,
			capture: true,
		},
		{
			name:    "MIME decode failure after complete PNG",
			rpc:     fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), image: pngImage, err: decodeErr},
			mime:    libvirtdomain.ScreenshotMIMEPNG,
			image:   pngImage,
			capture: true,
		},
		{
			name:    "MIME decode failure after complete PPM",
			rpc:     fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), image: []byte("P6\n2 1\n255\nabcdef"), err: decodeErr},
			mime:    libvirtdomain.ScreenshotMIMEPPM,
			image:   []byte("P6\n2 1\n255\nabcdef"),
			capture: true,
		},
		{
			name: "MIME decode failure without data",
			rpc:  fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), err: decodeErr},
			wantErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, decodeErr)
			},
			capture: true,
		},
		{
			name: "libvirt error after partial data",
			rpc:  fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), image: pngImage, err: libvirt.ErrInterrupted},
			wantErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, libvirt.ErrInterrupted)
			},
			capture: true,
		},
		{
			name: "unrecognized format",
			rpc:  fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, video), image: []byte("GIF89a"), err: decodeErr},
			wantErr: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "unrecognized image format")
			},
			capture: true,
		},
		{
			name: "too large",
			rpc: fakeScreenshotRPC{
				domain: owned, active: 1, xml: screenshotXML(d, video),
				image: append(bytes.Clone(pngImage), make([]byte, libvirtdomain.MaxScreenshotSize)...),
			},
			wantErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, libvirtdomain.ErrScreenshotTooLarge)
			},
			capture: true,
		},
		{
			name:    "inactive",
			rpc:     fakeScreenshotRPC{domain: owned, active: 0, xml: screenshotXML(d, video)},
			wantErr: requirePrecondition,
		},
		{
			name:    "foreign UUID",
			rpc:     fakeScreenshotRPC{domain: libvirt.Domain{Name: d.Name, UUID: libvirt.UUID(uuid.New()), ID: 3}, active: 1, xml: screenshotXML(d, video)},
			wantErr: requirePrecondition,
		},
		{
			name:    "unowned",
			rpc:     fakeScreenshotRPC{domain: owned, active: 1, xml: strings.ReplaceAll(screenshotXML(d, video), "digest", "")},
			wantErr: requirePrecondition,
		},
		{
			name:    "no video device",
			rpc:     fakeScreenshotRPC{domain: owned, active: 1, xml: screenshotXML(d, "")},
			wantErr: requirePrecondition,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rpc := tt.rpc

			image, mime, err := libvirtdomain.TakeScreenshot(&rpc, "qemu:///system", d)
			require.Equal(t, tt.capture, rpc.captured)
			require.True(t, rpc.disconnected)

			if tt.wantErr != nil {
				require.Error(t, err)
				tt.wantErr(t, err)
				require.Nil(t, image)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.mime, mime)
			require.Equal(t, tt.image, image)
		})
	}
}

func requirePrecondition(t *testing.T, err error) {
	t.Helper()

	var pre *libvirtdomain.ConsolePreconditionError
	require.ErrorAs(t, err, &pre)
}
