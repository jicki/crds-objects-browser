<template>
  <div class="resources-layout">
    <el-container>
      <el-aside width="300px">
        <div class="sidebar">
          <div class="header">
            <h2>Kubernetes CRD 浏览器</h2>
          </div>
          <div class="search-box">
            <el-input
              v-model="searchQuery"
              placeholder="🔍 搜索资源类型..."
              clearable
              size="large"
              style="width: 100%;"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>
          <div class="resources-list">
            <el-alert
              v-if="error"
              :title="error"
              type="error"
              :closable="false"
              show-icon
              style="margin-bottom: 15px;"
            />
            
            <!-- 调试信息 -->
            <div class="debug-info" style="background: #f0f0f0; padding: 10px; margin-bottom: 10px; font-size: 12px; border-radius: 4px;">
              <div style="color: #606266;">
                <span style="font-weight: bold;">📊 状态信息</span>
              </div>
              <div style="margin-top: 5px;">
                <span style="color: #909399;">加载状态:</span> 
                <el-tag :type="loading ? 'warning' : 'success'" size="small">
                  {{ loading ? '加载中...' : '已完成' }}
                </el-tag>
              </div>
              <div v-if="error" style="margin-top: 5px;">
                <span style="color: #F56C6C;">⚠️ 错误:</span> 
                <el-tag type="danger" size="small">{{ error }}</el-tag>
              </div>
              <div style="margin-top: 5px;">
                <span style="color: #67C23A;">📦 资源总数:</span> 
                <el-tag type="success" size="small">{{ sortedResources ? sortedResources.length : 0 }}</el-tag>
              </div>
              <div style="margin-top: 5px;">
                <span style="color: #409EFF;">🌳 分组数量:</span> 
                <el-tag type="primary" size="small">{{ resourcesTree ? resourcesTree.length : 0 }}</el-tag>
              </div>
              <div style="margin-top: 8px;">
                <el-button @click="refreshData" size="small" type="primary" plain>
                  <el-icon><Refresh /></el-icon>
                  刷新数据
                </el-button>
              </div>
            </div>
            
            <el-skeleton v-if="loading" :rows="6" animated />
            <el-empty v-else-if="!resourcesTree.length" description="暂无资源">
              <template #image>
                <div style="font-size: 60px; color: #909399;">📦</div>
              </template>
            </el-empty>
            <el-tree
              v-else
              :data="resourcesTree"
              :props="defaultProps"
              @node-click="handleNodeClick"
              node-key="id"
              :filter-node-method="filterNode"
              ref="resourcesTreeRef"
              highlight-current
              :expand-on-click-node="false"
              :check-strictly="true"
              :default-expanded-keys="defaultExpandedKeys"
            >
              <template #default="{ node, data }">
                <span 
                  class="custom-tree-node"
                  :class="{ 
                    'clickable': data.resource, 
                    'group-node': !data.resource && !data.isResourceGroup,
                    'resource-group-node': data.isResourceGroup,
                    'version-node': data.isVersion
                  }"
                  @click.stop="data.resource && handleResourceClick(data.resource)"
                >
                  <el-icon v-if="!data.resource && !data.isResourceGroup" class="group-icon">
                    <component :is="getGroupIcon(data)" />
                  </el-icon>
                  <el-icon v-else-if="data.isResourceGroup" class="resource-group-icon">
                    <Folder />
                  </el-icon>
                  <el-icon v-else-if="data.resource" class="resource-icon">
                    <component :is="getResourceIcon(data.resource)" />
                  </el-icon>
                  <span :class="getNodeClass(data)">{{ getDisplayLabel(node.label) }}</span>
                  <el-tag v-if="data.children && !data.isResourceGroup" size="small" type="info" class="count-tag">
                    {{ data.children.length }}
                  </el-tag>
                  <el-tag v-else-if="data.children && data.isResourceGroup" size="small" type="warning" class="version-count-tag">
                    {{ data.children.length }} 版本
                  </el-tag>
                </span>
              </template>
            </el-tree>
          </div>
        </div>
      </el-aside>
      <el-container>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script>
