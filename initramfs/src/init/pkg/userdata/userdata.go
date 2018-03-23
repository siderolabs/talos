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
	Version string `yaml:"version"`
}

// Execute downloads the user data and executes the instructions.
func Execute() error {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return fmt.Errorf("parse /proc/cmdline: %s", err.Error())
	}
	url, ok := arguments[constants.UserDataURLFlag]
	if !ok {
		return nil
	}

	userDataBytes, err := download(url)
	if err != nil {
		return fmt.Errorf("download user data: %s", err.Error())
	}

	userData := &UserData{}
	if err := yaml.Unmarshal(userDataBytes, userData); err != nil {
		return fmt.Errorf("decode user data: %s", err.Error())
	}

	return nil
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	userDataBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return userDataBytes, nil
}
