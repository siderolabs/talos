# Docsy 

Docsy is a [Hugo](https://gohugo.io/) theme for technical documentation sets, providing simple navigation, site structure, and more.

This is not an officially supported Google product. This project is actively being maintained.

## Prerequisites

The following are basic prerequisites for using Docsy in your site:

- Install a recent release of the Hugo "extended" version (we recommend version 0.53 or later). If you install from the 
  [release page](https://github.com/gohugoio/hugo/releases), make sure you download the `_extended` version 
  which supports SCSS.

- Install `PostCSS` so that the site build can create the final CSS assets. You can install it locally by running 
  the following commands from the root directory of your project:

  ```
  sudo npm install -D --save autoprefixer
  sudo npm install -D --save postcss-cli
  ```

## Example and usage

You can find an example project that uses Docsy in the [Docsy Example Project repo](https://github.com/google/docsy-example).The Docsy Example Project is hosted at [https://example.docsy.dev/](https://example.docsy.dev/). For real-life examples of sites that use Docsy (and their source repos), see our [Examples](https://www.docsy.dev/docs/examples/) page.

To use the Docsy theme for your own site:

  - (Recommended) Use the [example project](https://github.com/google/docsy-example),
     which includes the Docsy theme as a submodule, as a template to create your project.
    You can customize this pre-configured basic site into your own Docsy themed site. 
    [Learn more...](https://github.com/google/docsy-example)
  
  - Add Docsy to your existing Hugo site repo's `themes` directory. You can either add Docsy as a Git submodule, or 
    clone the Docsy theme into your project.

See the [Docsy Getting Started Guide](https://docsy.dev/docs/getting-started/) for 
details about the various usage options.

## Documentation

Docsy has its own user guide (using Docsy, of course!) with lots more information about using the theme, which you can find at [https://docsy.dev/](https://docsy.dev/). Alternatively you can use Hugo to generate and serve a local copy of the guide (also useful for testing local theme changes), making sure you have installed all the prerequisites listed above:

```
git clone --recurse-submodules --depth 1 https://github.com/google/docsy.git
cd docsy/userguide/
hugo server --themesDir ../..
```

Note: you need the `themesDir` flag when running Hugo because the site files are inside the theme repo.

## Contributing ![GitHub](https://img.shields.io/github/contributors/google/docsy)

Please read [CONTRIBUTING.md](https://github.com/google/docsy/blob/master/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.
See also the list of [contributors](https://github.com/google/docsy/graphs/contributors) who participated in this project.

## License ![GitHub](https://img.shields.io/github/license/google/docsy)

This project is licensed under the Apache License 2.0 - see the [LICENSE.md](https://github.com/google/docsy/blob/master/LICENSE) file for details
