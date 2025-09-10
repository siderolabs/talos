---
title: "Interactive Dashboard"
description: "A tool to inspect the running Talos machine state on the physical video console."
---

Interactive dashboard is enabled for all Talos platforms except for SBC images.
The dashboard can be disabled with kernel parameter `talos.dashboard.disabled=1`.

The dashboard runs only on the physical video console (not serial console) on the 2nd virtual TTY.
The first virtual TTY shows kernel logs same as in Talos <1.4.0.
The virtual TTYs can be switched with `<Alt+F1>` and `<Alt+F2>` keys.

Keys `<F1>` - `<Fn>` can be used to switch between different screens of the dashboard.

The dashboard is using either UEFI framebuffer or VGA/VESA framebuffer (for legacy BIOS boot).

## Dashboard Resolution Control

On legacy BIOS systems, the screen resolution can be adjusted with the [`vga=` kernel parameter](https://docs.kernel.org/fb/vesafb.html).

In modern kernels and platforms, this parameter is often ignored. For reliable results, it is recommended to boot with **UEFI**.

When running in **UEFI mode**, you can set the screen resolution through your hypervisor or UEFI firmware settings.

## Summary Screen (`F1`)

{{< imgproc "interactive-dashboard-1.png" Fit "920x920" >}}
Interactive Dashboard Summary Screen
{{< /imgproc >}}

The header shows brief information about the node:

* hostname
* Talos version
* uptime
* CPU and memory hardware information
* CPU and memory load, number of processes

Table view presents summary information about the machine:

* UUID (from SMBIOS data)
* Cluster name (when the machine config is available)
* Machine stage: `Installing`, `Upgrading`, `Booting`, `Maintenance`, `Running`, `Rebooting`, `Shutting down`, etc.
* Machine stage readiness: checks Talos service status, static pod status, etc. (for `Running` stage)
* Machine type: controlplane/worker
* Number of members discovered in the cluster
* Kubernetes version
* Status of Kubernetes components: `kubelet` and Kubernetes controlplane components (only on `controlplane` machines)
* Network information: Hostname, Addresses, Gateway, Connectivity, DNS and NTP servers

Bottom part of the screen shows kernel logs, same as on the virtual TTY 1.

## Monitor Screen (`F2`)

{{< imgproc "interactive-dashboard-2.png" Fit "920x920" >}}
Interactive Dashboard Monitor Screen
{{< /imgproc >}}

Monitor screen provides live view of the machine resource usage: CPU, memory, disk, network and processes.

## Network Config Screen (`F3`)

> Note: network config screen is only available for `metal` platform.

{{< imgproc "interactive-dashboard-3.png" Fit "920x920" >}}
Interactive Dashboard Network Config Screen
{{< /imgproc >}}

Network config screen provides editing capabilities for the `metal` [platform network configuration]({{< relref "install/bare-metal-platforms/network-config" >}}).

The screen is split into three sections:

* the leftmost section provides a way to enter network configuration: hostname, DNS and NTP servers, configure the network interface either via DHCP or static IP address, etc.
* the middle section shows the current network configuration.
* the rightmost section shows the network configuration which will be applied after pressing "Save" button.

Once the platform network configuration is saved, it is immediately applied to the machine.

## Running the Dashboard on a Serial Console

In some environments you might want to run the dashboard on a serial console instead of (or in addition to) VGA.
This is useful for:

* Headless servers with no physical monitor
* Serial-over-LAN (IPMI, iKVM, BMC)
* Hypervisors that only expose serial consoles

Talos itself does not (yet) natively launch the dashboard on serial consoles.
However, it can be achieved today with a privileged DaemonSet that attaches `/sbin/dashboard` to the host’s active serial TTY.

Example manifest:

```yaml
# ds.yaml

apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: serial-console
spec:
  selector:
    matchLabels:
      app: serial-console
  template:
    metadata:
      labels:
        app: serial-console
    spec:
      hostPID: true
      containers:
        - name: dashboard
          image: busybox:1.36
          securityContext:
            privileged: true
          env:
            - name: TERM
              value: linux
          command: ["/bin/sh","-c"]
          args:
            - |
              set -eu

              echo "=== detect active console ==="
              ACTIVE="$(cat /hostfs/sys/class/tty/console/active 2>/dev/null || true)"
              DEV=""
              if [ -n "$ACTIVE" ]; then
                for t in $ACTIVE; do
                  [ "$t" = "tty0" ] && continue
                  [ -e "/dev/$t" ] && DEV="/dev/$t" && break
                done
              fi

              if [ -z "${DEV:-}" ]; then
                for d in /dev/ttyS0 /dev/ttyAMA0 /dev/hvc0; do
                  [ -e "$d" ] && DEV="$d" && break
                done
              fi

              : "${DEV:?no serial TTY found}"
              echo "Using $DEV"
              stty -F "$DEV" raw -echo -ixon speed 115200 || true

              [ -S /system/run/machined/machine.sock ] || ln -sf /hostfs/system/run /system/run || true

              while true; do
                echo "Starting dashboard on $DEV ..."
                setsid sh -c "exec < $DEV > $DEV 2>&1 /hostfs/sbin/dashboard" || true
                sleep 5
              done
          volumeMounts:
            - { name: dev,         mountPath: /dev }
            - { name: hostfs,      mountPath: /hostfs, readOnly: true }
            - { name: system-run,  mountPath: /system/run }
      volumes:
        - { name: dev,        hostPath: { path: /dev } }
        - { name: hostfs,     hostPath: { path: / } }
        - { name: system-run, hostPath: { path: /system/run } }
```

Apply the manifest:

```bash
kubectl apply -f ds.yaml

```

Connect to the serial console via your host or IPMI/iKVM; the Talos dashboard should then open in its ncurses interface:

{{< imgproc "interactive-dashboard-4.png" Fit "920x920" >}}
Interactive Dashboard Summary Screen
{{< /imgproc >}}
