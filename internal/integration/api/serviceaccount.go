// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	serviceAccountGVR = schema.GroupVersionResource{
		Group:    constants.ServiceAccountResourceGroup,
		Version:  constants.ServiceAccountResourceVersion,
		Resource: constants.ServiceAccountResourcePlural,
	}
	secretGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
)

// ServiceAccountSuite verifies Talos ServiceAccount.
type ServiceAccountSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ServiceAccountSuite) SuiteName() string {
	return "api.ServiceAccountSuite"
}

// SetupTest ...
func (suite *ServiceAccountSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	suite.AssertClusterHealthy(suite.ctx)
}

// TearDownTest ...
func (suite *ServiceAccountSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestValid tests Kubernetes service accounts.
func (suite *ServiceAccountSuite) TestValid() {
	name := "test-valid"

	err := suite.configureAPIAccess(true, []string{"os:reader"}, []string{"kube-system"})
	suite.Assert().NoError(err)

	_, err = suite.getCRD()
	suite.Assert().NoError(err)

	sa, err := suite.createServiceAccount("kube-system", name, []string{"os:reader"})
	suite.Assert().NoError(err)

	defer suite.DeleteResource(suite.ctx, serviceAccountGVR, "default", name) //nolint:errcheck

	err = suite.WaitForEventExists(suite.ctx, "kube-system", func(event eventsv1.Event) bool {
		return event.Regarding.UID == sa.GetUID() &&
			event.Type == corev1.EventTypeNormal &&
			event.Reason == "Synced"
	})
	suite.Assert().NoError(err)

	secret, err := suite.waitForSecret("kube-system", name)
	suite.Assert().NoError(err)

	talosConfig := secret.Data["config"]

	conf, err := config.FromBytes(talosConfig)
	suite.Assert().NoError(err)

	expectedServiceName := fmt.Sprintf(
		"%s.%s",
		constants.KubernetesTalosAPIServiceName,
		constants.KubernetesTalosAPIServiceNamespace,
	)
	suite.Assert().Equal([]string{expectedServiceName}, conf.Contexts[conf.Context].Endpoints)

	err = suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name)
	suite.Require().NoError(err)

	err = suite.EnsureResourceIsDeleted(suite.ctx, 30*time.Second, secretGVR, "kube-system", name)
	suite.Assert().NoError(err)
}

// TestNotAllowedNamespace tests Kubernetes service accounts in not allowed namespaces.
//
//nolint:dupl
func (suite *ServiceAccountSuite) TestNotAllowedNamespace() {
	name := "test-allowed-ns"

	err := suite.configureAPIAccess(true, []string{"os:reader"}, []string{"kube-system"})
	suite.Require().NoError(err)

	sa, err := suite.createServiceAccount("default", name, []string{"os:reader"})
	suite.Require().NoError(err)

	defer suite.DeleteResource(suite.ctx, serviceAccountGVR, "default", name) //nolint:errcheck

	err = suite.WaitForEventExists(suite.ctx, "default", func(event eventsv1.Event) bool {
		return event.Regarding.UID == sa.GetUID() &&
			event.Type == corev1.EventTypeWarning &&
			event.Reason == "ErrNamespaceNotAllowed"
	})
	suite.Require().NoError(err)
}

// TestNotAllowedRoles tests Kubernetes service accounts with not allowed roles.
//
//nolint:dupl
func (suite *ServiceAccountSuite) TestNotAllowedRoles() {
	name := "test-not-allowed-roles"

	err := suite.configureAPIAccess(true, []string{"os:reader"}, []string{"kube-system"})
	suite.Assert().NoError(err)

	sa, err := suite.createServiceAccount("kube-system", name, []string{"os:admin"})
	suite.Assert().NoError(err)

	defer suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name) //nolint:errcheck

	err = suite.WaitForEventExists(suite.ctx, "kube-system", func(event eventsv1.Event) bool {
		return event.Regarding.UID == sa.GetUID() &&
			event.Type == corev1.EventTypeWarning &&
			event.Reason == "ErrRolesNotAllowed"
	})
	suite.Assert().NoError(err)
}

