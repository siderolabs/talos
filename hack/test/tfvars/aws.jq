{
    "cluster_name": .cluster_name,
    "num_control_planes": 3,
    "num_workers": 3,
    "ami_id": .ami_id,
    "ccm": true,
    "kubernetes_version": .kubernetes_version,
    "instance_type_control_plane": "t3.large",
    "instance_type_worker": "t3.large",
    "extra_tags": {
        "Name": .cluster_name,
        "Project": "talos-e2e-ci",
        "Environment": "ci"
    }
}
