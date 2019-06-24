---
title: "AWS"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  docs:
    parent: 'guides'
---

First, create the AMI:

```bash
docker run \
    --rm \
    --volume $HOME/.aws/credentials:/root/.aws/credentials \
    --env AWS_DEFAULT_PROFILE=${PROFILE} \
    --env AWS_DEFAULT_REGION=${REGION} \
    talos-systems/talos:latest ami -var regions=${COMMA_SEPARATED_LIST_OF_REGIONS}
```

Once the AMI is created, you can now start an EC2 instance using the AMI ID.
Provide the proper configuration as the instance's user data.

> An official Terraform module is currently being developed, stay tuned!
