<template>
  <div id="content" class="content mb-4 section-docs">
    <div class="md:flex flex-wrap">
      <div class="md:w-1/4 mt-6">
        <Sidebar></Sidebar>
      </div>
      <div class="md:w-3/4 mt-6">
        <Content></Content>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios'
import Sidebar from '~/components/Sidebar.vue'
import Content from '~/components/Content.vue'

export default {
  name: 'Doc',
  components: {
    Sidebar,
    Content
  },

  head: {
    bodyAttrs: {
      class: 'kind-section section-docs'
    }
  },

  async fetch({ store, params, payload }) {
    const version = params.pathMatch.split('/')[0]
    const lang = params.lang || 'en'

    let menu = null
    let sections = null

    if (payload) {
      menu = payload.menu
      sections = payload.sections
    } else {
      const base = process.client
        ? window.location.origin
        : 'http://localhost:3000'

      const [menuRes, sectionsRes] = await Promise.all([
        axios.get(base + `/${version}.menu.${lang}.json`),
        axios.get(base + `/${version}.sections.${lang}.json`)
      ])

      menu = menuRes.data
      sections = sectionsRes.data
    }

    store.commit('sidebar/setLang', lang)
    store.commit('sidebar/setVersion', version)
    store.commit('sidebar/setSections', sections)
    store.commit('sidebar/setMenu', menu)
  }
}
</script>

<style>
body.kind-page #content {
  @apply max-w-3xl mx-auto px-6;
}

body.kind-section #content,
body.section-docs #content {
  @apply max-w-6xl mx-auto px-6;
}
@screen lg {
  body.kind-section #content body.kind-page #content {
    @apply px-0;
  }
}
body.section-docs .page-heading {
  @apply mb-1;
}
</style>
