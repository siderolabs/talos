<template>
  <div class="content" v-if="hasAny($page.markdownPage.version)">
    <h2>Official AMI Images</h2>
    <details>
      <summary>List of AMI images for each AWS region:</summary>
      <table>
        <thead>
          <th>Region</th>
          <th>Version</th>
          <th>Instance Type</th>
          <th>Architecture</th>
          <th>AMI</th>
        </thead>
        <tbody>
          <template v-for="image in filtered($page.markdownPage.version)">
            <tr :key="image">
              <td>
                {{ image.region }}
              </td>
              <td>
                {{ image.version }}
              </td>
              <td>hvm</td>
              <td>
                <code>{{ image.arch }}</code>
              </td>
              <td>
                <a :href="amiLaunchURL(image)" target="_blank">{{ image.id }}</a>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </details>
  </div>
</template>

<script>
import cloudImages from "@/data/cloud-images.json";

export default {
  data() {
    return {
      cloudImages,
    };
  },
  methods: {
    hasAny(version) {
        return this.filtered(version).length > 0;
    },

    filtered(version) {
      return this.cloudImages.filter((image) => {
        return image.cloud === "aws" && image.version.startsWith(version);
      }).sort((a, b) => (a.region < b.region || a.version < b.version ? -1 : 1));
    },

    amiLaunchURL(image) {
       return "https://console.aws.amazon.com/ec2/home?region=" + image.region + "#launchAmi=" + image.id;
    }
  },
};
</script>
