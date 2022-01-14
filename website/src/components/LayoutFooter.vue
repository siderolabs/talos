<template>
  <div class="pb-8">
    <div class="flex flex-col items-center my-3">
      <img class="cncf" src="/images/cncf-color.png" alt="" />
      <p class="pt-2">
        We are a
        <a href="https://cncf.io" target="_blank"
          >Cloud Native Computing Foundation</a
        >
        member.
      </p>
    </div>
    <div class="text-center">
      <a class="inline-block no-underline color-inherit" href="/">
        © 2022 Sidero Labs, Inc.
      </a>
    </div>
  </div>
</template>

<style>
footer {
  @apply flex flex-col items-center font-sans justify-between py-5;
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
