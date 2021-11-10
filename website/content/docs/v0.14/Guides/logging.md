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

## Sending logs

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

### Filebeat example

Talos logs can be sent to [Filebeat](https://www.elastic.co/beats/filebeat).
If [Elastic Cloud on Kubernetes](https://www.elastic.co/elastic-cloud-kubernetes) is being used, the following Beat (custom resource) configuration might be helpful:

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

That input configuration ensures that messages and timestamps are extracted properly.
In `daemonSet` configuration, make sure that the host network is being used and that the port is exposed.
