import { createStore } from 'vuex'
import axios from 'axios'

// API基础URL
const API_URL = window.location.origin + '/api'

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
      commit('setError', null)
      try {
        const response = await axios.get(`${API_URL}/crds`)
        if (response.data) {
          commit('setResources', response.data)
        } else {
          throw new Error('No data received')
        }
      } catch (error) {
        console.error('Failed to fetch resources:', error)
        commit('setError', '获取资源列表失败：' + (error.response?.data?.error || error.message))
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 获取所有命名空间
    async fetchNamespaces({ commit }) {
      commit('setLoading', true)
      commit('setError', null)
      try {
        const response = await axios.get(`${API_URL}/namespaces`)
        if (response.data) {
          commit('setNamespaces', response.data)
        } else {
          throw new Error('No data received')
        }
      } catch (error) {
        console.error('Failed to fetch namespaces:', error)
        commit('setError', '获取命名空间列表失败：' + (error.response?.data?.error || error.message))
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
      commit('setError', null)
      try {
        const url = `${API_URL}/crds/${group}/${version}/${name}/objects${namespace !== 'all' ? `?namespace=${namespace}` : ''}`
        const response = await axios.get(url)
        if (response.data) {
          commit('setResourceObjects', response.data)
        } else {
          throw new Error('No data received')
        }
      } catch (error) {
        console.error('Failed to fetch resource objects:', error)
        commit('setError', '获取资源对象失败：' + (error.response?.data?.error || error.message))
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 获取资源可用的命名空间
    async fetchResourceNamespaces({ commit, state }) {
      if (!state.selectedResource) return
      
      const { group, version, name } = state.selectedResource
      
      commit('setLoading', true)
      commit('setError', null)
      try {
        const url = `${API_URL}/crds/${group}/${version}/${name}/namespaces`
        const response = await axios.get(url)
        if (response.data) {
          commit('setResourceNamespaces', response.data)
        } else {
          throw new Error('No data received')
        }
      } catch (error) {
        console.error('Failed to fetch resource namespaces:', error)
        commit('setError', '获取资源命名空间失败：' + (error.response?.data?.error || error.message))
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