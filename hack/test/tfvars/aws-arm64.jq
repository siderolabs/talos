{
    "cluster_name": .cluster_name,
    "ccm": true,
    "talos_version_contract": .talos_version_contract,
    "kubernetes_version": .kubernetes_version,
    "control_plane": (if .worker_group == "hypervisor" then {
        # A single bare-metal node running libvirtd: Graviton only exposes KVM on .metal sizes.
        # With no workers it must schedule pods itself. The content library patch uses a document
        # no released terraform provider decodes, so e2e-aws.sh applies it after boot.
        "ami_id": .extensions_ami_id,
        "instance_type": "c7g.metal",
        "config_patch_files": [
            "extensions-libvirt.yaml",
            "allow-scheduling-on-control-planes.yaml"
        ],
        "num_instances": 1
    } else {
        "ami_id": .ami_id,
        "instance_type": "t4g.large"
    } end),
    "worker_groups": (if .worker_group == "nvidia" then [
        {
            "name": "nvidia-t4",
            "ami_id": .extensions_ami_id,
            "instance_type": "g5g.xlarge",
            "config_patch_files": [
                "nvidia.yaml"
            ],
            "tags": {
                "Type": "nvidia-t4"
            }
        }
    ] elif .worker_group == "hypervisor" then [
    ] else [
        {
            "name": "default",
            "num_instances": 3,
            "ami_id": .ami_id,
            "instance_type": "t4g.large"
        }
    ] end),
    "extra_tags": {
        "ClusterName": .cluster_name,
        "Project": "talos-e2e-ci",
        "Environment": "ci"
    }
}
