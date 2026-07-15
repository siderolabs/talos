---
description: ExtensionServiceConfig is a extensionserviceconfig document.
title: ExtensionServiceConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: nut-client # Name of the extension service.
# The config files for the extension service.
configFiles:
    - content: MONITOR ${upsmonHost} 1 remote username password # The content of the extension service config file.
      mountPath: /usr/local/etc/nut/upsmon.conf # The mount path of the extension service config file.
# The environment for the extension service.
environment:
    - NUT_UPS=upsname
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the extension service.  | |
|`configFiles` |<a href="#ExtensionServiceConfig.configFiles.">[]ConfigFile</a> |The config files for the extension service.  | |
|`environment` |[]string |The environment for the extension service.  | |




## configFiles[] {#ExtensionServiceConfig.configFiles.}

ConfigFile is a config file for extension services.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`content` |string |The content of the extension service config file.  | |
|`mountPath` |string |The mount path of the extension service config file.  | |








