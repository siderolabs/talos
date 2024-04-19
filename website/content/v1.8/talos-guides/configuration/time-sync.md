---
title: "Time Synchronization"
description: "Configuring time synchronization."
---

Talos Linux itself does not require time to be synchronized across the cluster, but as Talos Linux and Kubernetes components issue certificates
with expiration dates, it is recommended to have time synchronized across the cluster.
Some workloads (e.g. Ceph) might require to be in sync across the machines in the cluster due to the design of the application.

Talos Linux tries to launch API even if the time is not sync, and if time jumps as a result of NTP sync, the API certificates will be rotated automatically.
Some components like `kubelet` and `etcd` wait for the time to be in sync before starting, as they don't support graceful certificate rotation.

By default, Talos Linux uses `time.cloudflare.com` as the NTP server, but it can be overridden in the machine configuration, or provided via DHCP, kernel args, platform sources, etc.
Talos Linux implements SNTP protocol to sync time with the NTP server.

## Observing Status

Current time sync status can be observed with:

```shell
$ talosctl get timestatus
NODE         NAMESPACE   TYPE         ID     VERSION   SYNCED
172.20.0.2   runtime     TimeStatus   node   2         true
```

The list of servers Talos Linux is syncing with can be observed with:

```shell
$ talosctl get timeservers
NODE         NAMESPACE   TYPE               ID            VERSION   TIMESERVERS
172.20.0.2   network     TimeServerStatus   timeservers   1         ["time.cloudflare.com"]
```

More detailed logs about the time sync process can be queried with:

```shell
$ talosctl logs controller-runtime | grep -i time.Sync
172.20.0.2: 2024-04-17T18:32:16.690Z DEBUG NTP response {"component": "controller-runtime", "controller": "time.SyncController", "clock_offset": "37.060204ms", "rtt": "3.044816ms", "leap": 0, "stratum": 3, "precision": "29ns", "root_delay": "70.617676ms", "root_dispersion": "259.399µs", "root_distance": "37.090645ms"}
172.20.0.2: 2024-04-17T18:32:16.690Z DEBUG sample stats {"component": "controller-runtime", "controller": "time.SyncController", "jitter": "150.196588ms", "poll_interval": "34m8s", "spike": false}
172.20.0.2: 2024-04-17T18:32:16.690Z DEBUG adjusting time (slew) by 37.060204ms via 162.159.200.1, state TIME_OK, status STA_PLL | STA_NANO {"component": "controller-runtime", "controller": "time.SyncController"}
172.20.0.2: 2024-04-17T18:32:16.690Z DEBUG adjtime state {"component": "controller-runtime", "controller": "time.SyncController", "constant": 7, "offset": "37.060203ms", "freq_offset": -1302069, "freq_offset_ppm": -19}
```

## Using PTP Devices

When running in a VM on a hypervisor, instead of doing network time sync, Talos can sync the time to the hypervisor clock (if supported by the hypervisor).

To check if the PTP device is available:

```shell
$ talosctl ls /sys/class/ptp/
NODE         NAME
172.20.0.2   .
172.20.0.2   ptp0
```

Make sure that the PTP device is provided by the hypervisor, as some PTP devices don't provide accurate time value without proper setup:

```shell
talosctl read /sys/class/ptp/ptp0/clock_name
KVM virtual PTP
```

To enable PTP sync, set the `machine.time.servers` to the PTP device name (e.g. `/dev/ptp0`):

```yaml
machine:
  time:
    servers:
      - /dev/ptp0
```

After setting the PTP device, Talos will sync the time to the PTP device instead of using the NTP server:

```text
172.20.0.2: 2024-04-17T19:11:48.817Z DEBUG adjusting time (slew) by 32.223689ms via /dev/ptp0, state TIME_OK, status STA_PLL | STA_NANO {"component": "controller-runtime", "controller": "time.SyncController"}
```

## Additional Configuration

Talos NTP sync can be disabled with the following machine configuration patch:

```yaml
machine:
  time:
    disabled: true
```

When time sync is disabled, Talos assumes that time is always in sync.

Time sync can be also configured on best-effort basis, where Talos will try to sync time for the specified period of time, but if it fails to do so, time will be configured to be in sync when the period expires:

```yaml
machine:
  time:
    bootTimeout: 2m
```
