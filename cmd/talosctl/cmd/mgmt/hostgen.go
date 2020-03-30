package mgmt

import (
	"fmt"
	"io/ioutil"

	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"gopkg.in/yaml.v2"
)

// hostMachineConfigurations is an array of machine configs
type hostMachineConfigurations struct {
	Configs map[string]v1alpha1.MachineConfig `yaml:"hostConfigs"`
}

// CreateHostConfigs will create host-specific configs
func CreateHostConfigs(configsFile string, input *generate.Input) (configs map[string]*v1alpha1.Config, err error) {

	// read configs
	data, err := ioutil.ReadFile(configsFile)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %s", configsFile)
	}

	// unmarshall as array of machine configs
	hostMachineConfigs := hostMachineConfigurations{}

	err = yaml.Unmarshal([]byte(data), &hostMachineConfigs)
	if err != nil {
		return nil, fmt.Errorf("could not parse machine config yaml")
	}

	if len(hostMachineConfigs.Configs) == 0 {
		return nil, fmt.Errorf("no configs parsed from host-config yaml")
	}

	configs = make(map[string]*v1alpha1.Config)

	for machineID, machineConfig := range hostMachineConfigs.Configs {

		// determine machine type
		var t machine.Type
		switch machineConfig.MachineType {
		case "init":
			t = machine.TypeInit
		case "controlplane":
			t = machine.TypeControlPlane
		case "join":
			t = machine.TypeWorker
		default:
			return nil, fmt.Errorf("invalid machine type %s for machine %s", machineConfig.MachineType, machineID)
		}

		configs[machineID], err = generate.Config(t, input, &machineConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to generate config for machineId %s", machineID)
		}
	}

	return configs, nil
}
