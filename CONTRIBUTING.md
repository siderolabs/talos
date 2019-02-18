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
cat <EOF | tee .git/hooks/commit-msg
#!/bin/sh

conform enforce --commit-msg-file $1
EOF
```

### Pull Requests

To avoid multiples CI runs, please ensure that you are running a full build before submitting your PR.

