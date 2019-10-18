<template>
  <article class="article-content c-rich-text">
    <h1 class="page-heading">{{ doc.title }}</h1>
    <div class="my-0">
      <a
        :href="
          'https://github.com/talos-systems/talos/edit/master/docs/website/content/' +
            $store.state.sidebar.activeMenuPath +
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
        Edit this Page
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
      return this.$store.state.sidebar.sections[
        this.$store.state.sidebar.activeMenuPath
      ]
    }
  }
}
</script>
