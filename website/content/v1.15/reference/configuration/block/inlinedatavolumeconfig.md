---
description: |
    InlineDataVolumeConfig is an inline data volume configuration document.
    Inline data volume is a set of files defined inline, similar to a Kubernetes ConfigMap.
    The files are kept in memory (`tmpfs`), which only Talos can write to.
    Containers mount it read-only with a `volume` mount in `ContainerConfig`.
    The name must not be taken by a user, existing or external volume.

    Changes to the files are applied in place, without restarting containers:
    the files are replaced atomically as a whole, by swapping the `..data` symbolic link
    to a directory holding the new set of files.
    Each top-level entry of the volume is a symbolic link into `..data`.
title: InlineDataVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: InlineDataVolumeConfig
name: app-config # Name of the volume.
# Files of the volume, keyed by the path relative to the volume root.
files:
    app.yaml:
        contents: | # The contents of the file.
            listen: :8080
    conf.d/extra.ini:
        mode: 0o600 # The file's permissions in octal.
        contents: | # The contents of the file.
            [section]
            key=value
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the volume.<br><br>Name might be between 1 and 34 characters long and can only contain:<br>lowercase and uppercase ASCII letters, digits, and hyphens.  | |
|`secret` |bool |Redacts file paths and contents when the machine configuration is redacted.<br>Files are still rendered in full.<br><br>Defaults to `false`.  | |
|`files` |<a href="#InlineDataVolumeConfig.files.-">map[string]InlineDataVolumeFileV1Alpha1</a> |Files of the volume, keyed by the path relative to the volume root.<br><br>Paths may contain subdirectories, which are created automatically.<br>Path components must not start with `..`, and must be at most 255 bytes long.<br>Paths must be at most 1024 bytes long.<br>There may be at most 256 files in at most 256 distinct directories,<br>and their total size, each rounded up to 4 KiB, must not exceed 1 MiB.  | |




## files.* {#InlineDataVolumeConfig.files.-}

InlineDataVolumeFileV1Alpha1 describes a single file of an inline data volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`mode` |InlineDataVolumeFileMode |The file's permissions in octal.<br><br>Defaults to `0o644`.  | |
|`contents` |string |The contents of the file.  | |








