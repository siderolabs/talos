<template>
  <div class="border-ui-primary w-full">
    <div class="container">
      <div class="flex items-center justify-between -mx-2 sm:-mx-4">
        <div class="flex flex-row items-center px-2 sm:px-4 sm:flex-row">
          <div v-if="$page" class="z-50 p-2 lg:hidden">
            <button
              class="p-2 text-ui-primary rounded-full"
              @click="$store.commit('toggleSidebarIsOpen')"
            >
              <XIcon v-if="$store.state.sidebarIsOpen" />
              <MenuIcon v-else />
            </button>
          </div>

          <g-link to="/" class="flex items-center text-ui-primary" title="Home">
            <span
              class="hidden ml-2 text-xl font-black tracking-tighter uppercase sm:block"
            >
              {{ meta.siteName }}
            </span>
          </g-link>

          <div
            v-if="settings.nav.links.length > 0"
            class="lg:ml-8 lg:mr-8 ml-2 mr-2 sm:block"
          >
            <g-link
              v-for="link in settings.nav.links"
              :key="link.path"
              :to="pathTo(link)"
              class="block p-1 font-medium nav-link text-ui-typo hover:text-ui-primary"
            >
              {{ link.title }}
            </g-link>
          </div>
        </div>

        <div class="flex items-center justify-end px-2 sm:px-4">
          <a
            v-if="settings.web"
            :href="settings.web"
            class="hidden ml-3 sm:block"
            target="_blank"
            rel="noopener noreferrer"
            title="Website"
            name="Website"
          >
            <GlobeIcon size="1.5x" />
          </a>

          <a
            v-if="settings.twitter"
            :href="settings.twitter"
            class="hidden ml-3 sm:block"
            target="_blank"
            rel="noopener noreferrer"
            title="Twitter"
            name="Twitter"
          >
            <TwitterIcon size="1.5x" />
          </a>

          <a
            v-if="settings.github"
            :href="settings.github"
            class="sm:ml-3"
            target="_blank"
            rel="noopener noreferrer"
            title="Github"
            name="Github"
          >
            <GithubIcon size="1.5x" />
          </a>

          <ToggleDarkMode class="ml-2 sm:ml-8">
            <template slot="default" slot-scope="{ dark }">
              <MoonIcon v-if="dark" size="1.5x" />
              <SunIcon v-else size="1.5x" />
            </template>
          </ToggleDarkMode>
        </div>
      </div>
    </div>
  </div>
</template>

<static-query>
query {
  metadata {
    siteName
    settings {
      web
      github
      twitter
      nav {
        links {
          path
          title
        }
      }
      dropdownOptions {
        version
        url
        latest
      }
    }
  }
}
</static-query>

<script>
import ToggleDarkMode from "@/components/ToggleDarkMode";
import Logo from "@/components/Logo";
import {
  SunIcon,
  MoonIcon,
  GlobeIcon,
  GithubIcon,
  TwitterIcon,
  MenuIcon,
  XIcon,
} from "vue-feather-icons";

export default {
  components: {
    Logo,
    ToggleDarkMode,
    SunIcon,
    MoonIcon,
    GlobeIcon,
    GithubIcon,
    TwitterIcon,
    MenuIcon,
    XIcon,
  },

  computed: {
    meta() {
      return this.$static.metadata;
    },
    settings() {
      return this.meta.settings;
    },
  },

  methods: {
    pathTo(link) {
      if (link.path) {
        return link.path;
      }

      let url = "";

      this.meta.settings.dropdownOptions.forEach((element) => {
        if (element.latest) {
          url = element.url;

          return;
        }
      });

      return url;
    },
  },
};
</script>

<style lang="scss">
header {
  svg:not(.feather-search) {
    &:hover {
      @apply text-ui-primary;
    }
  }
}

.nav-link {
  &.active {
    @apply text-ui-primary font-bold border-ui-primary;
  }
}
</style>
