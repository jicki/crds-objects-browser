<template>
  <div class="resource-detail">
    <div v-if="!selectedResource" class="no-resource">
      <p>请从左侧菜单选择一个资源</p>
    </div>
    <div v-else>
      <!-- 资源标题 -->
      <div class="resource-header">
        <h2>{{ resourceTitle }}</h2>
        
        <!-- 命名空间选择器 -->
        <div class="namespace-selector" v-if="selectedResource.namespaced">
          <el-select 
            v-model="currentNamespace" 
            placeholder="选择命名空间" 
            @change="handleNamespaceChange"
            size="large"
            style="width: 200px;"
          >
            <el-option key="all" label="🌐 所有命名空间" value="all" />
            <el-option
              v-for="ns in availableNamespaces"
              :key="ns"
              :label="`📁 ${ns}`"
              :value="ns"
            />
          </el-select>
        </div>
      </div>
      
      <!-- 统计信息和搜索 -->
      <div class="stats-and-search" v-if="!loading">
        <div class="stats-bar">
          <el-tag type="info" size="large">
            总计: {{ filteredObjects.length }} / {{ totalObjects }} 个对象
          </el-tag>
          <el-tag type="success" size="large" v-if="selectedResource.namespaced && currentNamespace !== 'all'">
            命名空间: {{ currentNamespace }}
          </el-tag>
          <el-tag type="warning" size="large" v-if="searchQuery">
            搜索: {{ searchQuery }}
          </el-tag>
        </div>
        
        <!-- 搜索框 -->
        <div class="search-container">
          <el-input
            v-model="searchQuery"
            placeholder="🔍 搜索资源名称、命名空间..."
            clearable
            size="large"
            style="width: 300px;"
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
      </div>
      
      <!-- 加载中提示 -->
      <el-skeleton v-if="loading" :rows="10" animated />
      
      <!-- 资源对象表格 -->
      <div v-else-if="paginatedObjects.length === 0" class="no-objects">
        <p>没有{{ selectedResource.name }}资源对象</p>
      </div>
      <div v-else class="resource-table">
        <el-table :data="paginatedObjects" style="width: 100%" border stripe>
          <el-table-column prop="name" label="名称" min-width="200" sortable />
          <el-table-column prop="namespace" label="命名空间" width="150" v-if="selectedResource.namespaced" sortable />
          <el-table-column prop="creationTimestamp" label="创建时间" width="200" sortable />
          
          <!-- 动态状态列 -->
          <el-table-column label="状态" width="150" align="center">
            <template #default="scope">
              <div v-if="getStatus(scope.row)">
                <el-tag :type="getStatusType(scope.row)">
                  {{ getStatus(scope.row) }}
                </el-tag>
              </div>
              <span v-else>-</span>
            </template>
          </el-table-column>
          
          <!-- 操作列 -->
          <el-table-column label="操作" width="100" align="center">
            <template #default="scope">
              <el-popover
                placement="left"
                trigger="click"
                :width="600"
                popper-class="yaml-popover"
              >
                <template #reference>
                  <el-button size="small" type="primary" plain>详情</el-button>
                </template>
                <div class="yaml-content">
                  <pre>{{ formatJson(scope.row) }}</pre>
                </div>
              </el-popover>
            </template>
          </el-table-column>
        </el-table>
        
        <!-- 分页组件 -->
        <div class="pagination-container">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[50, 100, 200, 500]"
            :total="filteredObjects.length"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handleCurrentChange"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, onMounted, ref, watch } from 'vue'
import { useStore } from 'vuex'
import { useRoute } from 'vue-router'
import { Search } from '@element-plus/icons-vue'

