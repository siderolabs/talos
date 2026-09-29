---
description: |
    ContentLibraryConfig is a content library configuration document.
    ContentLibraryConfig declares a content library: a store for virtual machine images
    kept on a volume.

    The library is the root of the backing volume, so a library backed by the user volume
    `vm-images` keeps its images directly in `/var/mnt/vm-images`. A volume backs exactly
    one library: two libraries pointed at the same volume are rejected.

    Contents are managed over the API with `talosctl hv content-library`, never by editing
    the machine configuration. Status is reported via `ContentLibraryStatus`.
title: ContentLibraryConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ContentLibraryConfig
name: my-vm-images-1 # Name of the content library.
# Volume backing the content library.
backing:
    volume: vm-images # Name of the volume storing the library's contents.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the content library.<br><br>Must be between 1 and 63 characters long, and can only contain ASCII letters,<br>digits and hyphens. It is the ID used to address the library over the API.  | |
|`backing` |<a href="#ContentLibraryConfig.backing">ContentLibraryBacking</a> |Volume backing the content library.  | |




## backing {#ContentLibraryConfig.backing}

ContentLibraryBacking describes the storage backing a content library.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volume` |string |Name of the volume storing the library's contents.<br><br>The name of a `UserVolumeConfig`, `ExistingVolumeConfig` or `ExternalVolumeConfig`<br>document, as that document declares it, not the `u-`/`e-`/`x-` prefixed ID Talos<br>derives from the kind and the name. Volume names are unique across those kinds, so<br>the name resolves to one volume.<br><br>The volume is declared separately and is not provisioned by this document; the<br>library becomes ready only once the volume is mounted. Uploads write into the<br>library, so a volume declared read-only cannot back one. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
volume: vm-images
{{< /highlight >}}</details> | |








