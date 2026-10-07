// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

func vncXML(d libvirtdomain.Domain, path string) string {
	return fmt.Sprintf(`<domain><name>%s</name><uuid>%s</uuid>
<metadata><definition xmlns="https://talos.dev/libvirt/domain">digest</definition></metadata>
<devices><graphics type="vnc"><listen type="socket" socket="%s"/></graphics></devices></domain>`, d.Name, d.UUID, path)
}

func TestVNCSocketOwnership(t *testing.T) {
	d := libvirtdomain.Domain{Name: "guest", UUID: uuid.New()}

	good := vncXML(d, "/run/libvirt/qemu/domain-7-guest/vnc.sock")
	for _, tt := range []struct {
		name, text string
		id         int32
		valid      bool
	}{
		{"valid", good, 7, true},
		{"wrong active ID", good, 8, false},
		{"invalid active ID", good, -1, false},
		{"malformed", "<domain", 7, false},
		{"foreign name", strings.Replace(good, "<name>guest", "<name>foreign", 1), 7, false},
		{"foreign UUID", strings.Replace(good, d.UUID.String(), uuid.NewString(), 1), 7, false},
		{"unowned", strings.ReplaceAll(good, "digest", ""), 7, false},
		{"TCP", strings.Replace(good, `type="socket" socket=`, `type="address" address=`, 1), 7, false},
		{"legacy path", vncXML(d, "/run/libvirt/qemu/guest/vnc.sock"), 7, false},
		{"relative", vncXML(d, "domain-7-guest/vnc.sock"), 7, false},
		{"unclean", vncXML(d, "/run/../domain-7-guest/vnc.sock"), 7, false},
		{"other socket", vncXML(d, "/run/domain-7-guest/console.sock"), 7, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path, err := libvirtdomain.VNCSocket(tt.text, d, tt.id)
			if tt.valid {
				require.NoError(t, err)
				require.Equal(t, "/run/libvirt/qemu/domain-7-guest/vnc.sock", path)
			} else {
				var pre *libvirtdomain.ConsolePreconditionError
				require.ErrorAs(t, err, &pre)
			}
		})
	}
}

//nolint:gocyclo,cyclop // Each protocol mode injects a distinct attachment failure.
func TestVNCAttachment(t *testing.T) {
	for _, mode := range []string{"success", "ID changed", "path changed", "ownership changed", "UUID changed", "stopped", "control disconnect", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				startup, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()

				raw, control := net.Pipe()
				defer func() { assert.NoError(t, control.Close()) }()

				graphics, peer := net.Pipe()
				defer func() { assert.NoError(t, peer.Close()) }()

				d := libvirtdomain.Domain{Name: "guest", UUID: uuid.New()}
				path := "/run/libvirt/qemu/domain-7-guest/vnc.sock"

				done := make(chan struct{})
				go func() {
					defer close(done)
					defer func() { assert.NoError(t, control.Close()) }()

					if err := serveHandshake(control); err != nil {
						return
					}

					for round := range 2 {
						for _, proc := range []uint32{23, 150, 14} {
							call, err := readCall(control)
							if err != nil {
								return
							}

							if !assert.Equal(t, proc, binary.BigEndian.Uint32(call[8:12])) {
								return
							}

							var payload []byte

							switch proc {
							case 23:
								payload = consoleXDRString(d.Name)

								id := d.UUID
								if round == 1 && mode == "UUID changed" {
									id = uuid.New()
								}

								payload = append(payload, id[:]...)

								activeID := uint32(7)
								if round == 1 && mode == "ID changed" {
									activeID = 8
								}

								payload = binary.BigEndian.AppendUint32(payload, activeID)
							case 150:
								active := uint32(1)
								if round == 1 && mode == "stopped" {
									active = 0
								}

								payload = binary.BigEndian.AppendUint32(nil, active)
							case 14:
								p := path
								if round == 1 && mode == "path changed" {
									p = "/other/domain-7-guest/vnc.sock"
								}

								text := vncXML(d, p)
								if round == 1 && mode == "ownership changed" {
									text = strings.ReplaceAll(text, "digest", "")
								}

								payload = consoleXDRString(text)
							}

							if err = replyCall(control, call, payload); err != nil {
								return
							}
						}
					}

					_, err := io.Copy(io.Discard, control)
					if err != nil {
						assert.ErrorIs(t, err, io.ErrClosedPipe)
					}
				}()

				dialed := false
				stream, err := libvirtdomain.OpenVNCConn(libvirtdomain.New("", "qemu:///system"), ctx, startup, raw, d, func(ctx context.Context, network, address string) (net.Conn, error) {
					require.Equal(t, "unix", network)
					require.Equal(t, path, address)

					dialed = true

					if mode == "control disconnect" {
						require.NoError(t, control.Close())
						synctest.Wait()
						<-ctx.Done()

						return nil, ctx.Err()
					}

					if mode == "timeout" {
						<-ctx.Done()

						return nil, ctx.Err()
					}

					return graphics, nil
				})

				require.True(t, dialed)

				if mode != "success" {
					require.Error(t, err)
					require.Nil(t, stream)
					require.NoError(t, graphics.Close())
				} else {
					require.NoError(t, err)

					go func() {
						_, err := peer.Write([]byte{0, 255, 128})
						assert.NoError(t, err)
					}()

					got := make([]byte, 3)
					_, err = io.ReadFull(stream, got)
					require.NoError(t, err)
					require.Equal(t, []byte{0, 255, 128}, got)
					require.NoError(t, control.Close())
					synctest.Wait()

					_, err = stream.Read(got)
					require.Error(t, err)
					require.NoError(t, stream.Close())
				}

				<-done
			})
		})
	}
}

func TestVNCValidatesDomain(t *testing.T) {
	for _, d := range []libvirtdomain.Domain{{}, {Name: "first"}, {UUID: uuid.New()}} {
		stream, err := libvirtdomain.New("", "").OpenVNC(t.Context(), d)
		require.Nil(t, stream)
		require.ErrorContains(t, err, "domain name and UUID must be nonempty")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := libvirtdomain.New("", "").OpenVNC(ctx, libvirtdomain.Domain{Name: "guest", UUID: uuid.New()})
	require.ErrorIs(t, err, context.Canceled)
}
