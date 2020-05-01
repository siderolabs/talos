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
      let activeDocPath = ''

      // if there's an #anchor specified, go there
      if (this.$route.hash) {
        activeDocPath = this.$route.hash.substring(1)
      } else {
        // otherwise go to the first item in the menu
        activeDocPath = this.$store.state.sidebar.menu[0].path
      }
      return sections[activeDocPath]
    },
    gitPath() {
      let path =
        'https://github.com/talos-systems/talos/edit/master/docs/website/content/'

      if (this.$route.hash) {
        path += this.$route.hash.substring(1)
      } else {
        path += this.$store.state.sidebar.menu[0].path + '/index'
      }
      return path + '.md'
    }
  }
}
</script>
