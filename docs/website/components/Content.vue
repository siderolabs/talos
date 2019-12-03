<template>
  <article class="max-w-3xl pt-1 pb-4 pl-6 pr-6 mx-auto c-rich-text">
    <h1 class="page-heading">{{ doc.title }}</h1>
    <div class="my-0">
      <a
        :href="
          'https://github.com/talos-systems/talos/edit/master/docs/website/content/' +
            $store.getters['sidebar/getActiveDocPath'] +
            '.md'
        "
        class="no-underline font-normal text-sm"
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
    <div class="border-t pt-4" v-html="doc.content"></div>
  </article>
</template>

<script>
export default {
  name: 'Content',

  computed: {
    doc() {
      const sections = this.$store.getters['sidebar/getSections']
      let activeDocPath = ''

      // if there's an #anchor specified, go there instead of the top-level
      if (this.$route.hash) {
        activeDocPath = this.$route.hash.substring(1)
      } else {
        activeDocPath = this.$store.getters['sidebar/getActiveDocPath']
      }

      return sections[activeDocPath]
    }
  }
}
</script>
