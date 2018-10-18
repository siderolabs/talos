<p align="center">
  <h1 align="center">Dianemo</h1>
  <p align="center">A modern Linux distribution for Kubernetes.</p>
  <p align="center">
    <a href="https://github.com/autonomy/dianemo/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/autonomy/dianemo.svg?style=flat-square"></a>
    <a href="https://github.com/autonomy/dianemo/releases/latest"><img alt="GitHub (pre-)release" src="https://img.shields.io/github/release/autonomy/dianemo/all.svg?style=flat-square"></a>
  </p>
</p>

---

**Dianemo** was designed to be secure, immutable, and minimal, providing the following benefits:

- **Security**: Reduce your attack surface by practicing the Principle of Least Privilege (PoLP) and enforcing mutual TLS (mTLS).
- **Predictability**: Remove needless variables and reduce unknown factors from your environment using immutable infrastructure.
- **Evolvability**: Simplify and increase your ability to easily accommodate future changes to your architecture.

## Developing Dianemo

Install [conform](https://github.com/autonomy/conform):

```bash
go get -u github.com/autonomy/conform
```

Start the build:

```bash
conform build
```

## License

[![license](https://img.shields.io/github/license/autonomy/dianemo.svg?style=flat-square)](https://github.com/autonomy/dianemo/blob/master/LICENSE)
