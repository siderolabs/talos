<template>
  <div>
    <div class="inline-flex justify-between w-full">
      <h3 class="pt-0 mt-0 mb-1 text-sm tracking-tight uppercase border-none">
        {{ title }}
      </h3>

      <svg
        class="h-6 w-6 fill-current mr-2 text-ui-typo cursor-pointer caret"
        :class="{ rotate: show }"
        viewBox="0 0 32 32"
        aria-hidden="true"
        @click="show = !show"
      >
        <path
          class="text-ui-typo"
          d="M16.003 18.626l7.081-7.081L25 13.46l-8.997 8.998-9.003-9 1.917-1.916z"
        />
      </svg>
    </div>

    <ul v-show="show" class="max-w-full pl-2">
      <li
        v-for="page in findPages(items)"
        :id="page.path"
        :key="page.path"
        :class="getClassesForAnchor(page)"
        @mousedown="$emit('navigate')"
      >
        <g-link :to="`${page.path}`" class="flex items-center py-1">
          <span
            class="absolute w-2 h-2 -ml-3 rounded-full opacity-0 bg-ui-primary transition transform scale-0 origin-center"
            :class="{
              'opacity-100 scale-100': currentPage.path === page.path,
            }"
          ></span>
          {{ page.title }}
        </g-link>
      </li>
    </ul>
  </div>
</template>

<script>
export default {
  props: ["title", "items"],

  data() {
    return {
      show: false,
    };
  },

  computed: {
    pages() {
      return this.$page.allMarkdownPage.edges.map((edge) => edge.node);
    },
    currentPage() {
      return this.$page.markdownPage;
    },
  },
  methods: {
    getClassesForAnchor({ path }) {
      return {
        "text-ui-primary": this.currentPage.path === path,
        "transition transform hover:translate-x-1 hover:text-ui-primary":
          !this.currentPage.path === path,
      };
    },
    findPages(links) {
      return links.map((link) => this.pages.find((page) => page.path === link));
    },
  },
};
</script>

<style>
.caret {
  -webkit-transition: all 0.3s ease-in-out;
  -moz-transition: all 0.3s ease-in-out;
  -o-transition: all 0.3s ease-in-out;
  transition: all 0.3s ease-in-out;
}

.rotate {
  transform: rotateX(180deg);
}
</style>
