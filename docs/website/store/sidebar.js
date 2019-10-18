export const state = () => ({
  activeMenuPath: 'v0.2/en/guides',
  lang: 'en',
  version: 'v0.2',
  sections: {},
  menu: []
})

export const mutations = {
  setMenu(state, menu) {
    state.menu = menu
  },

  setSections(state, sections) {
    state.sections = sections
  },

  setLang(state, lang) {
    state.lang = lang
  },

  setVersion(state, version) {
    state.version = version
  },

  setActiveDoc(state, activeMenuPath) {
    state.activeMenuPath = activeMenuPath.replace('docs/', '')
  }
}
