// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmemhttp_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/provision/internal/inmemhttp"
)

func TestServer(t *testing.T) {
	srv, err := inmemhttp.NewServer("localhost:0")
	assert.NoError(t, err)

	contents := []byte("DEADBEEF")

	assert.NoError(t, srv.AddFile("test.txt", contents))

	srv.Serve()
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	resp, err := http.Get(fmt.Sprintf("http://%s/test.txt", srv.GetAddr())) //nolint:noctx
	assert.NoError(t, err)

	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	got, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.Equal(t, contents, got)

	assert.NoError(t, resp.Body.Close())

	resp, err = http.Head(fmt.Sprintf("http://%s/test.txt", srv.GetAddr())) //nolint:noctx
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.EqualValues(t, 8, resp.ContentLength)

	assert.NoError(t, resp.Body.Close())

	resp, err = http.Get(fmt.Sprintf("http://%s/test.txt2", srv.GetAddr())) //nolint:noctx
	assert.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	assert.NoError(t, resp.Body.Close())

	assert.NoError(t, srv.Shutdown(context.Background()))
}
