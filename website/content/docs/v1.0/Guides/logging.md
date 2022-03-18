---
title: Logging
---

## Viewing logs

Kernel messages can be retrieved with `talosctl dmesg` command:

```sh
$ talosctl -n 172.20.1.2 dmesg

172.20.1.2: kern:    info: [2021-11-10T10:09:37.662764956Z]: Command line: init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 random.trust_cpu=on printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 console=ttyS0 reboot=k panic=1 talos.shutdown=halt talos.platform=metal talos.config=http://172.20.1.1:40101/config.yaml
[...]
```

Service logs can be retrieved with `talosctl logs` command:

```sh
$ talosctl -n 172.20.1.2 services

NODE         SERVICE      STATE     HEALTH   LAST CHANGE   LAST EVENT
172.20.1.2   apid         Running   OK       19m27s ago    Health check successful
172.20.1.2   containerd   Running   OK       19m29s ago    Health check successful
172.20.1.2   cri          Running   OK       19m27s ago    Health check successful
172.20.1.2   etcd         Running   OK       19m22s ago    Health check successful
172.20.1.2   kubelet      Running   OK       19m20s ago    Health check successful
172.20.1.2   machined     Running   ?        19m30s ago    Service started as goroutine
172.20.1.2   trustd       Running   OK       19m27s ago    Health check successful
172.20.1.2   udevd        Running   OK       19m28s ago    Health check successful

$ talosctl -n 172.20.1.2 logs machined

172.20.1.2: [talos] task setupLogger (1/1): done, 106.109µs
172.20.1.2: [talos] phase logger (1/7): done, 564.476µs
[...]
```

Container logs for Kubernetes pods can be retrieved with `talosctl logs -k` command:

```sh
$ talosctl -n 172.20.1.2 containers -k
NODE         NAMESPACE   ID                                                 IMAGE                                                         PID    STATUS
172.20.1.2   k8s.io      kube-system/kube-flannel-dk6d5                     k8s.gcr.io/pause:3.5                                          1329   SANDBOX_READY
172.20.1.2   k8s.io      └─ kube-system/kube-flannel-dk6d5:install-cni      ghcr.io/talos-systems/install-cni:v0.7.0-alpha.0-1-g2bb2efc   0      CONTAINER_EXITED
172.20.1.2   k8s.io      └─ kube-system/kube-flannel-dk6d5:install-config   quay.io/coreos/flannel:v0.13.0                                0      CONTAINER_EXITED
172.20.1.2   k8s.io      └─ kube-system/kube-flannel-dk6d5:kube-flannel     quay.io/coreos/flannel:v0.13.0                                1610   CONTAINER_RUNNING
172.20.1.2   k8s.io      kube-system/kube-proxy-gfkqj                       k8s.gcr.io/pause:3.5                                          1311   SANDBOX_READY
172.20.1.2   k8s.io      └─ kube-system/kube-proxy-gfkqj:kube-proxy         k8s.gcr.io/kube-proxy:v1.23.0                                 1379   CONTAINER_RUNNING

$ talosctl -n 172.20.1.2 logs -k kube-system/kube-proxy-gfkqj:kube-proxy
172.20.1.2: 2021-11-30T19:13:20.567825192Z stderr F I1130 19:13:20.567737       1 server_others.go:138] "Detected node IP" address="172.20.0.3"
172.20.1.2: 2021-11-30T19:13:20.599684397Z stderr F I1130 19:13:20.599613       1 server_others.go:206] "Using iptables Proxier"
[...]
```

## Sending logs

### Service logs

You can enable logs sendings in machine configuration:

```yaml
machine:
  logging:
    destinations:
      - endpoint: "udp://127.0.0.1:12345/"
        format: "json_lines"
      - endpoint: "tcp://host:5044/"
        format: "json_lines"
```

Several destinations can be specified.
Supported protocols are UDP and TCP.
The only currently supported format is `json_lines`:

```json
{
  "msg": "[talos] apply config request: immediate true, on reboot false",
  "talos-level": "info",
  "talos-service": "machined",
  "talos-time": "2021-11-10T10:48:49.294858021Z"
}
```

Messages are newline-separated when sent over TCP.
Over UDP messages are sent with one message per packet.
`msg`, `talos-level`, `talos-service`, and `talos-time` fields are always present; there may be additional fields.

### Kernel logs

Kernel log delivery can be enabled with the `talos.logging.kernel` kernel command line argument, which can be specified
in the `.machine.installer.extraKernelArgs`:

```yaml
machine:
  install:
    extraKernelArgs:
      - talos.logging.kernel=tcp://host:5044/
```

Kernel log destination is specified in the same way as service log endpoint.
The only supported format is `json_lines`.

Sample message:

```json
{
  "clock":6252819, // time relative to the kernel boot time
  "facility":"user",
  "msg":"[talos] task startAllServices (1/1): waiting for 6 services\n",
  "priority":"warning",
  "seq":711,
  "talos-level":"warn", // Talos-translated `priority` into common logging level
  "talos-time":"2021-11-26T16:53:21.3258698Z" // Talos-translated `clock` using current time
}
```

### Filebeat example

To forward logs to other Log collection services, one way to do this is sending
them to a [Filebeat](https://www.elastic.co/beats/filebeat) running in the
cluster itself (in the host network), which takes care of forwarding it to
other endpoints (and the necessary transformations).

If [Elastic Cloud on Kubernetes](https://www.elastic.co/elastic-cloud-kubernetes)
is being used, the following Beat (custom resource) configuration might be
helpful:

```yaml
apiVersion: beat.k8s.elastic.co/v1beta1
kind: Beat
metadata:
  name: talos
spec:
  type: filebeat
  version: 7.15.1
  elasticsearchRef:
    name: talos
  config:
    filebeat.inputs:
      - type: "udp"
        host: "127.0.0.1:12345"
        processors:
          - decode_json_fields:
              fields: ["message"]
              target: ""
          - timestamp:
              field: "talos-time"
              layouts:
                - "2006-01-02T15:04:05.999999999Z07:00"
          - drop_fields:
              fields: ["message", "talos-time"]
          - rename:
              fields:
                - from: "msg"
                  to: "message"

  daemonSet:
    updateStrategy:
      rollingUpdate:
        maxUnavailable: 100%
    podTemplate:
      spec:
        dnsPolicy: ClusterFirstWithHostNet
        hostNetwork: true
        securityContext:
          runAsUser: 0
        containers:
          - name: filebeat
            ports:
              - protocol: UDP
                containerPort: 12345
                hostPort: 12345
```

The input configuration ensures that messages and timestamps are extracted properly.
Refer to the Filebeat documentation on how to forward logs to other outputs.

Also note the `hostNetwork: true` in the `daemonSet` configuration.

This ensures filebeat uses the host network, and listens on `127.0.0.1:12345`
(UDP) on every machine, which can then be specified as a logging endpoint in
the machine configuration.
