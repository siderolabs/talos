---
title: "Machine Configuration OAuth2 Authentication"
description: "How to authenticate Talos machine configuration download (`talos.config=`) on `metal` platform using OAuth."
---

Talos Linux when running on the `metal` platform can be configured to authenticate the machine configuration download using OAuth2 device flow.
The machine configuration is fetched from the URL specified with `talos.config` kernel argument, and by default this HTTP request is not authenticated.
When the OAuth2 authentication is enabled, Talos will authenticate the request using OAuth device flow first, and then pass the token to the machine configuration download endpoint.

## Prerequisites

Obtain the following information:

* OAuth client ID (mandatory)
* OAuth client secret (optional)
* OAuth device endpoint
* OAuth token endpoint
* OAuth scopes, audience (optional)
* OAuth client secret (optional)
* extra Talos variables to send to the device auth endpoint (optional)

## Configuration

Set the following kernel parameters on the initial Talos boot to enable the OAuth flow:

* `talos.config` set to the URL of the machine configuration endpoint (which will be authenticated using OAuth)
* `talos.config.oauth.client_id` set to the OAuth client ID (required)
* `talos.config.oauth.client_secret` set to the OAuth client secret (optional)
* `talos.config.oauth.scope` set to the OAuth scopes (optional, repeat the parameter for multiple scopes)
* `talos.config.oauth.audience` set to the OAuth audience (optional)
* `talos.config.oauth.device_auth_url` set to the OAuth device endpoint (if not set defaults to `talos.config` URL with the path `/device/code`)
* `talos.config.oauth.token_url` set to the OAuth token endpoint (if not set defaults to `talos.config` URL with the path `/token`)
* `talos.config.oauth.extra_variable` set to the extra Talos variables to send to the device auth endpoint (optional, repeat the parameter for multiple variables)

The list of variables supported by the `talos.config.oauth.extra_variable` parameter is same as the [list of variables]({{< relref "../reference/kernel#talosconfig" >}}) supported by the `talos.config` parameter.

## Flow

On the initial Talos boot, when machine configuration is not available, Talos will print the following messages:

```text
[talos] downloading config {"component": "controller-runtime", "controller": "config.AcquireController", "platform": "metal"}
[talos] waiting for network to be ready
[talos] [OAuth] starting the authentication device flow with the following settings:
[talos] [OAuth]  - client ID: "<REDACTED>"
[talos] [OAuth]  - device auth URL: "https://oauth2.googleapis.com/device/code"
[talos] [OAuth]  - token URL: "https://oauth2.googleapis.com/token"
[talos] [OAuth]  - extra variables: ["uuid" "mac"]
[talos] waiting for variables: [uuid mac]
[talos] waiting for variables: [mac]
[talos] [OAuth] please visit the URL https://www.google.com/device and enter the code <REDACTED>
[talos] [OAuth] waiting for the device to be authorized (expires at 14:46:55)...
```

If the OAuth service provides the complete verification URL, the QR code to scan is also printed to the console:

```text
[talos] [OAuth] or scan the following QR code:
█████████████████████████████████
█████████████████████████████████
████ ▄▄▄▄▄ ██▄▀▀    ▀█ ▄▄▄▄▄ ████
████ █   █ █▄  ▀▄██▄██ █   █ ████
████ █▄▄▄█ ██▀▄██▄  ▀█ █▄▄▄█ ████
████▄▄▄▄▄▄▄█ ▀ █ ▀ █▄█▄▄▄▄▄▄▄████
████   ▀ ▄▄ ▄█  ██▄█   ███▄█▀████
████▀█▄  ▄▄▀▄▄█▀█▄██ ▄▀▄██▄ ▄████
████▄██▀█▄▄▄███▀ ▀█▄▄  ██ █▄ ████
████▄▀▄▄▄ ▄███ ▄ ▀ ▀▀▄▀▄▀█▄ ▄████
████▄█████▄█  █ ██ ▀ ▄▄▄  █▀▀████
████ ▄▄▄▄▄ █ █ ▀█▄█▄ █▄█  █▄ ████
████ █   █ █▄ ▄▀ ▀█▀▄▄▄   ▀█▄████
████ █▄▄▄█ █ ██▄ ▀  ▀███ ▀█▀▄████
████▄▄▄▄▄▄▄█▄▄█▄██▄▄▄▄█▄███▄▄████
█████████████████████████████████
```

Once the authentication flow is complete on the OAuth provider side, Talos will print the following message:

```text
[talos] [OAuth] device authorized
[talos] fetching machine config from: "http://example.com/config.yaml"
[talos] machine config loaded successfully {"component": "controller-runtime", "controller": "config.AcquireController", "sources": ["metal"]}
```
