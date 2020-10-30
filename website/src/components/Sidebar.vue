<template>
  <div>
    <SidebarDropdown />

    <div class="mb-4 max-w-lg max-w-screen-xs">
      <ClientOnly>
        <SidebarSearch />
      </ClientOnly>
    </div>

    <div
      v-for="section in sidebar.sections"
      :key="section.title"
      class="mb-4 border-ui-border"
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

const SidebarSearch = () =>
  import(
    /* webpackChunkName: "search" */ "@/components/SidebarSearch"
  ).catch((error) => console.warn(error));

export default {
  components: {
    SidebarDropdown,
    SidebarSection,
    SidebarSearch,
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
