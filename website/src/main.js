// This is the main.js file. Import global CSS and scripts here.
// The Client API can be used here. Learn more: gridsome.org/docs/client-api
import "~/assets/css/main.css";

import Vuex from "vuex";
import DefaultLayout from "~/layouts/Default.vue";

export default function(Vue, { router, head, isClient, appOptions }) {
  // Set default layout as a global component
  Vue.component("Layout", DefaultLayout);

  Vue.use(Vuex);

  appOptions.store = new Vuex.Store({
    state: {
      sidebarIsOpen: false,
    },

    mutations: {
      setSidebarIsOpen(state, val) {
        state.sidebarIsOpen = val;
      },
      toggleSidebarIsOpen(state) {
        state.sidebarIsOpen = !state.sidebarIsOpen;
      },
    },
  });

  router.beforeEach((to, _from, next) => {
    head.meta.push({
      key: "og:url",
      name: "og:url",
      content: process.env.GRIDSOME_BASE_PATH + to.path,
    });
    next();
  });

  head.script.push({
    src: "/js/asciinema-player.js",
    body: true,
  });
}
