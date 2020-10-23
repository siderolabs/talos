---
title: "Customizing the Kernel"
description: ""
---

```docker
FROM scratch AS customization
COPY --from=<custom kernel image> /lib/modules /lib/modules

FROM docker.io/andrewrynhard/installer:latest
COPY --from=<custom kernel image> /boot/vmlinuz /usr/install/vmlinuz
```

```bash
docker build --build-arg RM="/lib/modules" -t talos-installer .
```

> Note: You can use the `--squash` flag to create smaller images.

Now that we have a custom installer we can build Talos for the specific platform we wish to deploy to.
