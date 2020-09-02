<template>
  <header id="header">
    <div :class="{ 'w-4/5': narrowLogo }" class="flex flex-wrap mx-auto">
      <div class="flex py-2">
        <Logo></Logo>
      </div>
      <div class="flex py-6 ml-auto">
        <ul class="header-menu">
          <li>
            <a href="/docs/v0.6">
              <span class="font-semibold mr-1">Documentation</span>
            </a>
          </li>
          <li>
            <ProjectDropdown></ProjectDropdown>
          </li>
          <li class="">
            <CommunityDropdown></CommunityDropdown>
          </li>
          <li class="invisible lg:visible"><span class="divider"></span></li>
          <li>
            <EnterpriseLink />
          </li>
        </ul>
      </div>
    </div>
  </header>
</template>

<script>
import Logo from '~/components/Logo.vue'
import ProjectDropdown from '~/components/ProjectDropdown.vue'
import CommunityDropdown from '~/components/CommunityDropdown.vue'
import EnterpriseLink from '~/components/EnterpriseLink.vue'

export default {
  name: 'Header',

  components: {
    Logo,
    ProjectDropdown,
    CommunityDropdown,
    EnterpriseLink
  },
  props: {
    narrowLogo: {
      default: true,
      type: Boolean
    }
  }
}
</script>

<style scoped>
ul.header-menu {
  @apply flex flex-wrap items-center justify-end font-bold;
}

ul.header-menu li {
  @apply min-w-full;
  @screen md {
    @apply min-w-0;
  }
}

ul {
  list-style-type: none;
  margin: 0;
}

li {
  margin-left: 1.25rem;
  border-bottom-width: 4px;
  border-bottom-color: transparent;
}

/* this class doesn't do anything right now, but exists to match the style of the
   corp site and for future expansion. */
li:hover {
  border-bottom-width: 4px;
  border-bottom-color: transparent;
}

.divider {
  border-left-width: 4px;
  border-left-color: ddd;
}
</style>
