---
description: KmsgLogConfig is a event sink config document.
title: KmsgLogConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KmsgLogConfig
name: remote-log # Name of the config document.
url: tcp://192.168.3.7:3478/ # The URL encodes the log destination.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`url` |URL |The URL encodes the log destination.<br>The scheme must be tcp:// or udp://.<br>The path must be empty.<br>The port is required. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
url: udp://10.3.7.3:2810
{{< /highlight >}}</details> | |






