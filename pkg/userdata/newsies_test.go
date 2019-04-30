package userdata

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type validateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(validateSuite))
}