import { computed, ref, watch, nextTick, onMounted } from 'vue'
import { useStore } from 'vuex'
import { useRouter } from 'vue-router'
import { Search, Refresh, Box, Setting, Folder, Monitor, Connection, Grid, Document, Key, Link as LinkIcon, Timer, FolderOpened, User, DocumentCopy } from '@element-plus/icons-vue'

export default {
  name: 'ResourcesLayout',
  components: {
    Search,
    Refresh,
    Box,
    Setting,
    Folder,
    Monitor,
    Connection,
    Grid,
    Document,
    Key,
    LinkIcon,
    Timer,
    FolderOpened,
    User,
    DocumentCopy
  },
  setup() {
    const store = useStore()
    const router = useRouter()
    const resourcesTree = ref([])
    const searchQuery = ref('')
    const resourcesTreeRef = ref(null)
    const defaultExpandedKeys = ref([])

    // 初始化时加载资源
    store.dispatch('fetchResources')
    store.dispatch('fetchNamespaces')

    // 添加滚动位置监听器
    const setupScrollListener = () => {
      const resourcesList = document.querySelector('.resources-list')
      if (resourcesList) {
        resourcesList.addEventListener('scroll', () => {
          const scrollTop = resourcesList.scrollTop
          localStorage.setItem('resourcesListScrollTop', scrollTop.toString())
        })
      }
    }

    // 页面加载完成后设置监听器
    nextTick(() => {
      setTimeout(setupScrollListener, 500)
    })

    // 监听搜索查询变化
    watch(searchQuery, (val) => {
      resourcesTreeRef.value?.filter(val)
    })

    // 将资源列表转换为树形结构
    const buildResourcesTree = (resources) => {
      // 确保resources是数组
      if (!resources || !Array.isArray(resources) || resources.length === 0) {
        console.log('buildResourcesTree: 资源为空或不是数组')
        return []
      }
      
      const k8sResources = []
      const crdResources = []
      
      // 分离K8s默认资源和CRD资源
      resources.forEach(resource => {
        const isK8sCore = resource.group === '' || 
                         resource.group === 'apps' || 
                         resource.group === 'batch' || 
                         resource.group === 'networking.k8s.io' ||
                         resource.group === 'rbac.authorization.k8s.io'
        
        if (isK8sCore) {
          k8sResources.push(resource)
        } else {
          crdResources.push(resource)
        }
      })
      
      const result = []
      
      // 添加K8s默认资源组
      if (k8sResources.length > 0) {
        const k8sGroupMap = new Map()
        
        k8sResources.forEach(resource => {
          let groupName = resource.group || 'core'
          
          // 友好的组名显示
          const groupDisplayNames = {
            '': 'Kubernetes Core',
            'apps': 'Apps',
            'batch': 'Batch',
            'networking.k8s.io': 'Networking',
            'rbac.authorization.k8s.io': 'RBAC'
          }
          
          const displayName = groupDisplayNames[resource.group] || groupName
          
          if (!k8sGroupMap.has(groupName)) {
            k8sGroupMap.set(groupName, {
              id: `k8s-${groupName}`,
              label: `📦 ${displayName}`,
              children: [],
              resourceMap: new Map() // 用于按资源名分组
            })
          }
          
          const groupNode = k8sGroupMap.get(groupName)
          const resourceName = resource.name
          
          // 检查是否已有同名资源
          if (!groupNode.resourceMap.has(resourceName)) {
            groupNode.resourceMap.set(resourceName, {
              id: `k8s-${groupName}-${resourceName}`,
              label: resourceName,
              children: [],
              isResourceGroup: true
            })
          }
          
          const resourceGroup = groupNode.resourceMap.get(resourceName)
          resourceGroup.children.push({
            id: `${resource.group}/${resource.version}/${resource.name}`,
            label: `${resource.version}`,
            resource: resource,
            isVersion: true
          })
        })
        
        // 处理K8s资源组
        Array.from(k8sGroupMap.values())
          .sort((a, b) => a.label.localeCompare(b.label))
          .forEach(group => {
            // 将resourceMap转换为children数组
            group.children = Array.from(group.resourceMap.values())
              .sort((a, b) => a.label.localeCompare(b.label))
              .map(resourceGroup => {
                // 如果只有一个版本，直接显示资源
                if (resourceGroup.children.length === 1) {
                  const singleVersion = resourceGroup.children[0]
                  return {
                    id: singleVersion.id,
                    label: `${resourceGroup.label} (${singleVersion.label})`,
                    resource: singleVersion.resource
                  }
                } else {
                  // 多个版本，显示为子节点
                  resourceGroup.children.sort((a, b) => b.label.localeCompare(a.label)) // 版本倒序
                  return resourceGroup
                }
              })
            
            delete group.resourceMap // 清理临时属性
            result.push(group)
          })
      }
      
      // 添加CRD资源组
      if (crdResources.length > 0) {
        const crdGroupMap = new Map()
        
        crdResources.forEach(resource => {
          const groupName = resource.group || 'core'
          
          if (!crdGroupMap.has(groupName)) {
            crdGroupMap.set(groupName, {
              id: `crd-${groupName}`,
              label: `🔧 ${groupName}`,
              children: [],
              resourceMap: new Map()
            })
          }
          
          const groupNode = crdGroupMap.get(groupName)
          const resourceName = resource.name
          
          // 检查是否已有同名资源
          if (!groupNode.resourceMap.has(resourceName)) {
            groupNode.resourceMap.set(resourceName, {
              id: `crd-${groupName}-${resourceName}`,
              label: resourceName,
              children: [],
              isResourceGroup: true
            })
          }
          
          const resourceGroup = groupNode.resourceMap.get(resourceName)
          resourceGroup.children.push({
            id: `${resource.group}/${resource.version}/${resource.name}`,
            label: `${resource.version}`,
            resource: resource,
            isVersion: true
          })
        })
        
        // 处理CRD资源组
        Array.from(crdGroupMap.values())
          .sort((a, b) => a.label.localeCompare(b.label))
          .forEach(group => {
            // 将resourceMap转换为children数组
            group.children = Array.from(group.resourceMap.values())
              .sort((a, b) => a.label.localeCompare(b.label))
              .map(resourceGroup => {
                // 如果只有一个版本，直接显示资源
                if (resourceGroup.children.length === 1) {
                  const singleVersion = resourceGroup.children[0]
                  return {
                    id: singleVersion.id,
                    label: `${resourceGroup.label} (${singleVersion.label})`,
                    resource: singleVersion.resource
                  }
                } else {
                  // 多个版本，显示为子节点
                  resourceGroup.children.sort((a, b) => b.label.localeCompare(a.label)) // 版本倒序
                  return resourceGroup
                }
              })
            
            delete group.resourceMap // 清理临时属性
            result.push(group)
          })
      }
      
      return result
    }

    // 从Store获取排序后的资源列表
    const sortedResources = computed(() => store.getters.sortedResources)
    const loading = computed(() => store.state.loading)
    const error = computed(() => store.state.error)
    
    // 监听资源列表变化，重建树形结构
    watch(sortedResources, (resources) => {
      console.log('sortedResources 变化:', resources)
      console.log('资源数量:', resources ? resources.length : 0)
      
      // 确保resources是有效的数组
      if (resources && Array.isArray(resources) && resources.length > 0) {
        const newTree = buildResourcesTree(resources)
        resourcesTree.value = newTree
        console.log('构建的树结构:', newTree)
        
        // 恢复展开状态
        nextTick(() => {
          restoreTreeState()
        })
      } else {
        resourcesTree.value = []
        console.log('resources为空或无效，设置树结构为空数组')
      }
    }, { immediate: true, deep: true })

    // 恢复树形组件状态
    const restoreTreeState = () => {
      console.log('开始恢复树形组件状态')
      
      // 恢复展开的节点
      const savedExpandedKeys = localStorage.getItem('expandedKeys')
      if (savedExpandedKeys) {
        try {
          const expandedKeys = JSON.parse(savedExpandedKeys)
          console.log('恢复展开状态:', expandedKeys)
          defaultExpandedKeys.value = expandedKeys
          
          // 延迟展开节点，确保DOM已渲染
          setTimeout(() => {
            if (resourcesTreeRef.value && expandedKeys.length > 0) {
              expandedKeys.forEach(key => {
                const node = resourcesTreeRef.value.getNode(key)
                if (node) {
                  node.expanded = true
                }
              })
            }
          }, 100)
        } catch (e) {
          console.warn('恢复展开状态失败:', e)
        }
      } else {
        // 如果没有保存的状态，默认展开所有组节点
        const groupKeys = resourcesTree.value.map(group => group.id)
        defaultExpandedKeys.value = groupKeys
        console.log('使用默认展开状态:', groupKeys)
      }
      
      // 恢复选中的节点
      const selectedKey = localStorage.getItem('selectedResourceKey')
      if (selectedKey && resourcesTreeRef.value) {
        setTimeout(() => {
          resourcesTreeRef.value.setCurrentKey(selectedKey)
          console.log('恢复选中节点:', selectedKey)
        }, 150)
      }
      
      // 恢复滚动位置
      const savedScrollTop = localStorage.getItem('resourcesListScrollTop')
      if (savedScrollTop) {
        setTimeout(() => {
          const resourcesList = document.querySelector('.resources-list')
          if (resourcesList) {
            const scrollTop = parseInt(savedScrollTop, 10)
            resourcesList.scrollTop = scrollTop
            console.log('恢复滚动位置:', scrollTop)
          }
        }, 300)
      }
    }

    // 监听store状态变化
    watch(() => store.state.resources, (resources) => {
      console.log('store.state.resources 变化:', resources)
      console.log('原始资源数量:', resources ? resources.length : 0)
      
      // 强制触发computed重新计算
      if (resources && Array.isArray(resources) && resources.length > 0) {
        console.log('检测到资源数据，强制更新...')
        // 触发getter重新计算
        const sorted = store.getters.sortedResources
        console.log('重新获取的sortedResources长度:', sorted ? sorted.length : 0)
      }
    }, { immediate: true, deep: true })

    // 处理树节点点击
    const handleNodeClick = (node) => {
      // 只处理资源节点的点击
      if (node.resource) {
        handleResourceClick(node.resource)
      }
    }

    // 处理资源点击
    const handleResourceClick = (resource) => {
      console.log('=== handleResourceClick 开始 ===')
      console.log('点击的资源:', resource)
      console.log('资源详情:', JSON.stringify(resource, null, 2))
      
      // 检查资源对象是否有效
      if (!resource || resource.group === undefined || !resource.version || !resource.name) {
        console.error('资源对象无效:', resource)
        return
      }
      
      // 记住当前选中的节点
      const currentKey = `${resource.group}/${resource.version}/${resource.name}`
      localStorage.setItem('selectedResourceKey', currentKey)
      console.log('保存选中的资源key:', currentKey)
      
      // 保存当前滚动位置 - 使用更可靠的方法
      const resourcesList = document.querySelector('.resources-list')
      if (resourcesList) {
        const scrollTop = resourcesList.scrollTop
        localStorage.setItem('resourcesListScrollTop', scrollTop.toString())
        console.log('保存滚动位置:', scrollTop)
      }
      
      // 保存展开的节点
      if (resourcesTreeRef.value) {
        const expandedKeys = []
        const traverse = (nodes) => {
          nodes.forEach(node => {
            const treeNode = resourcesTreeRef.value.getNode(node.id)
            if (treeNode && treeNode.expanded) {
              expandedKeys.push(node.id)
            }
            if (node.children) {
              traverse(node.children)
            }
          })
        }
        traverse(resourcesTree.value)
        localStorage.setItem('expandedKeys', JSON.stringify(expandedKeys))
        console.log('保存展开状态:', expandedKeys)
      }
      
      // 先选择资源
      console.log('调用 store.dispatch selectResource')
      try {
        store.dispatch('selectResource', resource)
        console.log('selectResource 调用成功')
      } catch (error) {
        console.error('selectResource 调用失败:', error)
        return
      }
      
      // 构建路由参数
      let routeName, routeParams
      
      if (!resource.group || resource.group === '') {
        // Kubernetes Core资源（group为空）
        routeName = 'CoreResourceDetail'
        routeParams = {
          version: resource.version,
          resource: resource.name
        }
      } else {
        // 其他资源（有group）
        routeName = 'ResourceDetail'
        routeParams = {
          group: resource.group,
          version: resource.version,
          resource: resource.name
        }
      }
      
      console.log('准备跳转路由，名称:', routeName, '参数:', routeParams)
      console.log('当前路由:', router.currentRoute.value)
      
      // 使用replace避免历史记录问题，并立即恢复滚动位置
      console.log('开始路由跳转...')
      router.replace({
        name: routeName,
        params: routeParams
      }).then(() => {
        console.log('路由跳转成功')
        console.log('跳转后的路由:', router.currentRoute.value)
        // 路由跳转完成后立即恢复滚动位置
        setTimeout(() => {
          const savedScrollTop = localStorage.getItem('resourcesListScrollTop')
          if (savedScrollTop) {
            const resourcesList = document.querySelector('.resources-list')
            if (resourcesList) {
              const scrollTop = parseInt(savedScrollTop, 10)
              resourcesList.scrollTop = scrollTop
              console.log('路由跳转后恢复滚动位置:', scrollTop)
            }
          }
        }, 50)
      }).catch(error => {
        console.error('路由跳转失败:', error)
        console.error('错误详情:', error.message)
        console.error('错误堆栈:', error.stack)
      })
      
      console.log('=== handleResourceClick 结束 ===')
    }

    // 过滤节点的方法
    const filterNode = (value, data) => {
      if (!value) return true
      return data.label.toLowerCase().includes(value.toLowerCase())
    }

    const refreshData = () => {
      store.dispatch('fetchResources')
      store.dispatch('fetchNamespaces')
    }

    // 获取组图标
    const getGroupIcon = (data) => {
      const label = data.label.toLowerCase()
      if (label.includes('📦')) {
        return 'Box'
      } else if (label.includes('🔧')) {
        return 'Setting'
      }
      return 'Folder'
    }

    // 获取资源图标
    const getResourceIcon = (resource) => {
      const kind = resource.kind.toLowerCase()
      
      // 根据资源类型返回不同图标
      if (kind.includes('pod')) return 'Monitor'
      if (kind.includes('service')) return 'Connection'
      if (kind.includes('deployment')) return 'Grid'
      if (kind.includes('configmap')) return 'Document'
      if (kind.includes('secret')) return 'Key'
      if (kind.includes('ingress')) return 'LinkIcon'
      if (kind.includes('job')) return 'Timer'
      if (kind.includes('namespace')) return 'FolderOpened'
      if (kind.includes('node')) return 'Monitor'
      if (kind.includes('role')) return 'User'
      
      return 'DocumentCopy'
    }

    // 获取节点样式类
    const getNodeClass = (data) => {
      if (data.isResourceGroup) {
        return 'resource-group-label'
      } else if (data.isVersion) {
        return 'version-label'
      } else if (!data.resource) {
        return data.label.includes('📦') ? 'k8s-group-label' : 'crd-group-label'
      }
      return 'resource-label'
    }

    // 获取显示标签（去掉emoji）
    const getDisplayLabel = (label) => {
      return label.replace(/📦|🔧|📁|📄/g, '').trim()
    }

    // 强制保持滚动位置
    const forceKeepScrollPosition = () => {
      const savedScrollTop = localStorage.getItem('resourcesListScrollTop')
      if (savedScrollTop) {
        const scrollTop = parseInt(savedScrollTop, 10)
        
        // 多次尝试恢复滚动位置
        const attempts = [50, 100, 200, 300, 500]
        attempts.forEach(delay => {
          setTimeout(() => {
            const resourcesList = document.querySelector('.resources-list')
            if (resourcesList && resourcesList.scrollTop !== scrollTop) {
              resourcesList.scrollTop = scrollTop
              console.log(`第${delay}ms尝试恢复滚动位置:`, scrollTop)
            }
          }, delay)
        })
      }
    }

    // 监听路由变化
    watch(() => router.currentRoute.value.path, () => {
      forceKeepScrollPosition()
    })

    // 组件挂载时立即恢复滚动位置
    onMounted(() => {
      // 立即尝试恢复滚动位置
      forceKeepScrollPosition()
      
      // 确保在DOM完全渲染后再次恢复
      nextTick(() => {
        setTimeout(() => {
          forceKeepScrollPosition()
        }, 100)
      })
    })

    return {
      searchQuery,
      resourcesTree,
      resourcesTreeRef,
      loading,
      error,
      sortedResources,
      defaultProps: {
        children: 'children',
        label: 'label'
      },
      handleNodeClick,
      handleResourceClick,
      filterNode,
      refreshData,
      restoreTreeState,
      getGroupIcon,
      getResourceIcon,
      getNodeClass,
      getDisplayLabel,
      defaultExpandedKeys
    }
  }
}
</script>

