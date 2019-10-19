export const state = () => ({
  activeDocPath: '',
  lang: '',
  version: '',
  sections: {},
  menu: []
})

export const getters = {
  getMenu(state) {
    return state.menu
  },

  getSections(state) {
    return state.sections
  },

  getLang(state, lang) {
    return state.lang
  },

  getVersion(state) {
    return state.version
  },

  getActiveDocPath(state) {
    return state.activeDocPath
  }
}

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

  setActiveDocPath(state, activeDocPath) {
    state.activeDocPath = activeDocPath.replace('docs/', '')
  }
}
