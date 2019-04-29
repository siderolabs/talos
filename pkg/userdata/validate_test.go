package userdata

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata/generate"
	yaml "gopkg.in/yaml.v2"
)

type validateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(validateSuite))
}

func (suite *GenerateSuite) SetupSuite() {

	input, err = generate.NewInput("test", []string{"10.0.1.5", "10.0.1.6", "10.0.1.7"})
	suite.Require().NoError(err)
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	dataString, err := generate.Userdata(generate.TypeInit, input)
	suite.Require().NoError(err)
	data := &userdata.UserData{}
	err = yaml.Unmarshal([]byte(dataString), data)
	suite.Require().NoError(err)
}
