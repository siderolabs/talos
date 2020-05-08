<template>
  <div>
    <div class="text-gray-500 text-base font-bold uppercase">
      Table of Contents
    </div>
    <nav class="text-sm">
      <div id="toc" v-html="toc" class="pt-4 text-gray-700 text-sm"></div>
    </nav>
  </div>
</template>

<script>
export default {
  name: 'TableOfContents',
  props: { toc: { type: String, default: '' } }
}
</script>

<style>
div#toc {
  top: 90px;
  @apply font-sans tracking-wide;
}

div#toc ul {
  @apply pl-4;
}
</style>
