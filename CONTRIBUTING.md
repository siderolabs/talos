# Contributing

First of all, thank you!
We value your time and interest in making Talos a successful open source project.

## What We Need

There are a number of ways you can help!
We are in need of both technical and non-technical contributions.
Even just mentioning the project to a friend, colleague, or anyone else for that matter, would be a huge help.
We need writers, bloggers, engineers, graphics designers — you name it, we need it.

## Guidelines

Let's talk about some of the guidelines we have when making a contribution to Talos.

### Git Commits

You probably noticed we use a funny way of writing commit messages.
Indeed we do, but its based on a specification called [Conventional Commits](https://www.conventionalcommits.org).
Don't worry, it won't be _too_ much of hassle.
We have a small tool that you can use to remind you of our policy.

```bash
go get github.com/autonomy/conform
cat <<EOF | tee .git/hooks/commit-msg
#!/bin/sh

conform enforce --commit-msg-file \$1
EOF
chmod +x .git/hooks/commit-msg
```

In addition, all commits should be signed by the committer using `git commit -s`, which should produce a commit message with `Signed-off-by: Your Name <your@email>`.
It is not necessary to cryptographically sign commits with GPG.

### Pull Requests

To avoid multiples CI runs, please ensure that you are running a full build before submitting your PR, and branches should be squashed to a single commit.
To run some local tests you can use the included Makefile in this repo.
For example to run the conformance tests:

```bash
$ make conformance
docker run --rm -it -v /Users/user/projects/talos:/src -w /src docker.io/autonomy/conform:v0.1.0-alpha.19
POLICY         CHECK                        STATUS        MESSAGE
commit         Header Length                PASS          <none>
commit         Imperative Mood              PASS          <none>
commit         Header Case                  PASS          <none>
commit         Header Last Character        PASS          <none>
commit         DCO                          PASS          <none>
commit         Conventional Commit          PASS          <none>
commit         Spellcheck                   PASS          <none>
commit         Number of Commits            PASS          <none>
commit         Commit Body                  PASS          <none>
license        File Header                  PASS          <none>
```

Make sure all tests pass before creating a PR.

## Developing

To start developing for Talos you will need at least `GNU Make` and `Golang` v1.11.

From there you can bootstrap the rest of the toolchain and install Golang dependencies with the following commands:

```bash
GO111MODULE=on go get
make ci
```

## Make Targets

In the `Makefile` there are a variety of targets, the most common are:

- `kernel` creates the `vmlinuz` Linux kernel executable.
- `initamfs` creates the `initramfs.xz` initial RAMdisk filesystem.
- `image-vanilla` creates the `image.raw` file that can be used as a image volume for VMs.
- `talosctl-linux-amd64` and `talosctl-darwin-amd64` make the `talosctl` CLI tool for Linux & OSX respectively.
- `rootfs` creates an archive of the root filesystem preloaded with all the components needed to launch Talos & Kubernetes.

## Buildkit

Talos uses Moby [buildkit](https://github.com/moby/buildkit) for concurrent and cache-efficient builds.
By default, a buildkit service is started locally, but if you want to offload the builds to another server, you can start a buildkit service with the following command:

```bash
docker run --detach --privileged --restart always --publish 1234:1234 moby/buildkit --addr tcp://0.0.0.0:1234
```

Then using the `BUILDKIT_HOST` environment variable before running any `make` target, E.G.

```bash
BUILDKIT_HOST=tcp://192.168.1.50:1234 make initramfs
```

### Docker for mac

To enable building buildX on Docker for Mac you need to enable the experimental features in the docker app.
To enable this go to: Docker -> preferences -> Command Line -> "Enable experimental features" should be toggled on.

## VScode extensions

Visual studio code is a editor which is widely used, and has some neat features to make your life easier.
Below is a list of extensions that can help while developing for Talos.

- [Markdown lint](https://marketplace.visualstudio.com/items/DavidAnson.vscode-markdownlint)
- [Golang support](https://marketplace.visualstudio.com/items?itemName=golang.Go)
