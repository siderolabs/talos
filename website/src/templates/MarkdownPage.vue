<template>
  <Layout>
    <div>
      <h1 class="content" v-html="$page.markdownPage.title" />
      <AMIImages v-if="$page.markdownPage.title === 'AWS'" />
      <div
        class="content overflow-x-auto"
        v-html="$page.markdownPage.content"
      />

      <div class="mt-8 pt-8 lg:mt-12 lg:pt-12 border-t border-ui-border">
        <NextPrevLinks />
      </div>
    </div>
  </Layout>
</template>

<page-query>
query ($id: ID!) {
  markdownPage(id: $id) {
    id
    title
    description
    path
    timeToRead
    content
    version
    section
    next
    prev
    headings {
      depth
      value
      anchor
    }
  }
  allMarkdownPage{
    edges {
      node {
        path
        title
      }
    }
  }
}
</page-query>

<script>
import NextPrevLinks from "@/components/NextPrevLinks.vue";
import AMIImages from "@/components/AMIImages";

export default {
  components: {
    NextPrevLinks,
    AMIImages,
  },

  metaInfo() {
    const title = this.$page.markdownPage.title;
    const defaultDescription = "Talos is a modern OS designed to be secure, immutable, and minimal. Its purpose is to host Kubernetes clusters, so it is tightly integrated with Kubernetes.";
    const description =
      this.$page.markdownPage.description || this.$page.markdownPage.excerpt || defaultDescription;

    return {
      title: title,
      meta: [
        {
          name: "description",
          content: description,
        },
        {
          key: "og:title",
          name: "og:title",
          content: title,
        },
        {
          key: "twitter:title",
          name: "twitter:title",
          content: title,
        },
        {
          key: "og:description",
          name: "og:description",
          content: description,
        },
        {
          key: "twitter:description",
          name: "twitter:description",
          content: description,
        },
      ],
    };
  },
};
</script>

<style>
@import "prism-themes/themes/prism-material-oceanic.css";

code[class*="language-"],
pre[class*="language-"] {
  @apply text-sm;
}

hr {
  @apply my-4 border-t border-dashed border-ui-border;
}

.dt {
  @apply ml-8;
}

.dd {
  @apply mx-1 font-bold;
}
</style>
