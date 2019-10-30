const watch = require('node-watch')
const frontmatter = require('front-matter')
const path = require('path')
const fs = require('fs-extra')
const glob = require('glob')
const config = require('./docgen.config')
const marked = require('marked')
const prism = require('prismjs')
const loadLanguages = require('prismjs/components/')

marked.setOptions({
  highlight(code, lang) {
    loadLanguages([lang])

    return prism.highlight(code, prism.languages[lang], lang)
  }
})

const args = process.argv
  .slice(2)
  .map((arg) => arg.split('='))
  .reduce((args, [value, key]) => {
    args[value] = key
    return args
  }, {})

const Docgen = {
  sections: {},

  init: () => {
    Docgen.generateRoutes()
    Docgen.coldstart()
    if (!!args.watch) {
      watch(
        config.contentInputFolder,
        { filter: /\.md$/, recursive: true },
        Docgen.handleFileEvent
      )
      watch(
        config.contentInputFolder,
        { filter: /\.json$/, recursive: false },
        Docgen.handleMenuEvent
      )
    }
  },

  /**
   * Handles all node-watch file events (remove, update)
   * @param {string} event - node-watch event type; eg. 'remove' || 'change'
   * @param {string} contentFilePath - path to file that triggered that event
   */
  handleFileEvent: (event, contentFilePath) => {
    switch (event) {
      case 'remove':
        // contentFilePath = /my_absolute_file/content/content-delivery/en/topics/introduction.md
        // contentPath = content-delivery/en/topics/introduction
        const contentPath = contentFilePath
          .replace(config.contentInputFolder, '')
          .replace(path.parse(contentFilePath).ext, '')
        // [ content-delivery, en, topics, introduction ]
        const contentPathParts = contentPath.replace(/\\/g, '/').split('/')
        // content-delivery
        const version = contentPathParts.shift()
        // en
        const lang = contentPathParts.shift()

        delete Docgen.sections[version][lang][contentPath]

        Docgen.generate(version, lang)
        break
      default:
        const section = Docgen.load(contentFilePath)
        if (section.version == null || section.lang == null) {
          break
        }
        Docgen.generate(section.version, section.lang)
        break
    }
  },

  /**
   * Handles all node-watch file events (remove, update)
   * @param {string} event - node-watch event type; eg. 'remove' || 'change'
   * @param {string} contentFilePath - path to file that triggered that event
   */
  handleMenuEvent: (event, contentFilePath) => {
    switch (event) {
      case 'remove':
        // ignore
        return
        break
      default:
        const contentPathParts = contentFilePath
          .replace(config.contentInputFolder, '')
          .split('.')

        const version = contentPathParts.slice(0, -2).join('.')

        const lang = contentPathParts.slice(-2)[0]

        Docgen.exportMenu(version, lang)

        break
    }
  },

  /**
   * Iterates through all markdown files, loads their content
   * and generates section JSONs after preparation
   */
  coldstart: () => {
    glob(`${config.contentInputFolder}**/*.md`, (err, files) => {
      if (err) throw err

      files.forEach((contentFilePath) => {
        Docgen.load(contentFilePath)
      })

      Docgen.generateAll()
    })
  },

  /**
   * Iterate through all versions and languages to trigger
   * the generate for each content file.
   */
  generateAll: () => {
    // content-delivery, ...
    Docgen.listFoldersInFolder(config.contentInputFolder).forEach((version) => {
      // en, ...
      Docgen.listFoldersInFolder(config.contentInputFolder + version).forEach(
        (lang) => {
          // generate sections json from one language and version
          Docgen.generate(version, lang)
        }
      )
    })
  },

  /**
   * Generates sections JSON for one version and language combination
   * @param {string} version - first level of content folder, eg.: content-delivery, managmenet
   * @param {string} lang - second level of content folder, eg.: en, de, es, it, ...
   */
  generate: (version, lang) => {
    // order sections for one language and version
    Docgen.exportSections(Docgen.sections[version][lang], version, lang)

    // copies menu to static
    Docgen.exportMenu(version, lang)
  },

  /**
   * Exports the generated menu as JSON depending on version and language
   * @param {string} version - first level of content folder, eg.: content-delivery, managmenet
   * @param {string} lang - second level of content folder, eg.: en, de, es, it, ...
   */
  exportMenu: (version, lang) => {
    fs.copySync(
      config.menuInputFile
        .replace('{version}', version)
        .replace('{lang}', lang),
      config.menuOutputFile
        .replace('{version}', version)
        .replace('{lang}', lang)
    )
  },

  /**
   * Exports the sections as JSON depending on version and language
   * @param {Array} sections - Array of section objects
   * @param {string} version - first level of content folder, eg.: content-delivery, managmenet
   * @param {string} lang - second level of content folder, eg.: en, de, es, it, ...
   */
  exportSections: (sections, version, lang) => {
    return fs.writeFileSync(
      config.sectionsOutputFile
        .replace('{version}', version)
        .replace('{lang}', lang),
      JSON.stringify(sections)
    )
  },

  /**
   * Loads one file into Docgen.sections
   * @param {string} contentFilePath - Absolute path to Content Source File, will be a *.md file containing frontmatter.
   * @returns {Object} section - Object containing parsed markdown and additional information
   */
  load: (contentFilePath) => {
    const content = fs.readFileSync(contentFilePath, { encoding: 'utf8' })

    const frontmatterContent = frontmatter(content)

    const title = marked(frontmatterContent.attributes.title || '')
      .replace('<p>', '')
      .replace('</p>\n', '')

    const markdownContent = marked(frontmatterContent.body)

    // contentFilePath = /my_absolute_file/content/content-delivery/en/topics/introduction.md

    // contentPath = content-delivery/en/topics/introduction
    let contentPath = contentFilePath
      .replace(config.contentInputFolder, '')
      .replace(path.parse(contentFilePath).ext, '')

    if (path.basename(contentPath) == 'index') {
      contentPath = path.dirname(contentPath)
    }

    // [ content-delivery, en, topics, introduction ]
    const contentPathParts = contentPath.replace(/\\/g, '/').split('/')

    // content-delivery
    const version = contentPathParts.shift()

    // en
    const lang = contentPathParts.shift()

    // prepare data for json
    let section = {
      path: contentPath, // content-delivery/en/topics/introduction
      lang: lang, // en
      version: version, // content-delivery
      title: title, // title from frontmatter
      attributes: frontmatterContent.attributes, // all attributes from frontmatter
      content: markdownContent // Markdown Content for left part of method section already as HTML
    }

    // check if version already exists in sections object
    if (typeof Docgen.sections[version] === 'undefined') {
      Docgen.sections[version] = {}
    }

    // check if language already exists in section version
    if (typeof Docgen.sections[version][lang] === 'undefined') {
      Docgen.sections[version][lang] = {}
    }

    // assign data to version, lang and contentPath combination
    Docgen.sections[version][lang][contentPath] = section

    return section
  },

  /**
   * Generate and export a routes.json which will be used by Nuxt during "nuxt generate"
   */
  generateRoutes: () => {
    const routes = []
    Docgen.listFoldersInFolder(config.contentInputFolder).forEach((version) => {
      Docgen.listFoldersInFolder(config.contentInputFolder + version).forEach(
        (lang) => {
          if (lang == config.defaultLanguage) {
            routes.push(`/docs/${version}/`)
          } else {
            routes.push(`/${lang}/docs/${version}/`)
          }
        }
      )
    })

    fs.writeFile(config.availableRoutesFile, JSON.stringify(routes), (err) => {
      if (err) throw err
    })
  },

  /**
   * Returns all first level subfolder names as string Array
   * @param {string} folder - Path to folder you want all first level subfolders.
   * @returns {Array<string>} folders - Array of folder names as string
   */
  listFoldersInFolder: (folder) => {
    return fs.readdirSync(folder).filter((file) => {
      return fs.statSync(path.join(folder, file)).isDirectory()
    })
  }
}

Docgen.init()