<style scoped>
.resources-layout {
  height: 100%;
}

.el-container {
  height: 100%;
}

.el-aside {
  background-color: #f5f7fa;
  border-right: 1px solid #e6e6e6;
  height: 100%;
}

.sidebar {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  padding: 20px;
  border-bottom: 1px solid #e6e6e6;
}

.header h2 {
  margin: 0;
  font-size: 18px;
  color: #303133;
}

.search-box {
  padding: 15px;
  border-bottom: 1px solid #e6e6e6;
}

.resources-list {
  flex: 1;
  overflow-y: auto;
  padding: 15px;
  scroll-behavior: auto;
  position: relative;
}

.el-main {
  padding: 20px;
  background-color: #fff;
}

:deep(.el-tree-node__content) {
  height: 32px;
}

:deep(.el-tree-node.is-current > .el-tree-node__content) {
  background-color: #ecf5ff;
  color: #409eff;
}

.custom-tree-node {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 4px 0;
}

.custom-tree-node.clickable {
  cursor: pointer;
  transition: all 0.2s ease;
}

.custom-tree-node.clickable:hover {
  background-color: rgba(64, 158, 255, 0.1);
  border-radius: 4px;
  padding: 4px 8px;
}

.custom-tree-node.group-node {
  cursor: default;
  font-weight: 600;
}

