// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"
	"log"

	"github.com/cosi-project/runtime/pkg/controller"
	osruntime "github.com/cosi-project/runtime/pkg/controller/runtime"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/config"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/time"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Controller implements runtime.V1alpha2Controller.
type Controller struct {
	controllerRuntime *osruntime.Runtime

	v1alpha1Runtime runtime.Runtime
}

// NewController creates Controller.
func NewController(v1alpha1Runtime runtime.Runtime, loggingManager runtime.LoggingManager) (*Controller, error) {
	ctrl := &Controller{
		v1alpha1Runtime: v1alpha1Runtime,
	}

	logWriter, err := loggingManager.ServiceLog("controller-runtime").Writer()
	if err != nil {
		return nil, err
	}

	logger := log.New(logWriter, "", log.LstdFlags|log.Lmsgprefix)

	ctrl.controllerRuntime, err = osruntime.NewRuntime(v1alpha1Runtime.State().V1Alpha2().Resources(), logger)

	return ctrl, err
}

// Run the controller runtime.
func (ctrl *Controller) Run(ctx context.Context) error {
	for _, c := range []controller.Controller{
		&v1alpha1.BootstrapStatusController{},
		&v1alpha1.ServiceController{
			// V1Events
			V1Alpha1Events: ctrl.v1alpha1Runtime.Events(),
		},
		&time.SyncController{
			V1Alpha1Mode: ctrl.v1alpha1Runtime.State().Platform().Mode(),
		},
		&config.MachineTypeController{},
		&config.K8sControlPlaneController{},
		&k8s.ControlPlaneStaticPodController{},
		&k8s.ExtraManifestController{},
		&k8s.KubeletStaticPodController{},
		&k8s.ManifestController{},
		&k8s.ManifestApplyController{},
		&k8s.RenderSecretsStaticPodController{},
		&secrets.EtcdController{},
		&secrets.KubernetesController{},
		&secrets.RootController{},
	} {
		if err := ctrl.controllerRuntime.RegisterController(c); err != nil {
			return err
		}
	}

	return ctrl.controllerRuntime.Run(ctx)
}

// DependencyGraph returns controller-resources dependencies.
func (ctrl *Controller) DependencyGraph() (*controller.DependencyGraph, error) {
	return ctrl.controllerRuntime.GetDependencyGraph()
}
