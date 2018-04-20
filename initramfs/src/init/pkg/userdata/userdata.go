package userdata

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/kernel"
	yaml "gopkg.in/yaml.v2"
)

// UserData represents the user data.
type UserData struct {
	Version          string            `yaml:"version"`
	Token            string            `yaml:"token"`
	Join             bool              `yaml:"join,omitempty"`
	APIServer        string            `yaml:"apiServer,omitempty"`
	NodeName         string            `yaml:"nodeName,omitempty"`
	Labels           map[string]string `yaml:"labels,omitempty"`
	Nameservers      []string          `yaml:"nameservers,omitempty"`
	ContainerRuntime string            `yaml:"containerRuntime,omitempty"`
}

// Download downloads the user data and executes the instructions.
func Download() (UserData, error) {
	userData := UserData{}

	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return userData, fmt.Errorf("parse kernel parameters: %s", err.Error())
	}
	url, ok := arguments[constants.UserDataURLFlag]
	if !ok {
		return userData, nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return userData, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return userData, err
	}

	userDataBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return userData, err
	}

	if err != nil {
		return userData, fmt.Errorf("download user data: %s", err.Error())
	}

	if err := yaml.Unmarshal(userDataBytes, &userData); err != nil {
		return userData, fmt.Errorf("decode user data: %s", err.Error())
	}

	return userData, nil
}
