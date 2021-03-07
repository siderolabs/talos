<template>
  <g-image
    id="header-logo"
    src="/images/logo.svg"
    :width="getWidth"
    :height="getHeight"
  />
</template>

<script>
export default {
  props: {
    width: {
      type: Number,
      default: 40,
    },
  },

  computed: {
    getWidth() {
      return this.width;
    },

    getHeight() {
      return Math.ceil((this.getWidth / 48) * 48);
    },
  },
};
</script>
