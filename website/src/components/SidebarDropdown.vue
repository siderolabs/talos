<template>
  <div @mouseleave="active = false">
    <button
      @mouseover="active = true"
      class="font-semibold mb-4 rounded inline-flex items-center"
    >
      <span class="mr-1 text-ui-typo">{{ $page.markdownPage.version }}</span>
      <svg
        id="dropdown-caret"
        class="h-6 w-6 fill-current mr-2 text-ui-typo"
        viewBox="0 0 32 32"
        aria-hidden="true"
      >
        <path
          class="text-ui-typo"
          d="M16.003 18.626l7.081-7.081L25 13.46l-8.997 8.998-9.003-9 1.917-1.916z"
        />
      </svg>
    </button>
    <ul
      v-show="active"
      class="pt-1 mb-4 w-3/4 shadow-lg absolute bg-ui-background text-ui-typo z-50 rounded"
    >
      <li
        v-for="option in $static.metadata.settings.dropdownOptions"
        :key="option.version"
        class="w-full"
      >
        <button
          @click="change(option.version, option.url)"
          class="rounded-t py-2 px-4 w-full block text-left whitespace-no-wrap"
        >
          {{ version(option) }}
        </button>
      </li>
    </ul>
  </div>
</template>

<static-query>
query Sidebar {
  metadata {
    settings {
      dropdownOptions {
        version
        url
        latest
        prerelease
      }
    }
  }
}
</static-query>

<script>
export default {
  name: "SidebarDropdown",

  data() {
    return {
      options: [],
      active: false,
    };
  },

  methods: {
    change(version, url) {
      if (this.$page.markdownPage.version !== version) {
        window.location.href = url;
      }

      this.active = false;
    },
    version(option) {
      if (option.latest && option.prerelease) {
        return `${option.version} (latest, pre-release)`;
      }

      if (option.prerelease) {
        return `${option.version} (pre-release)`;
      }

      if (option.latest) {
        return `${option.version} (latest)`;
      }

      return option.version;
    },
  },
};
</script>
