<template>
  <div>
    <button
      @click.prevent="active = !active"
      class="font-semibold my-2 rounded inline-flex items-center"
    >
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
    <ul v-show="active" class="pt-1 mb-4 w-full shadow-md">
      <li v-for="option in options" :key="option.version">
        <nuxt-link
          :to="option.url"
          class="rounded-t py-2 px-4 block whitespace-no-wrap"
          >{{ version(option) }}</nuxt-link
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
      options: [
        { version: 'v0.7', url: '/docs/v0.7', prerelease: true },
        { version: 'v0.6', url: '/docs/v0.6', prerelease: false },
        { version: 'v0.5', url: '/docs/v0.5', prerelease: false },
        { version: 'v0.4', url: '/docs/v0.4', prerelease: false }
      ],
      active: false
    }
  },

  methods: {
    version(option) {
      if (option.prerelease) {
        return `${option.version} (pre-release)`
      }

      return option.version
    }
  }
}
</script>
