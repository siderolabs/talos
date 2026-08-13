---
description: EtcFileConfig configures a user-managed file under /etc.
title: EtcFileConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EtcFileConfig
name: nfsmount.conf # Path of the file relative to `/etc`.
mode: 0o644 # The file's permissions in octal.
contents: | # The contents of the file.
    [NFSMount_Global_Options]
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Path of the file relative to `/etc`.  | |
|`mode` |EtcFileMode |The file's permissions in octal.  | |
|`contents` |string |The contents of the file.  | |






