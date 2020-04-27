# Project Layout Standards

In this document we will cover our official layout standards.
You will find that the project is a monolithic repository that contains everything required to build and run Talos.

```bash
$ tree .
.
├── api
├── cmd
├── docs
├── hack
├── internal
│   ├── app
│   │   ├── app1
│   │   │   ├── internal
│   │   │   ├── pkg
│   │   │   └── proto
│   │   └── app2
│   │       ├── internal
│   │       ├── pkg
│   │       └── proto
│   └── pkg
└── pkg
```

## Internal Applications

Talos is comprised of applications designed to handle the various domains of an operating system.
The following requirements must be adhered to by an `app`:

- anything ran as a service, that is maintained by us, should live under `internal/app`
- each `app` is allowed at most 1 level of an `internal` package

## Internal Packages

There are a number of packages we will need to maintain that are strongly coupled with Talos business logic.
These package should be housed within the `internal/pkg` directory.
The criteria for deciding if a package should be housed here are as follows:

- code that is a high level abstraction specific to the internals of Talos
- code that should not be exposed publicly

## Public Packages

In building higher level abstractions, we should strive to create generic, general use packages that can be used independent of Talos.
The following rules apply to public packages:

- a `pkg` should _never_ contain `internal` code

### Graduation Criteria

In deciding if a package should be moved to an external repository, the following should be taken into consideration:

- are there requests for exposing the package?
- are there people willing to maintain the package?
