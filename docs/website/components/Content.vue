<template>
  <article class="c-rich-text">
    <div class="flex">
      <h2 class="flex-grow">{{ doc.title }}</h2>
      <GitHubLink
        class="flex-grow-0 object-middle p-2 pt-4 md:pr-4"
      ></GitHubLink>
    </div>

    <div v-html="doc.content" class="border-t pt-4"></div>
  </article>
</template>

<script>
import GitHubLink from '~/components/GitHubLink.vue'

export default {
  name: 'Content',
  components: { GitHubLink },
  props: { doc: { type: Object, default() {} } }
}
</script>
