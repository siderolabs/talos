// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package diagnostics

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	v1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// KubeletCSRNotApprovedCheck checks for kubelet server certificate rotation and no CSR approvers.
//
//nolint:gocyclo
func KubeletCSRNotApprovedCheck(ctx context.Context, r controller.Reader, logger *zap.Logger) (*runtime.DiagnosticSpec, error) {
	// check kubelet status to make sure it's running & health before proceeding any further
	kubeletService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "kubelet")
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading kubelet service: %w", err)
	}

	if !kubeletService.TypedSpec().Running || !kubeletService.TypedSpec().Healthy {
		return nil, nil
	}

	// fetch nodename
	nodeName, err := safe.ReaderGetByID[*k8s.Nodename](ctx, r, k8s.NodenameID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading nodename: %w", err)
	}

	// try to access kubelet API to see if we get 'tls: internal error'
	c, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp", "127.0.0.1:10250",
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)
	if err == nil {
		return nil, c.Close()
	}

	var netError *net.OpError

	if !errors.As(err, &netError) {
		// not our error
		return nil, nil
	}

	if !(netError.Op == "remote error" && netError.Err.Error() == tls.AlertError(80).Error()) { // remote error: tls: internal error
		return nil, nil
	}

	k8sClient, err := kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("error creating k8s client: %w", err)
	}

	defer k8sClient.Close() //nolint:errcheck

	csrs, err := k8sClient.Clientset.CertificatesV1().CertificateSigningRequests().List(ctx,
		metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.signerName", "kubernetes.io/kubelet-serving").String(),
		},
	)
	if err != nil {
		// error getting CSRs
		return nil, fmt.Errorf("error listing CSRs: %w", err)
	}

	expectedUsername := fmt.Sprintf("system:node:%s", nodeName.TypedSpec().Nodename)

	csrs.Items = xslices.Filter(csrs.Items, func(csr v1.CertificateSigningRequest) bool {
		if csr.Spec.Username != expectedUsername {
			return false
		}

		for _, condition := range csr.Status.Conditions {
			if condition.Type == v1.CertificateApproved {
				return false
			}
		}

		return true
	})

	if len(csrs.Items) == 0 {
		return nil, nil
	}

	return &runtime.DiagnosticSpec{
		Message: "kubelet server certificate rotation is enabled, but CSR is not approved",
		Details: []string{
			fmt.Sprintf("kubelet API error: %s", netError),
			fmt.Sprintf("pending CSRs: %s",
				strings.Join(
					xslices.Map(csrs.Items, func(csr v1.CertificateSigningRequest) string { return csr.Name }),
					", ",
				),
			),
		},
	}, nil
}
