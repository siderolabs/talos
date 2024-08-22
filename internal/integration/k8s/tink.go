// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	podsecurity "k8s.io/pod-security-admission/api"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/cluster/check"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// TinkSuite verifies Talos-in-Kubernetes.
type TinkSuite struct {
	base.K8sSuite
}

// SuiteName ...
func (suite *TinkSuite) SuiteName() string {
	return "k8s.TinkSuite"
}

//go:embed testdata/local-path-storage.yaml
var localPathStorageYAML []byte

const (
	tinkK8sPort   = "k8s-api"
	tinkTalosPort = "talos-api"
)

// TestDeploy verifies that tink can be deployed with a single control-plane node.
func (suite *TinkSuite) TestDeploy() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	suite.T().Cleanup(cancel)

	localPathStorage := suite.ParseManifests(localPathStorageYAML)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, localPathStorage)
	})

	suite.ApplyManifests(ctx, localPathStorage)

	const (
		namespace = "talos-in-talos"
		service   = "talos"
		ss        = "talos-cp"
	)

	talosImage := fmt.Sprintf("%s:%s", suite.TalosImage, version.Tag)

	suite.T().Logf("deploying Talos-in-Kubernetes from image %s", talosImage)

	tinkManifests := suite.getTinkManifests(namespace, service, ss, talosImage)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, tinkManifests)
	})

	suite.ApplyManifests(ctx, tinkManifests)

	// wait for the control-plane pod to be running
	suite.Require().NoError(suite.WaitForPodToBeRunning(ctx, time.Minute, namespace, ss+"-0"))

	// read back Service to figure out the ports
	svc, err := suite.Clientset.CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	suite.Require().NoError(err)

	var k8sPort, talosPort int

	for _, portSpec := range svc.Spec.Ports {
		switch portSpec.Name {
		case tinkK8sPort:
			k8sPort = int(portSpec.NodePort)
		case tinkTalosPort:
			talosPort = int(portSpec.NodePort)
		}
	}

	suite.Require().NotZero(k8sPort)
	suite.Require().NotZero(talosPort)

	// find pod IP
	pod, err := suite.Clientset.CoreV1().Pods(namespace).Get(ctx, ss+"-0", metav1.GetOptions{})
	suite.Require().NoError(err)

	suite.Require().NotEmpty(pod.Status.PodIP)

	podIP := netip.MustParseAddr(pod.Status.PodIP)

	// grab any random lbNode IP
	lbNode := suite.RandomDiscoveredNodeInternalIP()

	talosEndpoint := net.JoinHostPort(lbNode, strconv.Itoa(talosPort))

	in, err := generate.NewInput(namespace,
		fmt.Sprintf("https://%s", net.JoinHostPort(lbNode, strconv.Itoa(k8sPort))),
		constants.DefaultKubernetesVersion,
		generate.WithAdditionalSubjectAltNames([]string{lbNode}),
		generate.WithHostDNSForwardKubeDNSToHost(true),
	)
	suite.Require().NoError(err)

	// override pod/service subnets, as Talos-in-Talos would use it for "host" addresses
	in.PodNet = []string{"192.168.0.0/20"}
	in.ServiceNet = []string{"192.168.128.0/20"}

	cpCfg, err := in.Config(machine.TypeControlPlane)
	suite.Require().NoError(err)

	cpCfgBytes, err := cpCfg.Bytes()
	suite.Require().NoError(err)

	readyErr := suite.waitForEndpointReady(talosEndpoint)

	if readyErr != nil {
		suite.LogPodLogs(ctx, namespace, ss+"-0")
	}

	suite.Require().NoError(readyErr)

	insecureClient, err := client.New(ctx,
		client.WithEndpoints(talosEndpoint),
		client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	suite.Require().NoError(err)

	suite.T().Log("applying initial configuration")

	_, err = insecureClient.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: cpCfgBytes,
		Mode: machineapi.ApplyConfigurationRequest_AUTO,
	})
	suite.Require().NoError(err)

	// bootstrap
	talosconfig, err := in.Talosconfig()
	suite.Require().NoError(err)

	talosconfig.Contexts[talosconfig.Context].Endpoints = []string{talosEndpoint}
	talosconfig.Contexts[talosconfig.Context].Nodes = []string{podIP.String()}

	suite.T().Logf("talosconfig = %s", string(ensure.Value(talosconfig.Bytes())))

	readyErr = suite.waitForEndpointReady(talosEndpoint)

	if readyErr != nil {
		suite.LogPodLogs(ctx, namespace, ss+"-0")
	}

	suite.Require().NoError(readyErr)

	talosClient, err := client.New(ctx,
		client.WithConfigContext(talosconfig.Contexts[talosconfig.Context]),
	)
	suite.Require().NoError(err)

	suite.T().Log("bootstrapping")

	suite.Require().NoError(talosClient.Bootstrap(ctx, &machineapi.BootstrapRequest{}))

	clusterAccess := &tinkClusterAccess{
		KubernetesClient: cluster.KubernetesClient{
			ClientProvider: &cluster.ConfigClientProvider{
				TalosConfig: talosconfig,
			},
		},

		nodeIP: podIP,
	}

	suite.Require().NoError(
		check.Wait(
			ctx,
			clusterAccess,
			check.DefaultClusterChecks(),
			check.StderrReporter(),
		),
	)
}

