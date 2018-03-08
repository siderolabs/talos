<p align="center">
  <h1 align="center">Dianemo</h1>
  <p align="center">An Immutable, secure, and minimal Kubernetes distribution.</p>
  <p align="center">
    <a href="https://gitter.im/autonomy/dianemo"><img alt="Gitter" src="https://img.shields.io/gitter/room/autonomy/dianemo.svg?style=flat-square"></a>
    <a href="https://github.com/autonomy/dianemo/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/autonomy/dianemo.svg?style=flat-square"></a>
    <a href="https://github.com/autonomy/dianemo/releases/latest"><img alt="GitHub (pre-)release" src="https://img.shields.io/github/release/autonomy/dianemo/all.svg?style=flat-square"></a>
  </p>
</p>

---

**Dianemo** is a Kubernetes distribution. Some of the key features are:

- **Immutable**: Utilizes a design that adheres to immutable infrastructure
  ideology.
- **Secure**: Zero host-level access reduces the attack surface of a Kubernetes
  cluster.
- **Minimal**: Only the binaries required by Kubernetes are installed.

## Developing Dianemo

The build of Dianemo depends on [conform](https://github.com/autonomy/conform):

```bash
go get -u github.com/autonomy/conform
```

> **Note:** Conform leverages [multi-stage builds](https://docs.docker.com/engine/userguide/eng-image/multistage-build/). Docker 17.05.0 or greater is required.

To build, simply run:

```bash
conform enforce
```

### License
[![license](https://img.shields.io/github/license/autonomy/dianemo.svg?style=flat-square)](https://github.com/autonomy/dianemo/blob/master/LICENSE)
