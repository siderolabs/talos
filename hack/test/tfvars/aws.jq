{
    "cluster_name": .cluster_name,
    "num_control_planes": 3,
    "num_workers": (if .worker_group == "nvidia" then 0 else 3 end),
    "ami_id": .ami_id,
    "ccm": true,
    "kubernetes_version": .kubernetes_version,
    "instance_type_control_plane": "t3.large",
    "instance_type_worker": "t3.large",
    "extra_tags": {
        "Name": .cluster_name,
        "Project": "talos-e2e-ci",
        "Environment": "ci"
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
    ] else [] end)
}
