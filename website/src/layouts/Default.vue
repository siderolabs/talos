<template>
  <div class="font-sans antialiased text-ui-typo bg-ui-background">
    <header
      ref="header"
      class="sticky top-0 z-10 w-full border-b bg-ui-background border-ui-border h-20 flex items-center"
    >
      <LayoutHeader />
    </header>

    <main>
      <div v-if="$page" class="container grid grid-cols-1 md:grid-cols-10">
        <div class="col-span-1 md:col-span-2">
          <aside
            :class="{ open: $store.state.sidebarIsOpen }"
            class="sidebar-l"
          >
            <div class="w-full mt-10">
              <Sidebar />
            </div>
          </aside>
        </div>

        <div class="col-span-1 md:col-span-6">
          <div class="w-full mt-1 px-2 md:px-6">
            <slot />
          </div>
        </div>

        <div class="col-span-1 md:col-span-2">
          <aside class="sidebar-r">
            <div class="w-full mt-10">
              <OnThisPage />
            </div>
          </aside>
        </div>
      </div>

      <div v-else class="container grid grid-cols-1">
        <div class="col-span-1">
          <div class="w-full">
            <slot />
          </div>
        </div>
      </div>
    </main>

    <footer>
      <LayoutFooter />
    </footer>
  </div>
</template>

<static-query>
query {
  metadata {
    siteName
  }
}
</static-query>

<script>
import LayoutHeader from "@/components/LayoutHeader";
import Sidebar from "@/components/Sidebar";
import OnThisPage from "@/components/OnThisPage.vue";
import LayoutFooter from "@/components/LayoutFooter";

export default {
  components: {
    LayoutHeader,
    Sidebar,
    OnThisPage,
    LayoutFooter,
  },

  metaInfo() {
    return {
      meta: [
        {
          key: "og:type",
          name: "og:type",
          content: "website",
        },
        {
          key: "twitter:card",
          name: "twitter:card",
          content: "summary_large_image",
        },
        {
          key: "og:image",
          name: "og:image",
          content: process.env.SITE_URL + "/logo.jpg",
        },
        {
          key: "twitter:image",
          name: "twitter:image",
          content: process.env.SITE_URL + "/logo.jpg",
        },
      ],
    };
  },
};
</script>

<style lang="scss">
:root {
  --color-ui-background: theme("colors.white");
  --color-ui-typo: theme("colors.gray.700");
  --color-ui-sidebar: theme("colors.gray.200");
  --color-ui-border: theme("colors.gray.300");
  --color-ui-primary: theme("colors.teal.600");
}

html[lights-out] {
  --color-ui-background: theme("colors.gray.900");
  --color-ui-typo: theme("colors.gray.100");
  --color-ui-sidebar: theme("colors.gray.800");
  --color-ui-border: theme("colors.gray.800");
  --color-ui-primary: theme("colors.teal.500");

  pre[class*="language-"],
  code[class*="language-"] {
    @apply bg-ui-border;
  }
}

* {
  transition-property: color, background-color, border-color;
  transition-duration: 200ms;
  transition-timing-function: ease-in-out;
}

h1,
h2,
h3,
h4 {
  @apply leading-snug font-black mb-4 text-ui-typo;

  &:hover {
    a::before {
      @apply opacity-100;
    }
  }

  a {
    &::before {
      content: "#";
      margin-left: -1em;
      padding-right: 1em;
      @apply text-ui-primary absolute opacity-0 float-left;
    }
  }
}

h1 {
  @apply text-4xl;
}

h2 {
  @apply text-2xl;
}

h3 {
  @apply text-xl;
}

h4 {
  @apply text-lg;
}

a:not(.active):not(.text-ui-primary):not(.text-white) {
  @apply text-ui-typo;
}

p,
ol,
ul,
pre,
strong,
blockquote {
  @apply mb-4 text-base text-ui-typo;
}

.content {
  a {
    @apply text-ui-primary underline;
  }

  h1,
  h2,
  h3,
  h4,
  h5,
  h6 {
    @apply -mt-12 pt-20;
  }

  h2 + h3,
  h2 + h2,
  h3 + h3 {
    @apply border-none -mt-20;
  }

  h2,
  h3 {
    @apply border-b border-ui-border pb-1 mb-3;
  }

  ul {
    @apply list-disc;

    ul {
      list-style: circle;
    }
  }

  ol {
    @apply list-decimal;
  }

  ol,
  ul {
    @apply pl-5 py-1;

    li {
      @apply mb-2;

      p {
        @apply mb-0;
      }

      &:last-child {
        @apply mb-0;
      }
    }
  }
}

blockquote {
  @apply border-l-4 border-ui-border py-2 pl-4;

  p:last-child {
    @apply mb-0;
  }
}

code {
  @apply px-1 py-1 text-ui-typo bg-ui-sidebar font-mono border-b border-r border-ui-border text-sm rounded;
}

pre[class*="language-"] {
  @apply max-w-full overflow-x-auto rounded;

  & + p {
    @apply mt-4;
  }

  & > code[class*="language-"] {
    @apply border-none leading-relaxed;
  }
}

header {
  background-color: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(4px);
}

table {
  @apply text-left mb-6;

  td,
  th {
    @apply py-3 px-4;
    &:first-child {
      @apply pl-0;
    }
    &:last-child {
      @apply pr-0;
    }
  }

  tr {
    @apply border-b border-ui-border;
    &:last-child {
      @apply border-b-0;
    }
  }
}

.sidebar {
  @apply w-full inset-x-0 z-50 overflow-y-auto bg-ui-background border-ui-border;
  height: calc(100vh - 5rem);

  @screen lg {
    top: 5rem;
    @apply px-0 bg-transparent bottom-auto inset-x-auto sticky z-0;
  }
}

.sidebar::-webkit-scrollbar {
  display: none;
}

.sidebar-l {
  @extend .sidebar;
  @apply px-4 border-r transition-all;
  transform: translateX(-100%);

  &.open {
    transform: translateX(0);
  }

  @media (max-width: 1024px) {
    @apply fixed;
  }

  @screen lg {
    transform: translateX(0);
  }
}

.sidebar-r {
  @extend .sidebar;
  @screen lg {
    @apply px-4 border-l;
  }
}
</style>
