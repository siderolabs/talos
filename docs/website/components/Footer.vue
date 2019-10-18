<template>
  <footer>
    <div>
      <a class="inline-block no-underline color-inherit" href="/">
        © 2019 Talos Systems, Inc.
      </a>
    </div>
  </footer>
</template>

<style>
footer {
  @apply bg-gray-200 flex flex-wrap items-center font-sans justify-between py-4 px-4;
  z-index: 99;
}

footer a {
  @apply text-gray-600 text-xs;
}

footer a:hover {
  @apply text-black underline;
}

.color-inherit {
  color: inherit;
}
</style>
