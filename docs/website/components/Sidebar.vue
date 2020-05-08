<template>
  <nav id="sidenav" class="sidenav">
    <div class="sidebar-heading">
      Documentation
    </div>
    <DocumentationDropdown class="mb-4"></DocumentationDropdown>

    <ul>
      <li v-for="entry in $store.state.sidebar.menu" :key="entry.title">
        <span class="sidebar-category pt-4">{{ entry.title }}</span>
        <ul class="pt-1 pb-2">
          <li
            v-for="item in entry.items"
            :key="item.path"
            class="sidebar-item my-2"
          >
            <div v-if="item.children" class="ml-4 pt-2 sidebar-subcategory">
              {{ item.title }}
            </div>
            <nuxt-link v-else :to="'/docs/' + item.path" class="block ml-2">
              <span class="p-2">{{ item.title }}</span>
            </nuxt-link>

            <ul v-if="item.children" class="sidebar-children ml-4 mt-2">
              <li
                v-for="child in item.children"
                :key="child.path"
                class="sidebar-item my-1"
              >
                <nuxt-link :to="'/docs/' + child.path" class="block m-1">
                  <span class="p-2">{{ child.title }}</span>
                </nuxt-link>
              </li>
            </ul>
          </li>
        </ul>
      </li>
    </ul>
  </nav>
</template>

<script>
import DocumentationDropdown from '~/components/DocumentationDropdown.vue'

export default {
  name: 'Sidebar',
  components: {
    DocumentationDropdown
  }
}
</script>

<style>
.sidenav {
  @apply font-sans tracking-wide bg-white;
}

.sidenav::before {
  display: block;
  content: ' ';
  margin-top: -100px;
  height: 100px;
  visibility: hidden;
  pointer-events: none;
}

.sidebar-heading {
  @apply font-headings text-gray-800 relative block text-2xl;
}

.sidebar-category {
  @apply text-gray-500 text-base font-bold uppercase;
}

.sidebar-item {
  @apply text-gray-700 text-sm;
}

.sidebar-subcategory {
  @apply uppercase text-gray-500 font-bold;
}

a:hover {
  @apply text-gray-900 font-bold;
}

.nuxt-link-active {
  @apply bg-primary-color-100 rounded-md text-gray-800;
}
</style>
