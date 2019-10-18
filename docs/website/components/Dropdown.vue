<template>
  <div class="dropdown inline-block relative">
    <button class="font-semibold py-2 px-4 rounded inline-flex items-center">
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
      <span class="mr-1">Documentation</span>
    </button>
    <ul class="dropdown-menu absolute pt-1 w-full">
      <li class="" v-for="option in options" :key="option.version">
        <a
          :href="option.url"
          class="rounded-t py-2 px-4 block whitespace-no-wrap"
          >{{ option.version }}</a
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
      selected: '',
      options: [
        // { version: 'v0.3 (pre-release)', url: '/docs/v0.3' },
        { version: 'v0.2', url: '/docs/v0.2' }
      ]
    }
  }
}
</script>

<style>
.dropdown {
  @apply font-headings;
  font-size: 20px;
}

.dropdown-menu {
  height: 0px;
  background: white;
  overflow: hidden;
  -webkit-transition: all 0.3s ease-in-out;
  -moz-transition: all 0.3s ease-in-out;
  -o-transition: all 0.3s ease-in-out;
  transition: all 0.3s ease-in-out;
}

#dropdown-caret {
  -webkit-transition: all 0.3s ease-in-out;
  -moz-transition: all 0.3s ease-in-out;
  -o-transition: all 0.3s ease-in-out;
  transition: all 0.3s ease-in-out;
}
.dropdown:hover #dropdown-caret {
  transform: rotateX(180deg);
}

.dropdown:hover .dropdown-menu {
  height: 100px;
}
</style>
