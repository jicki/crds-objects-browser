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
              default-expand-all
              :expand-on-click-node="false"
            >
              <template #default="{ node, data }">
                <span class="custom-tree-node">
                  <el-icon v-if="!data.resource" class="group-icon">
                    <component :is="getGroupIcon(data)" />
                  </el-icon>
                  <el-icon v-else class="resource-icon">
                    <component :is="getResourceIcon(data.resource)" />
                  </el-icon>
                  <span :class="getNodeClass(data)">{{ getDisplayLabel(node.label) }}</span>
                  <el-tag v-if="data.children" size="small" type="info" class="count-tag">
                    {{ data.children.length }}
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
import { computed, ref, watch } from 'vue'
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

    // 初始化时加载资源
    store.dispatch('fetchResources')
    store.dispatch('fetchNamespaces')

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
              children: []
            })
          }
          
          const groupNode = k8sGroupMap.get(groupName)
          groupNode.children.push({
            id: `${resource.group}/${resource.version}/${resource.name}`,
            label: resource.name,
            resource: resource
          })
        })
        
        // 添加K8s资源组到结果
        Array.from(k8sGroupMap.values())
          .sort((a, b) => a.label.localeCompare(b.label))
          .forEach(group => {
            group.children.sort((a, b) => a.label.localeCompare(b.label))
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
              children: []
            })
          }
          
          const groupNode = crdGroupMap.get(groupName)
          groupNode.children.push({
            id: `${resource.group}/${resource.version}/${resource.name}`,
            label: resource.name,
            resource: resource
          })
        })
        
        // 添加CRD资源组到结果
        Array.from(crdGroupMap.values())
          .sort((a, b) => a.label.localeCompare(b.label))
          .forEach(group => {
            group.children.sort((a, b) => a.label.localeCompare(b.label))
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
        
        // 恢复之前选中的节点
        setTimeout(() => {
          const selectedKey = localStorage.getItem('selectedResourceKey')
          if (selectedKey && resourcesTreeRef.value) {
            resourcesTreeRef.value.setCurrentKey(selectedKey)
          }
        }, 100)
      } else {
        resourcesTree.value = []
        console.log('resources为空或无效，设置树结构为空数组')
      }
    }, { immediate: true, deep: true })

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
      if (node.resource) {
        // 记住当前选中的节点
        const currentKey = `${node.resource.group}/${node.resource.version}/${node.resource.name}`
        localStorage.setItem('selectedResourceKey', currentKey)
        
        store.dispatch('selectResource', node.resource)
        router.push({
          name: 'ResourceDetail',
          params: {
            group: node.resource.group,
            version: node.resource.version,
            resource: node.resource.name
          }
        })
      }
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
      if (!data.resource) {
        return data.label.includes('📦') ? 'k8s-group-label' : 'crd-group-label'
      }
      return 'resource-label'
    }

    // 获取显示标签（去掉emoji）
    const getDisplayLabel = (label) => {
      return label.replace(/📦|🔧|📁|📄/g, '').trim()
    }

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
      filterNode,
      refreshData,
      getGroupIcon,
      getResourceIcon,
      getNodeClass,
      getDisplayLabel
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

.group-icon {
  color: #409eff;
  font-size: 16px;
}

.resource-icon {
  color: #67c23a;
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

.resource-label {
  color: #606266;
  font-size: 13px;
}

.count-tag {
  margin-left: auto;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 10px;
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