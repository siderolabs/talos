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
|`selector` |<a href="#LinkAliasConfig.selector">LinkSelector</a> |Selector to match the link to alias.<br><br>By default, the selector must match exactly one link, otherwise the alias is not applied.<br>Set `requireUniqueMatch` to `false` to allow multiple matches and use the first matching link.<br>If multiple selectors match the same link, the first one is used.  | |




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
|`requireUniqueMatch` |bool |Require the selector to match exactly one link.<br><br>When set to `false`, if multiple links match the selector, the first matching link is used.<br>When set to `true` (default), if multiple links match, the alias is not applied.  | |
|`skipAliasedLinks` |bool |Skip links that already have an alias assigned by a previous LinkAliasConfig.<br><br>This allows creating sequential aliases like `net0` and `net1` from any N links<br>by using the same broad selector and relying on processing order.  | |








