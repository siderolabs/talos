---
title: "Expose the Etcd Metrics Endpoint"
description: "Learn how to the expose etcd metrics endpoint."
---

To allow monitoring tools to collect metrics from your etcd database, you need to explicitly expose the etcd metrics endpoint.

Here's how to do it:

1. Create a patch file named `etcd-metrics-patch.yaml` that exposes the etcd metrics endpoint on `port:2381`, accessible from all network interfaces

    ```shell
    cat << EOF > etcd-metrics-patch.yaml
    - op: add
    path: /cluster/etcd/extraArgs
    value:
        listen-metrics-urls: http://0.0.0.0:2381
    EOF
    ```

1. Create a `CP_IPS` variable that contains the IP addresses of your control plane nodes:

    ```bash
    CP_IPS="<control-plane-ip-1>,<control-plane-ip-2>,<control-plane-ip-3>"
    ```

   **Note**: Don’t use a load balancer or VIP endpoint for the control-plane-ips.
   It should be calling the Talos API directly.

1. Run this command to export your `TALOSCONFIG` variable.
   You can skip this step if you've already done it:

    ```bash
    mkdir -p ~/.talos
    cp ./talosconfig ~/.talos/config
    ```

1. Apply the `etcd-metrics-patch.yaml` patch to your control plane nodes:

    ```bash
    talosctl patch machineconfig \
    --patch @etcd-metrics-patch.yaml \
    --endpoints $CP_IPS \
    --nodes $CP_IPS
    ```

1. Reboot the nodes.
   Note that if you have only one control plane node, rebooting it will cause cluster downtime.

    ```bash
    for NODE in $(echo "${CP_IPS}" | tr ',' ' '); do
        echo "Rebooting control plane node: $NODE"
        talosctl reboot --endpoints "$NODE" --nodes "$NODE" --wait
    done
    ```

1. After the node reboots, run the following command to confirm that the etcd metrics endpoint is accessible:

    ```bash
    CP_IP=$(echo $CP_IPS | cut -d',' -f1)
    curl "${CP_IP}:2381/metrics"
    ```
