<template>
  <article class="max-w-3xl pt-1 pb-4 pl-6 pr-6 mx-auto c-rich-text">
    <div class="flex">
      <h2 class="page-heading flex-grow">{{ doc.title }}</h2>
      <a :href="gitPath" class="no-underline font-normal text-sm self-center"
        ><img
          src="/images/Git-Icon-Black.png"
          height="14px"
          width="14px"
          class="inline-block mr-1"
          alt=""
        />
        Edit this page
      </a>
    </div>
    <div v-html="doc.content" class="border-t pt-4"></div>
  </article>
</template>

<script>
export default {
  name: 'Content',

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
    },
    gitPath() {
      const path =
        'https://github.com/talos-systems/talos/edit/master/docs/website/content/'

      return path + this.$route.params.pathMatch + '.md'
    }
  },

  mounted() {
    // if we hit the top-level, redirect to the first page in the doc set
    if (!this.$route.params.pathMatch.includes('/')) {
      this.$router.replace('/docs/' + this.$store.state.sidebar.menu[0].path)
    }
  }
}
</script>
