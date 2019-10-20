<template>
  <header id="header" class="flex">
    <div
      class="flex flex-wrap items-center justify-start max-w-6xl mx-auto py-6"
    >
      <Logo></Logo>
    </div>
    <div class="flex flex-wrap items-center justify-end max-w-6xl mx-auto py-6">
      <DocumentationDropdown></DocumentationDropdown>
      <CommunityDropdown></CommunityDropdown>
    </div>
  </header>
</template>

<script>
import Logo from '~/components/Logo.vue'
import DocumentationDropdown from '~/components/DocumentationDropdown.vue'
import CommunityDropdown from '~/components/CommunityDropdown.vue'

export default {
  name: 'Header',
  components: {
    Logo,
    DocumentationDropdown,
    CommunityDropdown
  },

  mounted() {
    window.addEventListener('scroll', this.handleScroll)
  },

  destroyed() {
    window.removeEventListener('scroll', this.handleScroll)
  },

  methods: {
    handleScroll() {
      const distanceY = window.pageYOffset || document.documentElement.scrollTop
      const shrinkOn = 240
      const shrinkOff = 140
      const headerEl = document.getElementById('header')

      if (distanceY > shrinkOn) {
        headerEl.classList.add('scrolled')
      } else if (distanceY < shrinkOff) {
        headerEl.classList.remove('scrolled')
      }
    }
  }
}
</script>
