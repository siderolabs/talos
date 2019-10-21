<template>
  <div class="terminal w-1/2 mx-auto">
    <div id="terminal-body">
      <div id="terminal-player-wrapper"></div>
    </div>
    <div id="terminal-buttons">
      <div class="flex flex-wrap justify-center">
        <button
          v-for="cast in casts"
          :key="cast.src"
          class="bg-primary-color-500 text-white font-semibold m-1 p-1 rounded"
          style="width: 100px"
          @click="handleClick(cast.src)"
        >
          <span class="mr-1">{{ cast.title }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'Terminal',

  data() {
    return {
      casts: [
        { title: 'Cluster', src: '/cluster-create.cast' },
        { title: 'Services', src: '/osctl-services.cast' },
        { title: 'Routes', src: '/osctl-routes.cast' },
        { title: 'Interfaces', src: '/osctl-interfaces.cast' },
        { title: 'Containers', src: '/osctl-containers.cast' },
        { title: 'Processes', src: '/osctl-processes.cast' },
        { title: 'Mounts', src: '/osctl-mounts.cast' }
      ]
    }
  },

  mounted() {
    this.handleClick('/cluster-create.cast')
  },

  methods: {
    handleClick(src) {
      const terminalPlayerWrapper = document.getElementById(
        'terminal-player-wrapper'
      )
      const terminalRows = 25
      terminalPlayerWrapper.innerHTML =
        '<asciinema-player id="terminal-player" cols="100" rows="' +
        terminalRows +
        '" preload autoplay loop speed="1.0" src="' +
        src +
        '"></asciinema-player>'
      console.log(src)
      this.src = src
    }
  }
}
</script>

<style>
#terminal-body {
  height: auto;
  width: auto;
}

.control-bar {
  display: none;
}
</style>
