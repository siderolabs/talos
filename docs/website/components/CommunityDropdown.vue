<template>
  <div class="dropdown inline-block">
    <button class="font-semibold py-1 px-1 rounded inline-flex items-center">
      <span class="mr-1">Community</span>
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
      <li class="" v-for="link in links" :key="link.url">
        <a
          :href="link.url"
          class="rounded-t py-2 px-4 block whitespace-no-wrap"
          >{{ link.title }}</a
        >
      </li>
    </ul>
  </div>
</template>

<script>
export default {
  name: 'CommunityDropdown',

  data() {
    return {
      links: [
        { title: 'GitHub', url: 'https://github.com/talos-systems' },
        { title: 'Slack', url: 'https://slack.dev.talos-systems.io' },
        {
          title: 'Forum',
          url:
            'https://groups.google.com/a/talos-systems.com/forum/#!forum/community'
        },
        { title: 'Twitter', url: 'https://twitter.com/talossystems' }
      ]
    }
  }
}
</script>
