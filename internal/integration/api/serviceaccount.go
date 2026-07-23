// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/go-retry/retry"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
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
	jobGVR = schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
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

	suite.ClearConnectionRefused(suite.ctx, suite.DiscoverNodeInternalIPs(suite.ctx)...)
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

	// Best-effort cleanup in case a previous run left this SA behind.
	suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name)                          //nolint:errcheck
	suite.EnsureResourceIsDeleted(suite.ctx, 30*time.Second, serviceAccountGVR, "kube-system", name) //nolint:errcheck

	sa, err := suite.createServiceAccount("kube-system", name, []string{"os:reader"})
	suite.Require().NoError(err)

	defer suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name) //nolint:errcheck

	err = suite.WaitForEventExists(suite.ctx, "kube-system", func(event eventsv1.Event) bool {
		return event.Regarding.UID == sa.GetUID() &&
			event.Type == corev1.EventTypeNormal &&
			event.Reason == "Synced"
	})
	suite.Require().NoError(err)

	secret, err := suite.waitForSecret("kube-system", name)
	suite.Require().NoError(err)
	suite.Assert().True(metav1.IsControlledBy(secret, sa))

	talosConfig := secret.Data["config"]

	conf, err := config.FromBytes(talosConfig)
	suite.Require().NoError(err)

	expectedServiceName := fmt.Sprintf(
		"%s.%s",
		constants.KubernetesTalosAPIServiceName,
		constants.KubernetesTalosAPIServiceNamespace,
	)
	suite.Assert().Equal([]string{expectedServiceName}, conf.Contexts[conf.Context].Endpoints)

	node := suite.RandomDiscoveredNodeInternalIP()

	_, err = suite.createTestJob("kube-system", name, name, node)
	suite.Assert().NoError(err)

	defer func() {
		suite.DeleteResource(suite.ctx, jobGVR, "kube-system", name) //nolint:errcheck

		suite.Assert().NoError(suite.EnsureResourceIsDeleted(suite.ctx, 30*time.Second, jobGVR, "kube-system", name)) //nolint:errcheck
	}()

	err = suite.waitForJobReady(2*time.Minute, "kube-system", name)
	suite.Assert().NoError(err)

	err = suite.DeleteResource(suite.ctx, serviceAccountGVR, "kube-system", name)
	suite.Require().NoError(err)

	// The controller owns the secret through an owner reference. Kubernetes GC
	// discovers CRDs asynchronously, so waiting for it here races a freshly
	// installed ServiceAccount CRD. Verify the owner reference above and clean
	// up the test object directly.
	err = suite.DeleteResource(suite.ctx, secretGVR, "kube-system", name)
	suite.Require().NoError(err)

	err = suite.EnsureResourceIsDeleted(suite.ctx, 30*time.Second, secretGVR, "kube-system", name)
	suite.Assert().NoError(err)
}

// TestNotAllowedNamespace tests Kubernetes service accounts in not allowed namespaces.
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
func (suite *ServiceAccountSuite) TestNotAllowedRoles() {
	name := "test-not-allowed-roles"

	err := suite.configureAPIAccess(true, []string{"os:reader"}, []string{"kube-system"})
	suite.Assert().NoError(err)

	sa, err := suite.createServiceAccount("kube-system", name, []string{"os:admin"})
	suite.Assert().NoError(err)
	suite.Assert().NotNil(sa)

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
	suite.Assert().NotNil(sa)

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
		Object: map[string]any{
			"apiVersion": fmt.Sprintf("%s/%s", constants.ServiceAccountResourceGroup, constants.ServiceAccountResourceVersion),
			"kind":       constants.ServiceAccountResourceKind,
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"roles": roles,
			},
		},
	}, metav1.CreateOptions{})
}

