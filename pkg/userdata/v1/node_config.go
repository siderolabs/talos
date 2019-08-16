package v1

// NodeConfig holds the full representation of the node config
type NodeConfig struct {
	Version string
	Machine *MachineConfig
	Cluster *ClusterConfig
}
