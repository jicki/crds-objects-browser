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
              placeholder="搜索资源类型"
              prefix-icon="el-icon-search"
              clearable
            />
          </div>
          <div class="resources-list">
            <el-tree
              :data="resourcesTree"
              :props="defaultProps"
              @node-click="handleNodeClick"
              node-key="id"
              :filter-node-method="filterNode"
              ref="resourcesTree"
              highlight-current
              default-expand-all
            />
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

export default {
  name: 'ResourcesLayout',
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
      const groupMap = new Map()
      
      resources.forEach(resource => {
        const groupName = resource.group || 'core'
        
        if (!groupMap.has(groupName)) {
          groupMap.set(groupName, {
            id: groupName,
            label: groupName,
            children: []
          })
        }
        
        const groupNode = groupMap.get(groupName)
        groupNode.children.push({
          id: `${resource.group}/${resource.version}/${resource.name}`,
          label: resource.name,
          resource: resource
        })
      })
      
      // 按字母顺序排序组和资源
      return Array.from(groupMap.values())
        .sort((a, b) => a.label.localeCompare(b.label))
        .map(group => {
          group.children.sort((a, b) => a.label.localeCompare(b.label))
          return group
        })
    }

    // 从Store获取排序后的资源列表
    const sortedResources = computed(() => store.getters.sortedResources)
    
    // 监听资源列表变化，重建树形结构
    watch(sortedResources, (resources) => {
      resourcesTree.value = buildResourcesTree(resources)
    })

    // 处理树节点点击
    const handleNodeClick = (node) => {
      if (node.resource) {
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

    return {
      searchQuery,
      resourcesTree,
      resourcesTreeRef,
      defaultProps: {
        children: 'children',
        label: 'label'
      },
      handleNodeClick,
      filterNode
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
</style> 