<template>
  <div class="text-gray-800">
    <a :href="link"
      ><svg
        xmlns="http://www.w3.org/2000/svg"
        width="28"
        height="28"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        class="feather feather-edit text-gray-800"
      >
        <title>Edit on GitHub</title>
        <path
          d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"
        ></path>
        <path
          d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
        ></path>
      </svg>
    </a>
  </div>
</template>

<script>
export default {
  name: 'GitHubLink',

  computed: {
    link() {
      const path =
        'https://github.com/talos-systems/talos/edit/master/docs/website/content/'

      return path + this.$route.params.pathMatch + '.md'
    }
  }
}
</script>
