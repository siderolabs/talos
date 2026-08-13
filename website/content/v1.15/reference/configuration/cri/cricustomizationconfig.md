---
description: CRICustomizationConfig configures the CRI containerd instance.
title: CRICustomizationConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CRICustomizationConfig
name: enable-metrics # Name of the CRI customization.
content: | # CRI containerd configuration fragment in TOML format.
    [metrics]
      address = "0.0.0.0:11234"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the CRI customization.<br><br>Customizations are merged with physical CRI configuration parts in<br>lexicographical order by name. The legacy<br>`/etc/cri/conf.d/20-customization.part` machine file is included under<br>the reserved name `customization`.<br><br>Applying, updating, or removing a customization restarts CRI automatically.  | |
|`content` |string |CRI containerd configuration fragment in TOML format.  | |






