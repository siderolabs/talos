// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

func TestReduceURLsToAddresses(t *testing.T) {
	endpoints := []string{
		"123.123.123.123",
		"exammple.com:111",
		"234.234.234.234:4000",
		"https://111.111.222.222:444",
		"localhost",
		"localhost:890",
		"https://[42a1:cfa:5458:3967:e2ce:afaa:6194:12f]:40000",
		"https://localhost:890",
		"2001:db8:0:0:0:ff00:42:8329",
		"https://[be4d:c25e:aca0:9366:68b7:c84:a23b:f7be]",
		"https://www.somecompany.com",
		"www.company.com",
		"[2001:db8:4006:812::200e]:8080",
		"grpc://222.22.2.1",
		"grpc://[794b:389:73cb:76a2:59de:62fd:ee38:7c]:111",
	}
	expected := []string{
		"123.123.123.123",
		"exammple.com:111",
		"234.234.234.234:4000",
		"111.111.222.222:444",
		"localhost",
		"localhost:890",
		"[42a1:cfa:5458:3967:e2ce:afaa:6194:12f]:40000",
		"localhost:890",
		"2001:db8:0:0:0:ff00:42:8329",
		"[be4d:c25e:aca0:9366:68b7:c84:a23b:f7be]:443",
		"www.somecompany.com:443",
		"www.company.com",
		"[2001:db8:4006:812::200e]:8080",
		"222.22.2.1",
		"[794b:389:73cb:76a2:59de:62fd:ee38:7c]:111",
	}

	actual := client.ReduceURLsToAddresses(endpoints)

	assert.Equal(t, expected, actual)
}

func TestBuildTLSConfig(t *testing.T) {
	//nolint:lll
	ca := `LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJQakNCOGFBREFnRUNBaEFtbGVURnRuRVY3b3NHYTJFSU9RVUJNQVVHQXl0bGNEQVFNUTR3REFZRFZRUUsKRXdWMFlXeHZjekFlRncweU1qQTRNVEl4T0RNeE1EZGFGdzB6TWpBNE1Ea3hPRE14TURkYU1CQXhEakFNQmdOVgpCQW9UQlhSaGJHOXpNQ293QlFZREsyVndBeUVBVGZ3RjFMQjVwVjg2cGw4cHN2aS93R2dWWmkvTm5NME8wYUZNCjBoenZZdzZqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUkKS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVWTRhSGg3UnJxRnVObFNydAo4bXY4ZHduUjRKQXdCUVlESzJWd0EwRUFTaE5jYURXMGwrU24xYSt5c21Sd2M2NGlBa3Y5dUlZNGdXU0t3RWJ4CnpYQlR3SkZWcmNPWlZNNS9pM0Y0UjFWZVkzM3QwdFBQMFBGZVF5MVRWTDlVQ0E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==`

	caBytes, err := base64.StdEncoding.DecodeString(ca)
	assert.Nil(t, err)

	expectedRootCAs := x509.NewCertPool()
	expectedRootCAs.AppendCertsFromPEM(caBytes)

	//nolint:lll
	crt := `LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJNekNCNXFBREFnRUNBaEVBZ1BscnFYWUtDeVNHRkxmazVVK2JQekFGQmdNclpYQXdFREVPTUF3R0ExVUUKQ2hNRmRHRnNiM013SGhjTk1qSXdPREV5TVRnek1UQTNXaGNOTXpJd09EQTVNVGd6TVRBM1dqQVRNUkV3RHdZRApWUVFLRXdodmN6cGhaRzFwYmpBcU1BVUdBeXRsY0FNaEFKblVxM1V1TzNTaGg4YW50eEZzNGJnZDlXeGRtcit6CmZURkxIcGpQVWlUaG8xSXdVREFPQmdOVkhROEJBZjhFQkFNQ0I0QXdIUVlEVlIwbEJCWXdGQVlJS3dZQkJRVUgKQXdFR0NDc0dBUVVGQndNQ01COEdBMVVkSXdRWU1CYUFGR09HaDRlMGE2aGJqWlVxN2ZKci9IY0owZUNRTUFVRwpBeXRsY0FOQkFNaW1wdnlxa0RHWDhROFErMTBtVWowYXJoQUpqdHl4OHErQll2QnlWOThxYyt3VldnYlFBc3FmClV3Sy9lN2ZLak1qMi9kRUZqOCs2SGZpOVJMTE5udzQ9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K`

	key := `LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM0Q0FRQXdCUVlESzJWd0JDSUVJQ3FTdHpMTTNzaHNqMlZld2dXaVBPaDJUT01uUmM3cmNyRkczTGhNaFdkQQotLS0tLUVORCBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0K`

	keyBytes, err := base64.StdEncoding.DecodeString(key)
	assert.Nil(t, err)

	crtBytes, err := base64.StdEncoding.DecodeString(crt)
	assert.Nil(t, err)

	expectedCert, err := tls.X509KeyPair(crtBytes, keyBytes)
	assert.Nil(t, err)

	expectedCerts := []tls.Certificate{expectedCert}

	t.Run("Returns default tls config for empty config context.", func(t *testing.T) {
		// given
		configContext := clientconfig.Context{}

		// when
		tlsConfig, err := client.BuildTLSConfig(&configContext)
		assert.Nil(t, err)

		// then
		expected := &tls.Config{}
		assert.Equal(t, expected, tlsConfig)
	})

	t.Run("Returns tls config with CA for config context with CA.", func(t *testing.T) {
		// given
		configContext := clientconfig.Context{
			CA: ca,
		}

		// when
		tlsConfig, err := client.BuildTLSConfig(&configContext)
		assert.Nil(t, err)

		// then
		assert.True(t, expectedRootCAs.Equal(tlsConfig.RootCAs))

		assert.Len(t, tlsConfig.Certificates, 0)
	})

	t.Run("Returns tls config with Certificate for config context with Crt and Key.", func(t *testing.T) {
		// given
		configContext := clientconfig.Context{
			Crt: crt,
			Key: key,
		}

		// when
		tlsConfig, err := client.BuildTLSConfig(&configContext)
		assert.Nil(t, err)

		// then
		assert.Equal(t, expectedCerts, tlsConfig.Certificates)
		assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConfig.ClientAuth)

		assert.Nil(t, tlsConfig.RootCAs)
	})

	t.Run("Returns tls config with CA and Certificate for config context with CA, Crt and Key.", func(t *testing.T) {
		// given
		configContext := clientconfig.Context{
			CA:  ca,
			Crt: crt,
			Key: key,
		}

		// when
		tlsConfig, err := client.BuildTLSConfig(&configContext)
		assert.Nil(t, err)

		// then
		assert.True(t, expectedRootCAs.Equal(tlsConfig.RootCAs))

		assert.Equal(t, expectedCerts, tlsConfig.Certificates)
		assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConfig.ClientAuth)
	})
}
