const routes = require('./routes')

export default {
  mode: 'universal',

  /*
   ** Headers of the page
   */
  head: {
    title: process.env.npm_package_name || '',
    meta: [
      { charset: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      {
        hid: 'description',
        name: 'description',
        content: process.env.npm_package_description || ''
      }
    ],
    script: [{ src: '/js/asciinema-player.js' }],
    link: [{ rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' }]
  },

  /*
   ** Customize the progress-bar color
   */
  loading: { color: '#fff' },

  /*
   ** Global CSS
   */
  css: ['@/assets/css/main.css'],

  /*
   ** Plugins to load before mounting the App
   */
  plugins: [],

  /*
   ** Nuxt.js dev-modules
   */
  buildModules: [
    // Doc: https://github.com/nuxt-community/eslint-module
    '@nuxtjs/eslint-module',
    // Doc: https://github.com/nuxt-community/nuxt-tailwindcss
    '@nuxtjs/tailwindcss',
    [
      '@nuxtjs/google-analytics',
      {
        id: 'UA-141692582-2'
      }
    ]
  ],

  /*
   ** Nuxt.js dev-modules configuration
   */
  eslint: {
    fix: true
  },

  // PurgeCSS is automatically installed by @nuxtjs/tailwindcss
  purgeCSS: {
    enabled: false
  },

  /*
   ** Nuxt.js modules
   */
  modules: ['nuxt-webfontloader'],

  webfontloader: {
    google: {
      families: ['Lato:400,700', 'Nunito Sans:400,700', 'Fira Mono:400,700']
    }
  },

  generate: {
    fallback: true,
    routes(callback) {
      let generatedRoutes = []
      routes.forEach((route) => {
        const parts = route.split('/')

        let lang = parts[1]
        let version = parts[3]

        if (lang == 'docs') {
          lang = 'en'
          version = parts[2]
        }

        const r = {
          route: route,
          payload: {
            sections: require(`${__dirname}/static/${version}.sections.${lang}.json`),
            menu: require(`${__dirname}/static/${version}.menu.${lang}.json`)
          }
        }

        generatedRoutes.push(r)
      })

      callback(null, generatedRoutes)
    }
  },

  /*
   ** Build configuration
   */
  build: {
    /*
     ** You can extend webpack config here
     */
    extend(config, ctx) {},
    extractCSS: true
  }
}
