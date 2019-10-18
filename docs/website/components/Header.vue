<template>
  <header id="header">
    <div
      class="max-w-6xl mx-auto py-6 flex flex-wrap items-center justify-between"
    >
      <Logo></Logo>
      <Dropdown></Dropdown>
    </div>
  </header>
</template>

<script>
import Logo from '~/components/Logo.vue'
import Dropdown from '~/components/Dropdown.vue'

export default {
  name: 'Header',
  components: {
    Logo,
    Dropdown
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