// TestFeatureNotEnabled tests Kubernetes service accounts when API access feature is not enabled.
func (suite *ServiceAccountSuite) TestFeatureNotEnabled() {
	name := "test-feature-not-enabled"

	err := suite.configureAPIAccess(false, []string{"os:reader"}, []string{"kube-system"})
	suite.Assert().NoError(err)

	sa, err := suite.createServiceAccount("kube-system", name, []string{"os:reader"})
	if kubeerrors.IsNotFound(err) {
		// CRD is not created because the feature was never enabled, all good
		return
	}

	suite.Assert().NoError(err)

	defer suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name) //nolint:errcheck

	err = suite.WaitForEventExists(suite.ctx, "kube-system", func(event eventsv1.Event) bool {
		return event.Regarding.UID == sa.GetUID() &&
			event.Type == corev1.EventTypeWarning &&
			event.Reason == "ErrAccessNotEnabled"
	})

	suite.Assert().NoError(err)
}

func (suite *ServiceAccountSuite) waitForSecret(ns, name string) (*corev1.Secret, error) {
	var (
		secret *corev1.Secret
		err    error
	)

	err = retry.Constant(1*time.Minute).RetryWithContext(suite.ctx, func(ctx context.Context) error {
		secret, err = suite.Clientset.CoreV1().Secrets(ns).Get(suite.ctx, name, metav1.GetOptions{})
		if kubeerrors.IsNotFound(err) {
			return retry.ExpectedError(err)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (suite *ServiceAccountSuite) getCRD() (*unstructured.Unstructured, error) {
	crdName := fmt.Sprintf("%s.%s", constants.ServiceAccountResourcePlural, constants.ServiceAccountResourceGroup)

	return suite.DynamicClient.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).Get(suite.ctx, crdName, metav1.GetOptions{})
}

func (suite *ServiceAccountSuite) createServiceAccount(ns string, name string, roles []string) (*unstructured.Unstructured, error) {
	return suite.DynamicClient.Resource(serviceAccountGVR).Namespace(ns).Create(suite.ctx, &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", constants.ServiceAccountResourceGroup, constants.ServiceAccountResourceVersion),
			"kind":       constants.ServiceAccountResourceKind,
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"roles": roles,
			},
		},
	}, metav1.CreateOptions{})
}

// configureAPIAccess configures the API access feature on all control plane nodes.
func (suite *ServiceAccountSuite) configureAPIAccess(
	enabled bool,
	allowedRoles []string,
	allowedNamespaces []string,
) error {
	controlPlaneIPs := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	for _, ip := range controlPlaneIPs {
		nodeCtx := client.WithNodes(suite.ctx, ip)

		nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
		if err != nil {
			return err
		}

		bytes := suite.PatchV1Alpha1Config(nodeConfig, func(nodeConfigRaw *v1alpha1.Config) {
			accessConfig := v1alpha1.KubernetesTalosAPIAccessConfig{
				AccessEnabled:                     pointer.To(enabled),
				AccessAllowedRoles:                allowedRoles,
				AccessAllowedKubernetesNamespaces: allowedNamespaces,
			}

			nodeConfigRaw.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig = &accessConfig
		})

		_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: bytes,
			Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
		})
		if err != nil {
			return err
		}
	}

	if enabled { // wait for CRD and the Talos endpoint to be created
		return retry.Constant(30*time.Second).RetryWithContext(suite.ctx, func(ctx context.Context) error {
			_, err := suite.getCRD()
			if err != nil {
				return retry.ExpectedError(err)
			}

			_, err = suite.Clientset.CoreV1().
				Services(constants.KubernetesTalosAPIServiceNamespace).
				Get(suite.ctx, constants.KubernetesTalosAPIServiceName, metav1.GetOptions{})
			if err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})
	}

	return nil
}

func init() {
	allSuites = append(allSuites, new(ServiceAccountSuite))
}
