---
description: LinkAliasConfig is a config document to alias (give a different name) to a physical link.
title: LinkAliasConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: LinkAliasConfig
name: int0 # Alias for the link.
# Selector to match the link to alias.
selector:
    match: glob("00:1a:2b:*", mac(link.permanent_addr)) # The Common Expression Language (CEL) expression to match the link.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Alias for the link.<br><br>Don't use system interface names like "eth0", "ens3", "enp0s2", etc. as those may conflict<br>with existing physical interfaces. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: net0
{{< /highlight >}}{{< highlight yaml >}}
name: private
{{< /highlight >}}</details> | |
|`selector` |<a href="#LinkAliasConfig.selector">LinkSelector</a> |Selector to match the link to alias.<br><br>Selector must match exactly one link, otherwise an error is returned.<br>If multiple selectors match the same link, the first one is used.  | |




## selector {#LinkAliasConfig.selector}

LinkSelector selects a link to alias.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the link. <details><summary>Show example(s)</summary>match links with a specific MAC address:{{< highlight yaml >}}
match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"
{{< /highlight >}}match links by MAC address prefix:{{< highlight yaml >}}
match: glob("00:1a:2b:*", mac(link.permanent_addr))
{{< /highlight >}}match links by driver name:{{< highlight yaml >}}
match: link.driver == "e1000"
{{< /highlight >}}</details> | |








