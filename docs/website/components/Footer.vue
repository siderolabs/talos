<template>
  <footer>
    <div class="flex flex-col items-center my-3">
      <img class="cncf" src="/images/cncf-color.png" alt="" />
      <p>
        We are a
        <a href="https://cncf.io" target="_blank"
          >Cloud Native Computing Foundation</a
        >
        member.
      </p>
    </div>
    <div>
      <a class="inline-block no-underline color-inherit" href="/">
        © 2019 Talos Systems, Inc.
      </a>
    </div>
  </footer>
</template>

<style>
footer {
  @apply bg-white flex flex-col items-center font-sans justify-between py-5;
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

.cncf {
  width: 250px;
  height: 39px;
}
</style>
