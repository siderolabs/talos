---
title: "Knowledge Base"
weight: 1999
description: "Recipes for common configuration tasks with Talos Linux."
---

## Disabling `GracefulNodeShutdown` on a node

Talos Linux enables [Graceful Node Shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown) Kubernetes feature by default.

If this feature should be disabled, modify the `kubelet` part of the machine configuration with:

```yaml
machine:
  kubelet:
    extraArgs:
      feature-gates: GracefulNodeShutdown=false
    extraConfig:
      shutdownGracePeriod: 0s
      shutdownGracePeriodCriticalPods: 0s
```

## Generating Talos Linux ISO image with custom kernel arguments

Pass additional kernel arguments using `--extra-kernel-arg` flag:

```shell
$ docker run --rm -i ghcr.io/siderolabs/imager:{{< release >}} iso --arch amd64 --tar-to-stdout --extra-kernel-arg console=ttyS1 --extra-kernel-arg console=tty0 | tar xz
2022/05/25 13:18:47 copying /usr/install/amd64/vmlinuz to /mnt/boot/vmlinuz
2022/05/25 13:18:47 copying /usr/install/amd64/initramfs.xz to /mnt/boot/initramfs.xz
2022/05/25 13:18:47 creating grub.cfg
2022/05/25 13:18:47 creating ISO
```

ISO will be output to the file `talos-<arch>.iso` in the current directory.

## Logging Kubernetes audit logs with loki

If using loki-stack helm chart to gather logs from the Kubernetes cluster, you can use the helm values to configure loki-stack to log Kubernetes API server audit logs:

```yaml
promtail:
  extraArgs:
    - -config.expand-env
  # this is required so that the promtail process can read the kube-apiserver audit logs written as `nobody` user
  containerSecurityContext:
    capabilities:
      add:
        - DAC_READ_SEARCH
  extraVolumes:
    - name: audit-logs
      hostPath:
        path: /var/log/audit/kube
  extraVolumeMounts:
    - name: audit-logs
      mountPath: /var/log/audit/kube
      readOnly: true
  config:
    snippets:
      extraScrapeConfigs: |
        - job_name: auditlogs
          static_configs:
            - targets:
                - localhost
              labels:
                job: auditlogs
                host: ${HOSTNAME}
                __path__: /var/log/audit/kube/*.log
```

## Setting CPU scaling governor

While its possible to set [CPU scaling governor](https://kernelnewbies.org/Linux_5.9#CPU_Frequency_scaling) via `.machine.sysfs` it's sometimes cumbersome to set it for all CPU's individually.
A more elegant approach would be set it via a kernel commandline parameter.
This also means that the options are applied way early in the boot process.

This can be set in the machineconfig via the snippet below:

```yaml
machine:
  install:
    extraKernelArgs:
      - cpufreq.default_governor=performance
```

> Note: Talos needs to be upgraded for the `extraKernelArgs` to take effect.

## Disable `admissionControl` on control plane nodes

Talos Linux enables admission control in the API Server by default.

Although it is not recommended from a security point of view, admission control can be removed by patching your control plane machine configuration:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch-control-plane '[{"op": "remove", "path": "/cluster/apiServer/admissionControl"}]'
```
