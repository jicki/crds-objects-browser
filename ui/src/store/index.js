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
      console.log('sortedResources getter 被调用')
      console.log('state.resources:', state.resources)
      console.log('state.resources 类型:', typeof state.resources)
      console.log('state.resources 是否为数组:', Array.isArray(state.resources))
      console.log('state.resources 长度:', state.resources ? state.resources.length : 'null/undefined')
      
      // 确保总是返回数组
      if (!state.resources || !Array.isArray(state.resources)) {
        console.warn('state.resources 不是数组，返回空数组')
        return []
      }
      
      const sorted = [...state.resources].sort((a, b) => {
        if (a.group < b.group) return -1
        if (a.group > b.group) return 1
        if (a.name < b.name) return -1
        if (a.name > b.name) return 1
        return 0
      })
      
      console.log('排序后的资源数量:', sorted.length)
      return sorted
    }
  },
  mutations: {
    setResources(state, resources) {
      console.log('setResources mutation 被调用')
      console.log('传入的 resources:', resources)
      console.log('传入的 resources 类型:', typeof resources)
      console.log('传入的 resources 是否为数组:', Array.isArray(resources))
      console.log('传入的 resources 长度:', resources ? resources.length : 'null/undefined')
      
      // 确保响应性更新
      state.resources = Array.isArray(resources) ? [...resources] : []
      
      console.log('设置后的 state.resources:', state.resources)
      console.log('设置后的 state.resources 长度:', state.resources ? state.resources.length : 'null/undefined')
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
        const url = `${API_URL}/crds`
        console.log('开始获取CRD资源...')
        console.log('请求URL:', url)
        console.log('API_URL:', API_URL)
        console.log('window.location.origin:', window.location.origin)
        
        const response = await axios.get(url)
        console.log('API响应:', response)
        console.log('响应状态:', response.status)
        console.log('响应头:', response.headers)
        console.log('响应数据:', response.data)
        console.log('响应数据类型:', typeof response.data)
        console.log('响应数据长度:', Array.isArray(response.data) ? response.data.length : 'not array')
        
        if (response.data && Array.isArray(response.data)) {
          console.log('设置资源数据:', response.data.length, '个资源')
          console.log('即将调用 setResources mutation')
          commit('setResources', response.data)
          console.log('setResources mutation 调用完成')
        } else {
          console.error('响应数据格式不正确:', response.data)
          throw new Error('No valid data received')
        }
      } catch (error) {
        console.error('Failed to fetch resources:', error)
        console.error('错误详情:', error.response)
        console.error('错误消息:', error.message)
        console.error('错误堆栈:', error.stack)
        commit('setError', '获取资源列表失败：' + (error.response?.data?.error || error.message))
      } finally {
        console.log('设置 loading 为 false')
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
        // 不要因为命名空间获取失败而影响主要功能，只记录警告
        console.warn('获取命名空间列表失败，但不影响主要功能：' + (error.response?.data?.error || error.message))
        // 设置默认命名空间
        commit('setNamespaces', ['default', 'kube-system', 'kube-public'])
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
      
      // 修复group为空字符串时的URL构建问题
      const apiGroup = group || 'core'
      
      commit('setLoading', true)
      commit('setError', null)
      try {
        const url = `${API_URL}/crds/${apiGroup}/${version}/${name}/objects${namespace !== 'all' ? `?namespace=${namespace}` : ''}`
        console.log('API请求URL:', url)
        const response = await axios.get(url)
        if (response.data && Array.isArray(response.data)) {
          commit('setResourceObjects', response.data)
        } else {
          // 如果没有数据，设置空数组
          commit('setResourceObjects', [])
        }
      } catch (error) {
        console.warn('获取资源对象失败，使用空列表：', error.response?.data?.error || error.message)
        // 不显示错误信息，只设置空的对象列表
        commit('setResourceObjects', [])
      } finally {
        commit('setLoading', false)
      }
    },
    
    // 获取资源可用的命名空间
    async fetchResourceNamespaces({ commit, state }) {
      if (!state.selectedResource) return
      
      const { group, version, name } = state.selectedResource
      
      // 修复group为空字符串时的URL构建问题
      const apiGroup = group || 'core'
      
      try {
        const url = `${API_URL}/crds/${apiGroup}/${version}/${name}/namespaces`
        console.log('命名空间API请求URL:', url)
        const response = await axios.get(url)
        if (response.data) {
          commit('setResourceNamespaces', response.data)
        } else {
          commit('setResourceNamespaces', [])
        }
      } catch (error) {
        console.warn('获取资源命名空间失败，使用空列表：', error.response?.data?.error || error.message)
        // 不显示错误信息，只设置空的命名空间列表
        commit('setResourceNamespaces', [])
      }
    },
    
    // 设置当前命名空间并重新获取资源对象
    async setNamespace({ commit, dispatch }, namespace) {
      commit('setCurrentNamespace', namespace)
      await dispatch('fetchResourceObjects')
    }
  }
}) 