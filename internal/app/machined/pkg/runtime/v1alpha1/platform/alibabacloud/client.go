// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package alibabacloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	metadataEndpoint = "http://100.100.100.200/latest"
	tokenTTLSeconds  = "21600"

	metadataTokenHeader    = "X-Aliyun-Ecs-Metadata-Token"
	metadataTokenTTLHeader = "X-Aliyun-Ecs-Metadata-Token-Ttl-Seconds"
)

type metadataClient struct {
	endpoint string
	client   *http.Client
	token    string
}

func newMetadataClient() *metadataClient {
	return &metadataClient{
		endpoint: metadataEndpoint,
		client:   http.DefaultClient,
	}
}

func (client *metadataClient) getMetadataKey(ctx context.Context, key string) (string, error) {
	token, err := client.getToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/meta-data/%s", client.endpoint, key), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set(metadataTokenHeader, token)

	resp, err := client.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32)) //nolint:errcheck

		return "", fmt.Errorf("failed to fetch %q from Aliyun IMDS: status code %d, body %q", key, resp.StatusCode, string(body))
	}

	value, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(value), nil
}

func (client *metadataClient) getUserData(ctx context.Context) ([]byte, error) {
	token, err := client.getToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.endpoint+"/user-data", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set(metadataTokenHeader, token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32)) //nolint:errcheck

		return nil, fmt.Errorf("failed to fetch user-data from Aliyun IMDS: status code %d, body %q", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (client *metadataClient) getToken(ctx context.Context) (string, error) {
	if client.token != "" {
		return client.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, client.endpoint+"/api/token", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set(metadataTokenTTLHeader, tokenTTLSeconds)

	resp, err := client.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32)) //nolint:errcheck

		return "", fmt.Errorf("failed to fetch token from Aliyun IMDS: status code %d, body %q", resp.StatusCode, string(body))
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	client.token = strings.TrimSpace(string(token))
	if client.token == "" {
		return "", fmt.Errorf("failed to fetch token from Aliyun IMDS: empty token")
	}

	return client.token, nil
}
