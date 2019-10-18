<template>
  <div class="app">
    <Header></Header>
    <main role="main" class="content">
      <nuxt />
    </main>
    <Footer></Footer>
  </div>
</template>

<script>
import Header from '~/components/Header.vue'
import Footer from '~/components/Footer.vue'

export default {
  components: {
    Header,
    Footer
  }
}
</script>

<style>
.app {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

html {
  background: url(~assets/images/chips_bg_v1.svg) no-repeat center center fixed;
  -webkit-background-size: contain;
  -moz-background-size: contain;
  -o-background-size: contain;
  background-size: contain;
  background-position: center bottom;
}
</style>
