
---
title: "Docker Quickstart"
linkTitle: "Docker Quickstart"
weight: 3
date: 2018-07-30
description: >
  This page helps you to setup and run a local Docsy site with Docker in 5 minutes. 
---

## Install the prerequisites

1. On Mac and Windows, download and install [Docker
   Desktop](https://www.docker.com/get-started).  On Linux, install [Docker
   engine](https://docs.docker.com/engine/install/#server) and [Docker
   compose](https://docs.docker.com/compose/install/).
   
   The installation may require you to reboot your computer for the changes to
   take effect. 

1. [Install git](https://github.com/git-guides/install-git).

## Create your repository from the docsy-example template

The docsy-example repository provides a basic site structure that you can use
as starting point to create your own documentation.

1. Use the [docsy-example template](https://github.com/google/docsy-example)
   to [create your own repository](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-from-a-template).

1. Download the code to your local machine by [cloning your newly created
   repository](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/cloning-a-repository).

1. Change your working directory to the newly created folder:

   ```bash
   cd docsy-example
   ```

## Build and run the container

The docsy-example repository includes a
[Dockerfile](https://docs.docker.com/engine/reference/builder/) that you can
use to run your site.

1. Build the docker image:

   ```bash
   docker-compose build
   ```

1. Run the built image:

   ```bash
   docker-compose up
   ```

1. Open the address `http://localhost:1313` in your web browser to load the
   docsy-example homepage. You can now make changes to the source files, those
   changes will be live-reloaded in your browser.

## Cleanup

To cleanup your system and delete the container image follow the next steps.

1. Stop Docker Compose with **Ctrl + C**.

1. Remove the produced images

   ```bash
   docker-compose rm
   ```

## What's next?

* Learn about [basic setup and configurations for Docsy](/docs/getting-started/).
* [Add content and customize your site](/docs/adding-content/)
* [Publish your site](/docs/deployment/).
