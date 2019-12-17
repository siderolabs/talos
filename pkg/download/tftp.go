// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pin/tftp"
)

// NewTFTPTransport returns an http.RoundTripper capable of handling the TFTP
// protocol.
func NewTFTPTransport() http.RoundTripper {
	return &tftpRoundTripper{}
}

var _ http.RoundTripper = &tftpRoundTripper{}

type tftpRoundTripper struct{}

func (t *tftpRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	addr := req.URL.Host

	if req.URL.Port() == "" {
		addr += ":69"
	}

	c, err := tftp.NewClient(addr)
	if err != nil {
		return nil, err
	}

	w, err := c.Receive(req.URL.Path, "octet")
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}

	written, err := w.WriteTo(buf)
	if err != nil {
		return nil, err
	}

	if expected, ok := w.(tftp.IncomingTransfer).Size(); ok {
		if written != expected {
			return nil, fmt.Errorf("expected %d bytes, got %d", expected, written)
		}
	}

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "TFTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Body:          ioutil.NopCloser(buf),
		ContentLength: -1,
		Request:       req,
	}, nil
}
