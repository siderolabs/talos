<template>
  <div>
    <div class="px-6 mb-4 section-docs">
      <div class="flex flex-wrap">
        <div class="md:w-1/5 py-4">
          <Sidebar></Sidebar>
          <div class="sidebar-sticky pt-1">
            <a href="#sidenav"
              ><chevrons-up-icon class="inline align-middle"></chevrons-up-icon>
              Back to Top</a
            >
          </div>
        </div>
        <div class="w-full md:w-3/5">
          <Content :doc="doc" class="md:px-4"></Content>
        </div>
        <div class="md:w-1/5">
          <TableOfContents :toc="doc.toc" class="py-4"></TableOfContents>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ChevronsUpIcon } from 'vue-feather-icons'
import axios from 'axios'
import Sidebar from '~/components/Sidebar.vue'
import Content from '~/components/Content.vue'
import TableOfContents from '~/components/TableOfContents.vue'

export default {
  name: 'Doc',
  layout: 'doc',
  components: {
    Sidebar,
    Content,
    TableOfContents,
    ChevronsUpIcon
  },

  head() {
    return {
      title: this.doc.title + ' - Talos Documentation'
    }
  },

  computed: {
    doc() {
      const sections = this.$store.state.sidebar.sections

      // this is a hack to avoid breaking old (v0.3 and v0.4 only) deep links using '#' that look like:
      //   /docs/v0.5/en/guides/cloud/digitalocean#creating-a-cluster-via-the-cli
      // At 0.5, the # changed to a real path separator (/) and anchors ('#') are reserved for
      // deep links to particular headings within the markdown file.
      if (
        this.$route.hash.startsWith('#v0.3') ||
        this.$route.hash.startsWith('#v0.4')
      ) {
        return sections[this.$route.hash.substring(1)]
      }

      if (sections[this.$route.params.pathMatch])
        return sections[this.$route.params.pathMatch]
      else return sections[this.$store.state.sidebar.menu[0].path]
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
  },

  mounted() {
    // if we hit the top-level, redirect to the first page in the doc set
    if (!this.$route.params.pathMatch.includes('/')) {
      this.$router.replace('/docs/' + this.$store.state.sidebar.menu[0].path)
    }
  }
}
</script>

<style scoped>
.sidebar-sticky {
  position: sticky;
  top: 88px;
}
</style>
