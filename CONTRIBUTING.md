# Contributing

First of all, thank you!
We value your time and interest in making Talos a successful open source project.

## What can I do to help?

There are a number of ways you can help!
We are in need of both technical and non-technical contributions.
Even just mentioning the project to a friend, colleague, or anyone else for that matter, would be a huge help.
We need writers, bloggers, engineers, graphics designers — you name it, we need it.

## Guidelines

Let's talk about some of the guidelines we have when making a contribution to Talos.

### Git Commits

You probably noticed we use have a funny way of writing commit messages.
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

In addition, all commits should be signed by the committer using `git commit -s` which should produce a commit
message with `Signed-off-by: Your Name <your@email>`. It is not necessary to cryptographically sign commits
with GPG.

### Pull Requests

To avoid multiples CI runs, please ensure that you are running a full build before submitting your PR, and
branches should be squashed to a single commit.

## Developing

To start developing for Talos you will need at least `GNU Make` and `Golang` v1.11.

From there you can bootstrap the rest of the toolchain and install Golang dependencies with the following commands:

```bash
GO111MODULE=on go get
make ci
```

# Make Targets

In the `Makefile` there are a variety of targets, the most common are:

* `kernel` creates the `vmlinuz` Linux kernel executable.
* `initamfs` creates the `initramfs.xz` initial RAMdisk filesystem.
* `image-vanilla` creates the `image.raw` file that can be used as a image volume for VMs.
* `osctl-linux-amd64` and `osctl-darwin-amd64` make the `osctl` CLI tool for Linux & OSX respectivly.
* `rootfs` creates an archive of the root filesystem preloaded with all the components needed to launch Talos & Kubernetes.

# Buildkit

Talos uses Moby [buildkit](https://github.com/moby/buildkit) for concurrent and cache-efficient builds.
By default, a buildkit service is started locally, but if you want to offload the builds to another server,
you can start a buildkit service with the following command:

```bash
docker run --detach --privileged --restart always --publish 1234:1234 moby/buildkit --addr tcp://0.0.0.0:1234
```

Then using the `BUILDKIT_HOST` environment variable before running any `make` target, E.G.

```bash
BUILDKIT_HOST=tcp://192.168.1.50:1234 make initramfs
```
