---
title: Deploy Your First Workload to a Talos Cluster
weight: 40
description: "Deploy a sample workload to your Talos cluster to get started."
---

Deploying your first workload validates that your cluster is working properly and that you can schedule, expose, and access applications successfully.

If you don’t have a cluster running yet, check out the [Getting Started]({{< relref "getting-started" >}}) or [Production Notes]({{< relref "prodnotes" >}}) guides to learn how to create one.

To deploy a sample application to your cluster:

1. Run this command to deploy the application and expose it via a NodePort:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/siderolabs/example-workload/refs/heads/main/deploy/example-svc-nodeport.yaml
    ```

1. Verify that your pods and services are running:

    ```bash
    kubectl get pods,services # Lists the deployed pods and services
    ```

1. Create a NODE_IP variable by retrieving an IP address of any one of your nodes:

    ```bash
    NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}'; echo)
    ```

1. Retrieve the NodePort assigned to your Service using:

    ```bash
    NODE_PORT=$(kubectl get svc example-workload -o jsonpath='{.spec.ports[0].nodePort}')
    ```

1. Verify your application is running:

    ```bash
    curl http://$NODE_IP:$NODE_PORT
    ```

   And here is your application:

    <pre>

    🎉 CONGRATULATIONS! 🎉
    ========================================

    You successfully deployed the example workload!

    Resources:
    ----------
    🔗 Talos Linux: https://talos.dev
    🔗 Omni: https://omni.siderolabs.com
    🔗 Sidero Labs: https://siderolabs.com

    ========================================

    </pre>

## What’s Next?

* [Pod Security]({{< relref "../kubernetes-guides/configuration/pod-security" >}})
* [Set up persistent storage]({{< relref "../kubernetes-guides/configuration/storage" >}})
* [Deploy a Metrics Server]({{< relref "../kubernetes-guides/configuration/deploy-metrics-server" >}})
* [Explore the talosctl CLI reference]({{< relref "../reference/cli" >}})