func (suite *ServiceAccountSuite) createTestJob(ns, name, serviceAccount, node string) (*unstructured.Unstructured, error) {
	return suite.DynamicClient.Resource(jobGVR).Namespace(ns).Create(suite.ctx, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fmt.Sprintf("%s/%s", jobGVR.Group, jobGVR.Version),
			"kind":       "Job",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"backoffLimit": int64(2),
				"template": map[string]any{
					"spec": map[string]any{
						"restartPolicy": "Never",
						"volumes": []map[string]any{
							{
								"name": "talos-secrets",
								"secret": map[string]any{
									"secretName": serviceAccount,
								},
							},
						},
						"containers": []map[string]any{
							{
								"name":  "talosctl",
								"image": "ghcr.io/siderolabs/talosctl:v1.13.5", // sync with cmd/talosctl/cmd/talos/image.go imageNames
								"args": []string{
									"--nodes", node,
									"version",
								},
								"volumeMounts": []map[string]any{
									{
										"mountPath": "/var/run/secrets/talos.dev",
										"name":      "talos-secrets",
									},
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
}

//nolint:gocyclo
func (suite *ServiceAccountSuite) waitForJobReady(duration time.Duration, ns, name string) error {
	cli := suite.DynamicClient.Resource(jobGVR).Namespace(ns)

	return retry.Constant(duration).RetryWithContext(suite.ctx, func(ctx context.Context) error {
		job, err := cli.Get(ctx, name, metav1.GetOptions{})
		if kubeerrors.IsNotFound(err) {
			return retry.ExpectedError(fmt.Errorf("job %s/%s not found", ns, name))
		} else if err != nil {
			return err
		}

		if job.Object["status"] == nil {
			return retry.ExpectedError(fmt.Errorf("job %s/%s status is not set", ns, name))
		}

		status := job.Object["status"].(map[string]any)

		// check if the job has been marked Failed (all backoff retries exhausted)
		if conditions, ok := status["conditions"].([]any); ok {
			for _, c := range conditions {
				cond, ok := c.(map[string]any)
				if !ok {
					continue
				}

				if cond["type"] == "Failed" && cond["status"] == "True" {
					failed, _ := status["failed"].(int64)
					podLogs := suite.getJobPodLogs(ctx, ns, name)

					return fmt.Errorf("job %s/%s exhausted retries (failed=%d)%s", ns, name, failed, podLogs)
				}
			}
		}

		if status["succeeded"] == nil || status["succeeded"].(int64) == 0 {
			return retry.ExpectedError(fmt.Errorf("job %s/%s is not ready yet", ns, name))
		}

		return nil
	})
}

func (suite *ServiceAccountSuite) getJobPodLogs(ctx context.Context, ns, jobName string) string {
	pods, err := suite.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	})
	if err != nil {
		return fmt.Sprintf(": (failed to list pods: %v)", err)
	}

	if len(pods.Items) == 0 {
		return ": (no pods found)"
	}

	// pick the most recently created pod
	newest := pods.Items[0]

	for _, p := range pods.Items[1:] {
		if p.CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newest = p
		}
	}

	tailLines := int64(50)

	req := suite.Clientset.CoreV1().Pods(ns).GetLogs(newest.Name, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})

	logs, err := req.DoRaw(ctx)
	if err != nil {
		return fmt.Sprintf(": pod %s (no logs: %v)", newest.Name, err)
	}

	return fmt.Sprintf(": pod %s logs:\n%s", newest.Name, string(logs))
}

// configureAPIAccess configures the API access feature on all control plane nodes.
//
//nolint:gocyclo
func (suite *ServiceAccountSuite) configureAPIAccess(
	enabled bool,
	allowedRoles []string,
	allowedNamespaces []string,
) error {
	controlPlaneIPs := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	for _, ip := range controlPlaneIPs {
		nodeCtx := client.WithNode(suite.ctx, ip)

		var patch any

		if !enabled {
			patch = map[string]any{
				"apiVersion": "v1alpha1",
				"kind":       k8s.KubeTalosAPIAccessConfig,
				"$patch":     "delete",
			}
		} else {
			cfg := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
			cfg.AccessAllowedKubernetesNamespaces = allowedNamespaces
			cfg.AccessAllowedRoles = allowedRoles

			patch = cfg
		}

		suite.PatchMachineConfig(nodeCtx, patch)
	}

	if enabled { // wait for CRD, Talos endpoint service, and at least one ready endpoint
		return retry.Constant(30*time.Second).RetryWithContext(suite.ctx, func(ctx context.Context) error {
			_, err := suite.getCRD()
			if err != nil {
				return retry.ExpectedError(err)
			}

			_, err = suite.Clientset.CoreV1().
				Services(constants.KubernetesTalosAPIServiceNamespace).
				Get(ctx, constants.KubernetesTalosAPIServiceName, metav1.GetOptions{})
			if err != nil {
				return retry.ExpectedError(err)
			}

			slices, err := suite.Clientset.DiscoveryV1().
				EndpointSlices(constants.KubernetesTalosAPIServiceNamespace).
				List(ctx, metav1.ListOptions{
					LabelSelector: "kubernetes.io/service-name=" + constants.KubernetesTalosAPIServiceName,
				})
			if err != nil {
				return retry.ExpectedError(err)
			}

			for _, slice := range slices.Items {
				for _, ep := range slice.Endpoints {
					if len(ep.Addresses) > 0 && (ep.Conditions.Ready == nil || *ep.Conditions.Ready) {
						return nil
					}
				}
			}

			return retry.ExpectedError(fmt.Errorf("service %s/%s has no ready endpoints",
				constants.KubernetesTalosAPIServiceNamespace, constants.KubernetesTalosAPIServiceName))
		})
	}

	return nil
}

func init() {
	allSuites = append(allSuites, new(ServiceAccountSuite))
}
