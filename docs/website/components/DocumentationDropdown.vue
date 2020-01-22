<template>
  <div class="dropdown inline-block">
    <button class="font-semibold py-2 pl-4 rounded inline-flex items-center">
      <span class="mr-1">{{ $store.state.sidebar.version }}</span>
      <svg
        id="dropdown-caret"
        class="h-6 w-6 fill-current mr-2"
        viewBox="0 0 32 32"
        aria-hidden="true"
      >
        <path
          d="M16.003 18.626l7.081-7.081L25 13.46l-8.997 8.998-9.003-9 1.917-1.916z"
        />
      </svg>
    </button>
    <ul class="dropdown-menu absolute pt-1 w-full shadow-md">
      <li v-for="option in options" :key="option.version" class="">
        <a
          :href="option.url"
          class="rounded-t py-2 px-4 block whitespace-no-wrap"
          @click="handleClick(option)"
          >{{ version(option) }}</a
        >
      </li>
    </ul>
  </div>
</template>

<script>
export default {
  name: 'Dropdown',

  data() {
    return {
      options: [{ version: 'v0.3', url: '/docs/v0.3', prerelease: false }]
    }
  },

  methods: {
    handleClick(option) {
      this.$store.commit('sidebar/setVersion', option.version)
    },

    version(option) {
      if (option.prerelease) {
        return `${option.version} (pre-release)`
      }

      return option.version
    }
  }
}
</script>
