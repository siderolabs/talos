/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"log"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/userdata"
)

// PKI represents the PKI task.
type PKI struct{}

// NewPKITask initializes and returns an UserData task.
func NewPKITask() phase.Task {
	return &PKI{}
}

// RuntimeFunc returns the runtime function.
func (task *PKI) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *PKI) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.IsControlPlane() {
		log.Println("generating PKI locally")
		var csr *x509.CertificateSigningRequest
		if csr, err = data.NewIdentityCSR(); err != nil {
			return err
		}
		var crt *x509.Certificate
		crt, err = x509.NewCertificateFromCSRBytes(data.Security.OS.CA.Crt, data.Security.OS.CA.Key, csr.X509CertificateRequestPEM)
		if err != nil {
			return err
		}
		data.Security.OS.Identity.Crt = crt.X509CertificatePEM
		return nil
	}

	log.Println("generating PKI from trustd")
	var generator *gen.Generator
	generator, err = gen.NewGenerator(data, constants.TrustdPort)
	if err != nil {
		return errors.Wrap(err, "failed to create trustd client")
	}
	if err = generator.Identity(data); err != nil {
		return errors.Wrap(err, "failed to generate identity")
	}

	return nil
}
