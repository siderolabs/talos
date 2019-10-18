<template>
  <div class="content w-full mb-4">
    <article class="article-content c-rich-text">
      <h1 class="page-heading">{{ attributes.title }}</h1>
      <section class="border-t pt-4" v-html="content"></section>
    </article>
  </div>
</template>

<script>
import axios from 'axios'

export default {
  async asyncData({ params, error, payload }) {
    if (payload) {
      return {
        attributes: payload.attributes,
        content: payload.content
      }
    }

    const base = process.client
      ? window.location.origin
      : 'http://localhost:3000'

    const [indexRes] = await Promise.all([axios.get(base + '/index.json')])
    const index = indexRes.data

    return {
      attributes: index.attributes,
      content: index.content
    }
  }
}
</script>
