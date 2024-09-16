---
title: "Cgroups Resource Analysis"
description: "How to use `talosctl cgroups` to monitor resource usage on the node."
---

Talos provides a way to monitor resource usage of the [control groups](https://docs.kernel.org/admin-guide/cgroup-v2.html) on the machine.
This feature is useful to understand how much resources are being used by the containers and processes running on the machine.

Talos creates several system cgroups:

* `init` (contains `machined` PID 1)
* `system` (contains system services, and extension services)
* `podruntime` (contains CRI containerd, kubelet, etcd)

Kubelet creates a tree of cgroups for each pod, and each container in the pod, starting with `kubepods` as the root group.

Talos Linux might set some default limits for the cgroups, and these are not configurable at the moment.
Kubelet is configured by default to reserve some amount of RAM and CPU for system processes to prevent the system from becoming unresponsive under extreme resource pressure.

> Note: this feature is only available in `cgroupsv2` mode which is Talos default.

The `talosctl cgroups` command provides a way to monitor the resource usage of the cgroups on the machine, it has a set of presets which are described below.

## Presets

### `cpu`

```text
$ talosctl cgroups --preset=cpu
NAME                                                                          CpuWeight   CpuNice   CpuMax            CpuUser        User/%    CpuSystem      System/%   Throttled
.                                                                              unset       unset    []                 7m42.43755s   -         8m51.855608s   -                    0s
├──init                                                                           79           1    [   max 100000]     35.061148s     7.58%     41.027589s     7.71%              0s
├──kubepods                                                                       77           1    [   max 100000]   3m29.902395s    45.39%   4m41.033592s    52.84%              0s
│   ├──besteffort                                                                  1          19    [   max 100000]      1.297303s     0.62%      960.152ms     0.34%              0s
│   │   └──kube-system/kube-proxy-6r5bz                                            1          19    [   max 100000]      1.297441s   100.01%      960.014ms    99.99%              0s
│   │       ├──kube-proxy                                                          1          19    [   max 100000]      1.289143s    99.36%      958.587ms    99.85%              0s
│   │       └──sandbox                                                             1          19    [   max 100000]        9.724ms     0.75%             0s     0.00%              0s
│   └──burstable                                                                  14           9    [   max 100000]   3m28.653931s    99.41%   4m40.024231s    99.64%              0s
│       ├──kube-system/kube-apiserver-talos-default-controlplane-1                 8          11    [   max 100000]   2m22.458603s    68.28%   2m22.983949s    51.06%              0s
│       │   ├──kube-apiserver                                                      8          11    [   max 100000]   2m22.440159s    99.99%   2m22.976538s    99.99%              0s
│       │   └──sandbox                                                             1          19    [   max 100000]       14.774ms     0.01%       11.081ms     0.01%              0s
│       ├──kube-system/kube-controller-manager-talos-default-controlplane-1        2          18    [   max 100000]     17.314271s     8.30%      3.014955s     1.08%              0s
│       │   ├──kube-controller-manager                                             2          18    [   max 100000]     17.303941s    99.94%      3.001934s    99.57%              0s
│       │   └──sandbox                                                             1          19    [   max 100000]       11.675ms     0.07%       11.675ms     0.39%              0s
│       ├──kube-system/kube-flannel-jzx6m                                          4          14    [   max 100000]     38.986678s    18.68%   1m47.717143s    38.47%              0s
│       │   ├──kube-flannel                                                        4          14    [   max 100000]     38.962703s    99.94%   1m47.690508s    99.98%              0s
│       │   └──sandbox                                                             1          19    [   max 100000]       14.228ms     0.04%        7.114ms     0.01%              0s
│       └──kube-system/kube-scheduler-talos-default-controlplane-1                 1          19    [   max 100000]     20.103563s     9.63%     16.099219s     5.75%              0s
│           ├──kube-scheduler                                                      1          19    [   max 100000]     20.092317s    99.94%     16.086603s    99.92%              0s
│           └──sandbox                                                             1          19    [   max 100000]        11.93ms     0.06%        11.93ms     0.07%              0s
├──podruntime                                                                     79           1    [   max 100000]   4m59.707084s    64.81%    5m4.010222s    57.16%              0s
│   ├──etcd                                                                       79           1    [   max 100000]   2m38.215322s    52.79%    3m7.812204s    61.78%              0s
│   ├──kubelet                                                                    39           4    [   max 100000]   1m29.026444s    29.70%   1m23.112332s    27.34%              0s
│   └──runtime                                                                    39           4    [   max 100000]     48.501668s    16.18%     37.049334s    12.19%              0s
└──system                                                                         59           2    [   max 100000]     32.395345s     7.01%     12.176964s     2.29%              0s
    ├──apid                                                                       20           7    [   max 100000]      1.261381s     3.89%      756.827ms     6.22%              0s
    ├──dashboard                                                                   8          11    [   max 100000]     22.231337s    68.63%      5.328927s    43.76%              0s
    ├──runtime                                                                    20           7    [   max 100000]      7.282253s    22.48%      5.924559s    48.65%              0s
    ├──trustd                                                                     10          10    [   max 100000]      1.254353s     3.87%      220.698ms     1.81%              0s
    └──udevd                                                                      10          10    [   max 100000]       78.726ms     0.24%      233.244ms     1.92%              0s
```

In the CPU view, the following columns are displayed:

* `CpuWeight`: the CPU weight of the cgroup (relative, controls the CPU shares/bandwidth)
* `CpuNice`: the CPU nice value (direct translation of the `CpuWeight` to the `nice` value)
* `CpuMax`: the maximum CPU time allowed for the cgroup
* `CpuUser`: the total CPU time consumed by the cgroup and its children in user mode
* `User/%`: the percentage of CPU time consumed by the cgroup and its children in user mode relative to the parent cgroup
* `CpuSystem`: the total CPU time consumed by the cgroup and its children in system mode
* `System/%`: the percentage of CPU time consumed by the cgroup and its children in system mode relative to the parent cgroup
* `Throttled`: the total time the cgroup has been throttled on CPU

### `cpuset`

```bash
$ talosctl cgroups --preset=cpuset
NAME                                                                          CpuSet         CpuSet(Eff)    Mems           Mems(Eff)
.                                                                                                     0-1                             0
├──init                                                                                               0-1                             0
├──kubepods                                                                                           0-1                             0
│   ├──besteffort                                                                                     0-1                             0
│   │   └──kube-system/kube-proxy-6r5bz                                                               0-1                             0
│   │       ├──kube-proxy                                                                             0-1                             0
│   │       └──sandbox                                                                                0-1                             0
│   └──burstable                                                                                      0-1                             0
│       ├──kube-system/kube-apiserver-talos-default-controlplane-1                                    0-1                             0
│       │   ├──kube-apiserver                                                                         0-1                             0
│       │   └──sandbox                                                                                0-1                             0
│       ├──kube-system/kube-controller-manager-talos-default-controlplane-1                           0-1                             0
│       │   ├──kube-controller-manager                                                                0-1                             0
│       │   └──sandbox                                                                                0-1                             0
│       ├──kube-system/kube-flannel-jzx6m                                                             0-1                             0
│       │   ├──kube-flannel                                                                           0-1                             0
│       │   └──sandbox                                                                                0-1                             0
│       └──kube-system/kube-scheduler-talos-default-controlplane-1                                    0-1                             0
│           ├──kube-scheduler                                                                         0-1                             0
│           └──sandbox                                                                                0-1                             0
├──podruntime                                                                                         0-1                             0
│   ├──etcd                                                                                           0-1                             0
│   ├──kubelet                                                                                        0-1                             0
│   └──runtime                                                                                        0-1                             0
└──system                                                                                             0-1                             0
    ├──apid                                                                                           0-1                             0
    ├──dashboard                                                                                      0-1                             0
    ├──runtime                                                                                        0-1                             0
    ├──trustd                                                                                         0-1                             0
    └──udevd                                                                                          0-1                             0
```

This preset shows information about the CPU and memory sets of the cgroups, it is mostly useful with `kubelet` CPU manager.

* `CpuSet`: the CPU set of the cgroup
* `CpuSet(Eff)`: the effective CPU set of the cgroup
* `Mems`: the memory set of the cgroup (NUMA nodes)
* `Mems(Eff)`: the effective memory set of the cgroup

### `io`

```bash
$ talosctl cgroups --preset=io
NAME                                                                          Bytes Read/Written                         ios Read/Write             PressAvg10   PressAvg60   PressTotal
.                                                                             loop0: 94 MiB/0 B vda: 700 MiB/803 MiB                                  0.12         0.37       2m12.512921s
├──init                                                                       loop0: 231 KiB/0 B vda: 4.9 MiB/4.3 MiB    loop0: 6/0 vda: 206/37       0.00         0.00          232.446ms
├──kubepods                                                                   vda: 282 MiB/16 MiB                        vda: 3195/3172               0.00         0.00          383.858ms
│   ├──besteffort                                                             vda: 58 MiB/0 B                            vda: 678/0                   0.00         0.00           86.833ms
│   │   └──kube-system/kube-proxy-6r5bz                                       vda: 58 MiB/0 B                            vda: 678/0                   0.00         0.00           86.833ms
│   │       ├──kube-proxy                                                     vda: 58 MiB/0 B                            vda: 670/0                   0.00         0.00           86.554ms
│   │       └──sandbox                                                        vda: 692 KiB/0 B                           vda: 8/0                     0.00         0.00              467µs
│   └──burstable                                                              vda: 224 MiB/16 MiB                        vda: 2517/3172               0.00         0.00          308.616ms
│       ├──kube-system/kube-apiserver-talos-default-controlplane-1            vda: 76 MiB/16 MiB                         vda: 870/3171                0.00         0.00          151.677ms
│       │   ├──kube-apiserver                                                 vda: 76 MiB/16 MiB                         vda: 870/3171                0.00         0.00          156.375ms
│       │   └──sandbox                                                                                                                                0.00         0.00                 0s
│       ├──kube-system/kube-controller-manager-talos-default-controlplane-1   vda: 62 MiB/0 B                            vda: 670/0                   0.00         0.00           95.432ms
│       │   ├──kube-controller-manager                                        vda: 62 MiB/0 B                            vda: 670/0                   0.00         0.00          100.197ms
│       │   └──sandbox                                                                                                                                0.00         0.00                 0s
│       ├──kube-system/kube-flannel-jzx6m                                     vda: 36 MiB/4.0 KiB                        vda: 419/1                   0.00         0.00           64.203ms
│       │   ├──kube-flannel                                                   vda: 35 MiB/0 B                            vda: 399/0                   0.00         0.00            55.26ms
│       │   └──sandbox                                                                                                                                0.00         0.00                 0s
│       └──kube-system/kube-scheduler-talos-default-controlplane-1            vda: 50 MiB/0 B                            vda: 558/0                   0.00         0.00           64.331ms
│           ├──kube-scheduler                                                 vda: 50 MiB/0 B                            vda: 558/0                   0.00         0.00           62.821ms
│           └──sandbox                                                                                                                                0.00         0.00                 0s
├──podruntime                                                                 vda: 379 MiB/764 MiB                       vda: 3802/287674             0.39         0.39       2m13.409399s
│   ├──etcd                                                                   vda: 308 MiB/759 MiB                       vda: 2598/286420             0.50         0.41       2m15.407179s
│   ├──kubelet                                                                vda: 69 MiB/62 KiB                         vda: 834/13                  0.00         0.00          122.371ms
│   └──runtime                                                                vda: 76 KiB/3.9 MiB                        vda: 19/1030                 0.00         0.00          164.984ms
└──system                                                                     loop0: 18 MiB/0 B vda: 3.2 MiB/0 B         loop0: 590/0 vda: 116/0      0.00         0.00          153.609ms
    ├──apid                                                                   loop0: 1.9 MiB/0 B                         loop0: 103/0                 0.00         0.00            3.345ms
    ├──dashboard                                                              loop0: 16 MiB/0 B                          loop0: 487/0                 0.00         0.00           11.596ms
    ├──runtime                                                                                                                                        0.00         0.00           28.957ms
    ├──trustd                                                                                                                                         0.00         0.00                 0s
    └──udevd                                                                  vda: 3.2 MiB/0 B                           vda: 116/0                   0.00         0.00          135.586ms
```

In the IO (input/output) view, the following columns are displayed:

* `Bytes Read/Written`: the total number of bytes read and written by the cgroup and its children, per each blockdevice
* `ios Read/Write`: the total number of I/O operations read and written by the cgroup and its children, per each blockdevice
* `PressAvg10`: the average IO pressure of the cgroup and its children over the last 10 seconds
* `PressAvg60`: the average IO pressure of the cgroup and its children over the last 60 seconds
* `PressTotal`: the total IO pressure of the cgroup and its children (see [PSI](https://docs.kernel.org/accounting/psi.html#psi) for more information)

### `memory`

```bash
$ talosctl cgroups --preset=memory
NAME                                                                          MemCurrent   MemPeak    MemLow     Peak/Low   MemHigh    MemMin     Current/Min   MemMax
.                                                                                unset        unset      unset    unset%       unset      unset    unset%          unset
├──init                                                                        133 MiB      133 MiB    192 MiB    69.18%         max     96 MiB   138.35%            max
├──kubepods                                                                    494 MiB      505 MiB        0 B      max%         max        0 B      max%        1.4 GiB
│   ├──besteffort                                                               70 MiB       74 MiB        0 B      max%         max        0 B      max%            max
│   │   └──kube-system/kube-proxy-6r5bz                                         70 MiB       74 MiB        0 B      max%         max        0 B      max%            max
│   │       ├──kube-proxy                                                       69 MiB       73 MiB        0 B      max%         max        0 B      max%            max
│   │       └──sandbox                                                         872 KiB      2.2 MiB        0 B      max%         max        0 B      max%            max
│   └──burstable                                                               424 MiB      435 MiB        0 B      max%         max        0 B      max%            max
│       ├──kube-system/kube-apiserver-talos-default-controlplane-1             233 MiB      242 MiB        0 B      max%         max        0 B      max%            max
│       │   ├──kube-apiserver                                                  232 MiB      242 MiB        0 B      max%         max        0 B      max%            max
│       │   └──sandbox                                                         208 KiB      3.3 MiB        0 B      max%         max        0 B      max%            max
│       ├──kube-system/kube-controller-manager-talos-default-controlplane-1     78 MiB       80 MiB        0 B      max%         max        0 B      max%            max
│       │   ├──kube-controller-manager                                          78 MiB       80 MiB        0 B      max%         max        0 B      max%            max
│       │   └──sandbox                                                         212 KiB      3.3 MiB        0 B      max%         max        0 B      max%            max
│       ├──kube-system/kube-flannel-jzx6m                                       48 MiB       50 MiB        0 B      max%         max        0 B      max%            max
│       │   ├──kube-flannel                                                     46 MiB       48 MiB        0 B      max%         max        0 B      max%            max
│       │   └──sandbox                                                         216 KiB      3.1 MiB        0 B      max%         max        0 B      max%            max
│       └──kube-system/kube-scheduler-talos-default-controlplane-1              66 MiB       67 MiB        0 B      max%         max        0 B      max%            max
│           ├──kube-scheduler                                                   66 MiB       67 MiB        0 B      max%         max        0 B      max%            max
│           └──sandbox                                                         208 KiB      3.4 MiB        0 B      max%         max        0 B      max%            max
├──podruntime                                                                  549 MiB      647 MiB        0 B      max%         max        0 B      max%            max
│   ├──etcd                                                                    382 MiB      482 MiB    256 MiB   188.33%         max        0 B      max%            max
│   ├──kubelet                                                                 103 MiB      104 MiB    192 MiB    54.31%         max     96 MiB   107.57%            max
│   └──runtime                                                                  64 MiB       71 MiB    392 MiB    18.02%         max    196 MiB    32.61%            max
└──system                                                                      229 MiB      232 MiB    192 MiB   120.99%         max     96 MiB   239.00%            max
    ├──apid                                                                     26 MiB       28 MiB     32 MiB    88.72%         max     16 MiB   159.23%         40 MiB
    ├──dashboard                                                               113 MiB      113 MiB        0 B      max%         max        0 B      max%        196 MiB
    ├──runtime                                                                  74 MiB       77 MiB     96 MiB    79.89%         max     48 MiB   154.57%            max
    ├──trustd                                                                   10 MiB       11 MiB     16 MiB    69.85%         max    8.0 MiB   127.78%         24 MiB
    └──udevd                                                                   6.8 MiB       14 MiB     16 MiB    86.87%         max    8.0 MiB    84.67%            max
```

In the memory view, the following columns are displayed:

* `MemCurrent`: the current memory usage of the cgroup and its children
* `MemPeak`: the peak memory usage of the cgroup and its children
* `MemLow`: the low memory reservation of the cgroup
* `Peak/Low`: the ratio of the peak memory usage to the low memory reservation
* `MemHigh`: the high memory limit of the cgroup
* `MemMin`: the minimum memory reservation of the cgroup
* `Current/Min`: the ratio of the current memory usage to the minimum memory reservation
* `MemMax`: the maximum memory limit of the cgroup

### `swap`

```bash
$ talosctl cgroups --preset=swap
NAME                                                                          SwapCurrent   SwapPeak   SwapHigh   SwapMax
.                                                                                unset         unset      unset      unset
├──init                                                                            0 B           0 B        max        max
├──kubepods                                                                        0 B           0 B        max        max
│   ├──besteffort                                                                  0 B           0 B        max        max
│   │   └──kube-system/kube-proxy-6r5bz                                            0 B           0 B        max        max
│   │       ├──kube-proxy                                                          0 B           0 B        max        0 B
│   │       └──sandbox                                                             0 B           0 B        max        max
│   └──burstable                                                                   0 B           0 B        max        max
│       ├──kube-system/kube-apiserver-talos-default-controlplane-1                 0 B           0 B        max        max
│       │   ├──kube-apiserver                                                      0 B           0 B        max        0 B
│       │   └──sandbox                                                             0 B           0 B        max        max
│       ├──kube-system/kube-controller-manager-talos-default-controlplane-1        0 B           0 B        max        max
│       │   ├──kube-controller-manager                                             0 B           0 B        max        0 B
│       │   └──sandbox                                                             0 B           0 B        max        max
│       ├──kube-system/kube-flannel-jzx6m                                          0 B           0 B        max        max
│       │   ├──kube-flannel                                                        0 B           0 B        max        0 B
│       │   └──sandbox                                                             0 B           0 B        max        max
│       └──kube-system/kube-scheduler-talos-default-controlplane-1                 0 B           0 B        max        max
│           ├──kube-scheduler                                                      0 B           0 B        max        0 B
│           └──sandbox                                                             0 B           0 B        max        max
├──podruntime                                                                      0 B           0 B        max        max
│   ├──etcd                                                                        0 B           0 B        max        max
│   ├──kubelet                                                                     0 B           0 B        max        max
│   └──runtime                                                                     0 B           0 B        max        max
└──system                                                                          0 B           0 B        max        max
    ├──apid                                                                        0 B           0 B        max        max
    ├──dashboard                                                                   0 B           0 B        max        max
    ├──runtime                                                                     0 B           0 B        max        max
    ├──trustd                                                                      0 B           0 B        max        max
    └──udevd                                                                       0 B           0 B        max        max
```

In the swap view, the following columns are displayed:

* `SwapCurrent`: the current swap usage of the cgroup and its children
* `SwapPeak`: the peak swap usage of the cgroup and its children
* `SwapHigh`: the high swap limit of the cgroup
* `SwapMax`: the maximum swap limit of the cgroup

## Custom Schemas

The `talosctl cgroups` command allows you to define custom schemas to display the cgroups information in a specific way.
The schema is defined in a YAML file with the following structure:

```yaml
columns:
  - name: Bytes Read/Written
    template: '{{ range $disk, $v := .IOStat }}{{ if $v }}{{ $disk }}: {{ $v.rbytes.HumanizeIBytes }}/{{ $v.wbytes.HumanizeIBytes }} {{ end }}{{ end }}'
  - name: ios Read/Write
    template: '{{ if .Parent }}{{ range $disk, $v := .IOStat }}{{ $disk }}: {{ $v.rios }}/{{ $v.wios }} {{ end }}{{ end }}'
  - name: PressAvg10
    template: '{{ .IOPressure.some.avg10 | printf "%6s" }}'
  - name: PressAvg60
    template: '{{ .IOPressure.some.avg60 | printf "%6s" }}'
  - name: PressTotal
    template: '{{ .IOPressure.some.total.UsecToDuration | printf "%12s" }}'
```

The schema file can be passed to the `talosctl cgroups` command with the `--schema-file` flag:

```bash
talosctl cgroups --schema-file=schema.yaml
```

In the schema, for each column, you can define a `name` and a `template` which is a Go template that will be executed with the cgroups data.
In the template, there's a `.` variable that contains the cgroups data, and `.Parent` variable which is a parent cgroup (if available).
Each cgroup node contains information parsed from the cgroup filesystem, with field names matching the filenames adjusted for Go naming conventions,
e.g. `io.stat` becomes `.IOStat` in the template.

The schemas for the presets above can be found in the [source code](https://github.com/siderolabs/talos/tree/main/cmd/talosctl/cmd/talos/cgroupsprinter/schemas).
