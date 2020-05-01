export const state = () => ({
  lang: '',
  version: '',
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
  }
}