type tinkClusterAccess struct {
	cluster.KubernetesClient

	nodeIP netip.Addr
}

func (access *tinkClusterAccess) Nodes() []cluster.NodeInfo {
	return []cluster.NodeInfo{
		{
			InternalIP: access.nodeIP,
			IPs:        []netip.Addr{access.nodeIP},
		},
	}
}

func (access *tinkClusterAccess) NodesByType(typ machine.Type) []cluster.NodeInfo {
	switch typ {
	case machine.TypeControlPlane:
		return []cluster.NodeInfo{
			{
				InternalIP: access.nodeIP,
				IPs:        []netip.Addr{access.nodeIP},
			},
		}
	case machine.TypeWorker, machine.TypeInit:
		return nil
	case machine.TypeUnknown:
		fallthrough
	default:
		panic(fmt.Sprintf("unexpected machine type: %s", typ))
	}
}

func (suite *TinkSuite) waitForEndpointReady(endpoint string) error {
	return retry.Constant(30*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		c, err := tls.Dial("tcp", endpoint,
			&tls.Config{
				InsecureSkipVerify: true,
			},
		)

		if c != nil {
			c.Close() //nolint:errcheck
		}

		return retry.ExpectedError(err)
	})
}

func (suite *TinkSuite) getTinkManifests(namespace, serviceName, ssName, talosImage string) []unstructured.Unstructured {
	labels := map[string]string{
		"app": "talos-cp",
	}

	tinkManifests := []runtime.Object{
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					podsecurity.EnforceLevelLabel: string(podsecurity.LevelPrivileged),
				},
			},
		},
		&corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeNodePort,
				Selector: labels,
				Ports: []corev1.ServicePort{
					{
						Name:       tinkK8sPort,
						Protocol:   corev1.ProtocolTCP,
						Port:       constants.DefaultControlPlanePort,
						TargetPort: intstr.FromString(tinkK8sPort),
					},
					{
						Name:       tinkTalosPort,
						Protocol:   corev1.ProtocolTCP,
						Port:       constants.ApidPort,
						TargetPort: intstr.FromString(tinkTalosPort),
					},
				},
			},
		},
	}

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ssName,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: serviceName,
			Replicas:    pointer.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "talos",
							Image:           talosImage,
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
								Privileged:             pointer.To(true),
								ReadOnlyRootFilesystem: pointer.To(true),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeUnconfined,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PLATFORM",
									Value: "container",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
									corev1.ResourceCPU:    resource.MustParse("750m"),
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: constants.ApidPort,
									Name:          tinkTalosPort,
								},
								{
									ContainerPort: constants.DefaultControlPlanePort,
									Name:          tinkK8sPort,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, ephemeralMount := range []string{"run", "system", "tmp"} {
		statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				MountPath: "/" + ephemeralMount,
				Name:      ephemeralMount,
			},
		)

		statefulSet.Spec.Template.Spec.Volumes = append(
			statefulSet.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: ephemeralMount,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		)
	}

	type overlayMountSpec struct {
		MountPoint string
		Size       string
	}

	for _, overlayMount := range append(
		[]overlayMountSpec{
			{
				MountPoint: constants.StateMountPoint,
				Size:       "100Mi",
			},
			{
				MountPoint: constants.EphemeralMountPoint,
				Size:       "6Gi",
			},
		},
		xslices.Map(
			xslices.Filter(constants.Overlays, func(overlay string) bool { return overlay != "/opt" }), // /opt/cni/bin contains CNI binaries
			func(mountPath string) overlayMountSpec {
				return overlayMountSpec{
					MountPoint: mountPath,
					Size:       "100Mi",
				}
			},
		)...,
	) {
		name := strings.ReplaceAll(strings.TrimLeft(overlayMount.MountPoint, "/"), "/", "-")

		statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				MountPath: overlayMount.MountPoint,
				Name:      name,
			},
		)

		statefulSet.Spec.VolumeClaimTemplates = append(
			statefulSet.Spec.VolumeClaimTemplates,
			corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(overlayMount.Size),
						},
					},
				},
			})
	}

	tinkManifests = append(tinkManifests, statefulSet)

	return xslices.Map(tinkManifests, suite.ToUnstructured)
}

func init() {
	allSuites = append(allSuites, new(TinkSuite))
}
