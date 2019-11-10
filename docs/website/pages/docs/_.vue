<template>
  <div>
    <div id="content" class="content mb-4 section-docs">
      <div class="flex justify-between">
        <div class="c-rich-text">
          <h3>Documentation</h3>
        </div>
        <DocumentationDropdown></DocumentationDropdown>
      </div>
      <div class="flex flex-wrap">
        <div class="md:w-1/4 mt-1">
          <Sidebar></Sidebar>
        </div>
        <div class="md:w-3/4 mt-1">
          <Content></Content>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios'
import DocumentationDropdown from '~/components/DocumentationDropdown.vue'
import Sidebar from '~/components/Sidebar.vue'
import Content from '~/components/Content.vue'

export default {
  name: 'Doc',
  components: {
    DocumentationDropdown,
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
    // The initial state does not have an active doc set, so we set the default
    // to the first element in the menu.
    const defaultActiveDoc = menu[0].path
    store.commit('sidebar/setActiveDocPath', defaultActiveDoc)
  }
}
</script>

<style></style>
