<template>
  <div ref="sidebar" class="px-4 pt-8 lg:pt-12">
    <SidebarDropdown />

    <div class="pr-2 pt-2 pb-10 max-w-lg max-w-screen-xs">
      <ClientOnly>
        <Search />
      </ClientOnly>
    </div>

    <div
      v-for="section in sidebar.sections"
      :key="section.title"
      class="mb-6 border-ui-border"
    >
      <SidebarSection :title="section.title" :items="section.items" />
    </div>
  </div>
</template>

<static-query>
query Sidebar {
  allSidebar {
    edges {
      node {
        id
        sections {
          title
          items
        }
      }
    }
  }
}
</static-query>

<script>
import SidebarDropdown from "~/components/SidebarDropdown.vue";
import SidebarSection from "~/components/SidebarSection.vue";

const Search = () =>
  import(
    /* webpackChunkName: "search" */ "@/components/Search"
  ).catch((error) => console.warn(error));

export default {
  components: {
    SidebarDropdown,
    SidebarSection,
    Search,
  },

  computed: {
    sidebar() {
      const sidebars = this.$static.allSidebar.edges.filter((edge) => {
        return edge.node.id === this.$page.markdownPage.version;
      });

      if (sidebars.length === 1) {
        return sidebars[0].node;
      }

      return null;
    },
  },
};
</script>
