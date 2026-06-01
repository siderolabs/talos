{
    "cluster_name": .cluster_name,
    "ccm": true,
    "talos_version_contract": .talos_version_contract,
    "kubernetes_version": .kubernetes_version,
    "control_plane": {
        "ami_id": .ami_id,
        "instance_type": "t3.large"
    },
    "worker_groups": (if .worker_group == "nvidia" then [
        {
            "name": "nvidia-t4",
            "ami_id": .nvidia_ami_id,
            "instance_type": "g4dn.xlarge",
            "config_patch_files": [
                "nvidia.yaml"
            ],
            "tags": {
                "Type": "nvidia-t4"
            }
        }
    ] else [
        {
            "name": "default",
            "num_instances": 3,
            "ami_id": .ami_id,
            "instance_type": "t3.large"
        }
    ] end),
    "extra_tags": {
        "ClusterName": .cluster_name,
        "Project": "talos-e2e-ci",
        "Environment": "ci"
    }
}