.group-icon {
  color: #409eff;
  font-size: 16px;
}

.resource-icon {
  color: #67c23a;
  font-size: 14px;
}

.resource-group-icon {
  color: #e6a23c;
  font-size: 14px;
}

.k8s-group-label {
  font-weight: 600;
  color: #409eff;
  font-size: 14px;
}

.crd-group-label {
  font-weight: 600;
  color: #e6a23c;
  font-size: 14px;
}

.resource-group-label {
  font-weight: 500;
  color: #606266;
  font-size: 13px;
}

.resource-label {
  color: #606266;
  font-size: 13px;
}

.version-label {
  color: #909399;
  font-size: 12px;
  font-style: italic;
}

.count-tag {
  margin-left: auto;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 10px;
}

.version-count-tag {
  margin-left: auto;
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 8px;
}

/* 树形组件样式优化 */
:deep(.el-tree) {
  background: transparent;
}

:deep(.el-tree-node__content) {
  height: 36px;
  border-radius: 6px;
  margin: 2px 0;
  transition: all 0.3s ease;
}

:deep(.el-tree-node__content:hover) {
  background-color: #f0f9ff;
  transform: translateX(4px);
}

:deep(.el-tree-node.is-current > .el-tree-node__content) {
  background: linear-gradient(135deg, #409eff 0%, #67c23a 100%);
  color: white;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
}

:deep(.el-tree-node.is-current .k8s-group-label),
:deep(.el-tree-node.is-current .crd-group-label),
:deep(.el-tree-node.is-current .resource-label) {
  color: white;
}

:deep(.el-tree-node.is-current .group-icon),
:deep(.el-tree-node.is-current .resource-icon) {
  color: white;
}

:deep(.el-tree-node__expand-icon) {
  color: #409eff;
  font-size: 14px;
}

:deep(.el-tree-node__expand-icon.expanded) {
  transform: rotate(90deg);
}

/* 侧边栏整体优化 */
.sidebar {
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
  border-right: 1px solid #e2e8f0;
}

.header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border-bottom: none;
}

.search-box {
  background: rgba(255, 255, 255, 0.9);
  border-bottom: 1px solid #e2e8f0;
}

:deep(.el-empty__image) {
  display: flex;
  justify-content: center;
  align-items: center;
  color: #909399;
}
</style> 