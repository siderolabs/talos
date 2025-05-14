// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
	"github.com/siderolabs/kms-client/api/kms"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
	"github.com/siderolabs/talos/internal/pkg/endpoint"
	"github.com/siderolabs/talos/pkg/grpc/dialer"
	"github.com/siderolabs/talos/pkg/httpdefaults"
)

// KMSToken is the userdata stored in the partition token metadata.
type KMSToken struct {
	SealedData []byte `json:"sealedData"`
}

// KMSKeyHandler seals token using KMS service.
type KMSKeyHandler struct {
	KeyHandler
	kmsEndpoint   string
	getSystemInfo helpers.SystemInformationGetter
}

// NewKMSKeyHandler creates new KMSKeyHandler.
func NewKMSKeyHandler(key KeyHandler, kmsEndpoint string, getSystemInfo helpers.SystemInformationGetter) (*KMSKeyHandler, error) {
	return &KMSKeyHandler{
		KeyHandler:    key,
		kmsEndpoint:   kmsEndpoint,
		getSystemInfo: getSystemInfo,
	}, nil
}

// NewKey implements Handler interface.
func (h *KMSKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := h.getConn()
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing KMS endpoint %q: %w", h.kmsEndpoint, err)
	}

	defer conn.Close() //nolint:errcheck

	client := kms.NewKMSServiceClient(conn)

	key := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, err
	}

	systemInformation, err := h.getSystemInfo(ctx)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Seal(ctx, &kms.Request{
		NodeUuid: systemInformation.TypedSpec().UUID,
		Data:     key,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to seal KMS passphrase, slot %d: %w", h.Slot(), err)
	}

	token := &luks.Token[*KMSToken]{
		Type: TokenTypeKMS,
		UserData: &KMSToken{
			SealedData: resp.Data,
		},
	}

	return encryption.NewKey(h.slot, []byte(base64.StdEncoding.EncodeToString(key))), token, nil
}

// GetKey implements Handler interface.
func (h *KMSKeyHandler) GetKey(ctx context.Context, t token.Token) (*encryption.Key, error) {
	token, ok := t.(*luks.Token[*KMSToken])
	if !ok {
		return nil, ErrTokenInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := h.getConn()
	if err != nil {
		return nil, fmt.Errorf("error dialing KMS endpoint %q: %w", h.kmsEndpoint, err)
	}

	defer conn.Close() //nolint:errcheck

	client := kms.NewKMSServiceClient(conn)

	systemInformation, err := h.getSystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := client.Unseal(ctx, &kms.Request{
		NodeUuid: systemInformation.TypedSpec().UUID,
		Data:     token.UserData.SealedData,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unseal KMS passphrase, slot %d: %w", h.Slot(), err)
	}

	return encryption.NewKey(h.slot, []byte(base64.StdEncoding.EncodeToString(resp.Data))), nil
}

func (h *KMSKeyHandler) getConn() (*grpc.ClientConn, error) {
	var transportCredentials credentials.TransportCredentials

	endpoint, err := endpoint.Parse(h.kmsEndpoint)
	if err != nil {
		return nil, err
	}

	if endpoint.Insecure {
		transportCredentials = insecure.NewCredentials()
	} else {
		transportCredentials = credentials.NewTLS(&tls.Config{
			RootCAs: httpdefaults.RootCAs(),
		})
	}

	return grpc.NewClient(
		endpoint.Host,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithSharedWriteBuffer(true),
		grpc.WithContextDialer(dialer.DynamicProxyDialer),
	)
}
