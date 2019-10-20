<template>
  <div class="flex flex-col min-h-screen">
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
