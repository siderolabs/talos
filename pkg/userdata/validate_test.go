package userdata

import (
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/suite"
)

type validateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(validateSuite))
}

func (suite *validateSuite) TestValidateWorkerData() {
	wd := genWorkerData()
	err := (*WorkerData)(wd).Validate()
	suite.Require().NoError(err.(*multierror.Error).ErrorOrNil())
}

func genWorkerData() *UserData {
	return &UserData{
		Version: "1",
		Services: &Services{
			Init: &Init{
				CNI: "flannel",
			},
			Kubeadm: &Kubeadm{
				ConfigurationStr: joinConfig,
			},
			Trustd: &Trustd{
				Token: "yolotoken",
			},
		},
	}
}

var joinConfig = `---
   apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      discovery:
        bootstrapToken:
          token: 'yolobootstraptoken'
          unsafeSkipCAVerification: true
          apiServerEndpoint: 127.0.0.1:443
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
      token: 'yolotoken'`
