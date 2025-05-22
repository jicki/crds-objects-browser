import { createStore } from 'vuex'
import axios from 'axios'

// API基础URL
const API_URL = '/api'

export default createStore({
  state: {
    resources: [],
    namespaces: [],
    selectedResource: null,
    resourceObjects: [],
    resourceNamespaces: [],
    currentNamespace: 'all',
    loading: false,
    error: null
  },
  getters: {
    sortedResources(state) {
      return [...state.resources].sort((a, b) => {
        if (a.group < b.group) return -1
        if (a.group > b.group) return 1
        if (a.name < b.name) return -1
        if (a.name > b.name) return 1
        return 0
      })
    }
  },
  mutations: {
    setResources(state, resources) {
      state.resources = resources
    },
    setNamespaces(state, namespaces) {
      state.namespaces = namespaces
    },
    setSelectedResource(state, resource) {
      state.selectedResource = resource
    },
    setResourceObjects(state, objects) {
      state.resourceObjects = objects
    },
    setResourceNamespaces(state, namespaces) {
      state.resourceNamespaces = namespaces
    },
    setCurrentNamespace(state, namespace) {
      state.currentNamespace = namespace
    },
    setLoading(state, isLoading) {
      state.loading = isLoading
    },
    setError(state, error) {
      state.error = error
    }
  },
  actions: {
    // 获取所有CRD资源
    async fetchResources({ commit }) {
      commit('setLoading', true)
      try {
        const response = await axios.get(`${API_URL}/crds`)
        commit('setResources', response.data)
      } catch (error) {
        console.error('Failed to fetch resources:', error)
        commit('setError', 'Failed to fetch resources')
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 获取所有命名空间
    async fetchNamespaces({ commit }) {
      commit('setLoading', true)
      try {
        const response = await axios.get(`${API_URL}/namespaces`)
        commit('setNamespaces', response.data)
      } catch (error) {
        console.error('Failed to fetch namespaces:', error)
        commit('setError', 'Failed to fetch namespaces')
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 选择资源
    selectResource({ commit }, resource) {
      commit('setSelectedResource', resource)
      commit('setResourceObjects', [])
      commit('setCurrentNamespace', 'all')
    },
    
    // 获取资源对象
    async fetchResourceObjects({ commit, state }) {
      if (!state.selectedResource) return
      
      const { group, version, name } = state.selectedResource
      const namespace = state.currentNamespace
      
      commit('setLoading', true)
      try {
        let url = `${API_URL}/resources/${group}/${version}/${name}`
        if (namespace && namespace !== 'all') {
          url += `?namespace=${namespace}`
        }
        
        const response = await axios.get(url)
        commit('setResourceObjects', response.data)
      } catch (error) {
        console.error('Failed to fetch resource objects:', error)
        commit('setError', 'Failed to fetch resource objects')
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 获取资源可用的命名空间
    async fetchResourceNamespaces({ commit, state }) {
      if (!state.selectedResource) return
      
      const { group, version, name } = state.selectedResource
      
      commit('setLoading', true)
      try {
        const url = `${API_URL}/resources/${group}/${version}/${name}/namespaces`
        const response = await axios.get(url)
        commit('setResourceNamespaces', response.data)
      } catch (error) {
        console.error('Failed to fetch resource namespaces:', error)
        commit('setError', 'Failed to fetch resource namespaces')
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 设置当前命名空间并重新获取资源对象
    async setNamespace({ commit, dispatch }, namespace) {
      commit('setCurrentNamespace', namespace)
      await dispatch('fetchResourceObjects')
    }
  }
}) 