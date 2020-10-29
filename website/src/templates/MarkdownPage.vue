<template>
  <Layout>
    <div class="flex flex-wrap items-start justify-start">
      <div
        class="order-2 w-full md:w-1/3 sm:pl-4 md:pl-6 lg:pl-8 sticky"
        style="top: 5rem"
      >
        <OnThisPage />
      </div>

      <div class="order-1 w-full md:w-2/3">
        <h1 class="content" v-html="$page.markdownPage.title" />
        <div
          class="content overflow-x-auto"
          v-html="$page.markdownPage.content"
        />

        <div class="mt-8 pt-8 lg:mt-12 lg:pt-12 border-t border-ui-border">
          <NextPrevLinks />
        </div>
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
import OnThisPage from "@/components/OnThisPage.vue";
import NextPrevLinks from "@/components/NextPrevLinks.vue";

export default {
  components: {
    OnThisPage,
    NextPrevLinks,
  },

  metaInfo() {
    const title = this.$page.markdownPage.title;
    const description =
      this.$page.markdownPage.description || this.$page.markdownPage.excerpt;

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
</style>
