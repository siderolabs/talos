// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"strings"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"libvirt.org/go/libvirtxml"
)

// MaxScreenshotSize bounds the encoded image accepted from libvirt.
const MaxScreenshotSize = 16 * 1024 * 1024

// Image MIME types libvirt's QEMU driver produces.
const (
	ScreenshotMIMEPNG = "image/png"
	ScreenshotMIMEPPM = "image/x-portable-pixmap"
)

var (
	pngSignature = []byte("\x89PNG\r\n\x1a\n")
	ppmSignature = []byte("P6")
)

// Screenshot captures the primary screen of an active, owned domain. The image
// is returned as libvirt produced it, along with its MIME type.
func (c *Connector) Screenshot(ctx context.Context, d Domain) ([]byte, string, error) {
	if err := validateDomain(d); err != nil {
		return nil, "", err
	}

	session, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(session, "unix", c.socket)
	if err != nil {
		return nil, "", err
	}

	defer closeTransport(conn)

	// go-libvirt RPCs have no context argument, so cancellation closes the transport.
	stop := context.AfterFunc(session, func() { closeTransport(conn) })
	defer stop()

	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))

	image, mime, err := screenshot(rpc, c.uri, d)
	if err != nil && session.Err() != nil {
		return nil, "", session.Err()
	}

	return image, mime, err
}

type screenshotRPC interface {
	ConnectToURI(libvirt.ConnectURI) error
	Disconnect() error
	DomainLookupByName(string) (libvirt.Domain, error)
	DomainIsActive(libvirt.Domain) (int32, error)
	DomainGetXMLDesc(libvirt.Domain, libvirt.DomainXMLFlags) (string, error)
	DomainScreenshot(libvirt.Domain, io.Writer, uint32, uint32) (libvirt.OptString, error)
}

func screenshot(rpc screenshotRPC, uri string, d Domain) ([]byte, string, error) {
	if err := rpc.ConnectToURI(libvirt.ConnectURI(uri)); err != nil {
		return nil, "", err
	}

	//nolint:errcheck // The transport is closed regardless.
	defer rpc.Disconnect()

	found, err := screenshotDomain(rpc, d)
	if err != nil {
		return nil, "", err
	}

	buf := &limitedBuffer{limit: MaxScreenshotSize}

	reported, err := rpc.DomainScreenshot(found, buf, 0, 0)
	if errors.Is(buf.err, ErrScreenshotTooLarge) {
		return nil, "", ErrScreenshotTooLarge
	}

	// go-libvirt decodes the reply from the stream's final, empty packet, so a
	// complete transfer may still fail to decode the MIME type. The libvirt error
	// path and transport failures are not XDR decode errors.
	if err != nil && (buf.Len() == 0 || !strings.HasPrefix(err.Error(), "xdr:")) {
		return nil, "", err
	}

	mime, err := screenshotMIME(buf.Bytes(), reported)
	if err != nil {
		return nil, "", err
	}

	return buf.Bytes(), mime, nil
}

func screenshotMIME(image []byte, reported libvirt.OptString) (string, error) {
	if len(reported) > 0 && reported[0] != "" {
		return reported[0], nil
	}

	switch {
	case bytes.HasPrefix(image, pngSignature):
		return ScreenshotMIMEPNG, nil
	case bytes.HasPrefix(image, ppmSignature):
		return ScreenshotMIMEPPM, nil
	default:
		return "", errors.New("screenshot has an unrecognized image format")
	}
}

func screenshotDomain(rpc screenshotRPC, d Domain) (libvirt.Domain, error) {
	found, err := rpc.DomainLookupByName(d.Name)
	if err != nil {
		return found, err
	}

	if found.Name != d.Name || found.UUID != libvirt.UUID(d.UUID) || found.ID <= 0 {
		return found, consolePreconditionError("screenshot domain identity or active ID mismatch")
	}

	active, err := rpc.DomainIsActive(found)
	if err != nil {
		return found, err
	}

	if active != 1 {
		return found, consolePreconditionError("screenshot domain is not active")
	}

	text, err := rpc.DomainGetXMLDesc(found, 0)
	if err != nil {
		return found, err
	}

	return found, screenshotDomainXML(text, d)
}

// screenshotDomainXML checks the live definition is owned by Talos and has a screen to capture.
func screenshotDomainXML(text string, d Domain) error {
	var desc libvirtxml.Domain
	if err := desc.Unmarshal(text); err != nil {
		return consolePreconditionError("malformed screenshot domain XML")
	}

	if desc.Name != d.Name || desc.UUID != d.UUID.String() || desc.Metadata == nil {
		return consolePreconditionError("screenshot domain XML ownership mismatch")
	}

	var metadata struct {
		Digest string `xml:"https://talos.dev/libvirt/domain definition"`
	}
	if err := xml.Unmarshal([]byte("<metadata>"+desc.Metadata.XML+"</metadata>"), &metadata); err != nil || metadata.Digest == "" {
		return consolePreconditionError("screenshot domain has no Talos ownership metadata")
	}

	if desc.Devices == nil || len(desc.Devices.Videos) == 0 {
		return consolePreconditionError("screenshot domain has no video device")
	}

	return nil
}

// ErrScreenshotTooLarge reports a screenshot larger than MaxScreenshotSize.
var ErrScreenshotTooLarge = errors.New("screenshot exceeds the maximum size")

// limitedBuffer fails writes past limit, which aborts the libvirt stream.
type limitedBuffer struct {
	bytes.Buffer

	limit int
	err   error
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}

	if b.Len()+len(p) > b.limit {
		b.err = ErrScreenshotTooLarge

		return 0, b.err
	}

	return b.Buffer.Write(p)
}