export default {
  name: 'ResourceDetail',
  components: {
    Search
  },
  setup() {
    const store = useStore()
    const route = useRoute()
    const currentNamespace = ref('all')
    const availableNamespaces = ref([])
    const currentPage = ref(1)
    const pageSize = ref(100)
    const searchQuery = ref('')

    // 从Store获取数据
    const selectedResource = computed(() => store.state.selectedResource)
    const resourceObjects = computed(() => store.state.resourceObjects)
    const loading = computed(() => store.state.loading)

    // 计算资源标题
    const resourceTitle = computed(() => {
      if (!selectedResource.value) return ''
      return `${selectedResource.value.kind} (${selectedResource.value.group}/${selectedResource.value.version})`
    })

    // 路由参数变化时加载资源
    const loadResourceFromRoute = () => {
      const { group, version, resource } = route.params
      if (group && version && resource) {
        // 查找资源
        const resourceItem = store.state.resources.find(r => 
          r.group === group && r.version === version && r.name === resource
        )
        
        if (resourceItem) {
          store.dispatch('selectResource', resourceItem)
          fetchData()
        }
      }
    }

    // 获取数据
    const fetchData = async () => {
      if (selectedResource.value) {
        // 获取资源对象
        await store.dispatch('fetchResourceObjects')
        
        // 如果是命名空间资源，获取可用的命名空间
        if (selectedResource.value.namespaced) {
          await store.dispatch('fetchResourceNamespaces')
          availableNamespaces.value = store.state.resourceNamespaces
        }
      }
    }

    // 处理命名空间变化
    const handleNamespaceChange = (namespace) => {
      store.dispatch('setNamespace', namespace)
      // 重置分页到第一页
      currentPage.value = 1
    }

    // 获取对象状态
    const getStatus = (row) => {
      if (!row.status) return null
      
      // 尝试从常见的状态字段获取状态信息
      const statusFields = ['phase', 'state', 'status', 'conditions']
      
      for (const field of statusFields) {
        if (row.status[field]) {
          if (Array.isArray(row.status[field]) && row.status[field].length > 0) {
            // 如果是条件数组，返回最新条件的状态
            const latestCondition = row.status[field][row.status[field].length - 1]
            return latestCondition.status || latestCondition.type
          }
          return row.status[field]
        }
      }
      
      return null
    }

    // 根据状态获取标签类型
    const getStatusType = (row) => {
      const status = getStatus(row)
      if (!status) return ''
      
      const statusLower = String(status).toLowerCase()
      
      if (statusLower.includes('running') || statusLower.includes('ready') || statusLower.includes('success') || statusLower.includes('true')) {
        return 'success'
      } else if (statusLower.includes('pending') || statusLower.includes('waiting')) {
        return 'warning'
      } else if (statusLower.includes('error') || statusLower.includes('failed') || statusLower.includes('false')) {
        return 'danger'
      }
      
      return 'info'
    }

    // 格式化JSON以便显示
    const formatJson = (obj) => {
      return JSON.stringify(obj, null, 2)
    }

    // 监听路由变化
    watch(() => route.params, loadResourceFromRoute, { immediate: true })

    // 监听选中资源变化，重新获取数据
    watch(selectedResource, fetchData)

    // 监听资源对象变化，重置分页和搜索
    watch(resourceObjects, () => {
      currentPage.value = 1
      searchQuery.value = ''
    })

    // 监听选中资源变化，重置搜索
    watch(selectedResource, () => {
      searchQuery.value = ''
      currentPage.value = 1
    })

    // 组件挂载时，如果有参数则加载资源
    onMounted(() => {
      if (route.params.resource) {
        loadResourceFromRoute()
      }
    })

    // 分页相关逻辑
    const totalObjects = computed(() => resourceObjects.value.length)
    
    // 搜索过滤逻辑
    const filteredObjects = computed(() => {
      if (!searchQuery.value) {
        return resourceObjects.value
      }
      
      const query = searchQuery.value.toLowerCase()
      return resourceObjects.value.filter(obj => {
        // 搜索名称
        if (obj.name && obj.name.toLowerCase().includes(query)) {
          return true
        }
        
        // 搜索命名空间
        if (obj.namespace && obj.namespace.toLowerCase().includes(query)) {
          return true
        }
        
        // 搜索Kind
        if (obj.kind && obj.kind.toLowerCase().includes(query)) {
          return true
        }
        
        return false
      })
    })
    
    const paginatedObjects = computed(() => {
      const start = (currentPage.value - 1) * pageSize.value
      const end = start + pageSize.value
      return filteredObjects.value.slice(start, end)
    })

    const handleSizeChange = (newSize) => {
      pageSize.value = newSize
    }

    const handleCurrentChange = (newPage) => {
      currentPage.value = newPage
    }

    const handleSearch = () => {
      // 搜索时重置到第一页
      currentPage.value = 1
    }

    return {
      selectedResource,
      resourceObjects,
      loading,
      resourceTitle,
      currentNamespace,
      availableNamespaces,
      handleNamespaceChange,
      getStatus,
      getStatusType,
      formatJson,
      totalObjects,
      filteredObjects,
      paginatedObjects,
      currentPage,
      pageSize,
      handleSizeChange,
      handleCurrentChange,
      searchQuery,
      handleSearch
    }
  }
}
</script>

<style>
.resource-detail {
  padding: 20px;
}

.resource-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  padding: 15px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 8px;
  color: white;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.resource-header h2 {
  margin: 0;
  font-weight: 500;
  font-size: 24px;
}

.namespace-selector {
  margin-left: 20px;
}

.namespace-selector .el-select {
  --el-select-input-color: white;
  --el-select-border-color-hover: rgba(255, 255, 255, 0.6);
}

.namespace-selector .el-input__wrapper {
  background-color: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.3) inset;
}

.namespace-selector .el-input__wrapper:hover {
  border-color: rgba(255, 255, 255, 0.6);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.6) inset;
}

.namespace-selector .el-input__inner {
  color: white;
}

.namespace-selector .el-input__inner::placeholder {
  color: rgba(255, 255, 255, 0.7);
}

.stats-bar {
  margin-bottom: 20px;
  display: flex;
  gap: 10px;
  align-items: center;
}

.stats-and-search {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 15px;
}

.search-container {
  display: flex;
  align-items: center;
}

.no-resource, .no-objects {
  text-align: center;
  padding: 50px 0;
  color: #909399;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
  padding: 20px 0;
  border-top: 1px solid #ebeef5;
}

.yaml-popover .el-popover__title {
  font-weight: bold;
}

.yaml-content {
  max-height: 400px;
  overflow-y: auto;
}

.yaml-content pre {
  margin: 0;
  padding: 10px;
  font-family: 'Courier New', Courier, monospace;
  background-color: #f5f7fa;
  color: #333;
  border-radius: 4px;
  white-space: pre-wrap;
}
</style> 