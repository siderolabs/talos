<template>
  <header id="header" class="flex">
    <div class="flex items-center justify-start max-w-6xl mx-auto py-2">
      <Logo></Logo>
    </div>
    <div class="flex items-center justify-end max-w-6xl mx-auto py-6">
      <ul class="flex items-center justify-end">
        <li class="link">
          <a href="/docs/v0.2">
            <span class="font-semibold mr-1">Documentation</span>
          </a>
        </li>
        <li class="link">
          <CommunityDropdown></CommunityDropdown>
        </li>
        <li class="link">
          <a href="/faqs">
            <span class="font-semibold mr-1">FAQs</span>
          </a>
        </li>
      </ul>
    </div>
  </header>
</template>

<script>
import Logo from '~/components/Logo.vue'
import CommunityDropdown from '~/components/CommunityDropdown.vue'

export default {
  name: 'Header',
  components: {
    Logo,
    CommunityDropdown
  }
}
</script>

<style scoped>
li {
  margin-left: 1.25rem;
}
</style>
