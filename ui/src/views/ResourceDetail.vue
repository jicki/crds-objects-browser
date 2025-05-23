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
        <el-table 
          :data="paginatedObjects" 
          style="width: 100%" 
          border 
          stripe
          :header-cell-style="{ background: '#f5f7fa', color: '#606266', fontWeight: 'bold' }"
          :row-style="{ height: '50px' }"
          size="default"
        >
          <el-table-column prop="name" label="名称" min-width="200" sortable show-overflow-tooltip>
            <template #default="scope">
              <div class="name-cell">
                <el-icon class="resource-icon"><Document /></el-icon>
                <span class="resource-name">{{ scope.row.name }}</span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column 
            prop="namespace" 
            label="命名空间" 
            width="180" 
            v-if="selectedResource.namespaced" 
            sortable 
            show-overflow-tooltip
          >
            <template #default="scope">
              <el-tag v-if="scope.row.namespace" type="info" size="small" effect="plain">
                📁 {{ scope.row.namespace }}
              </el-tag>
              <span v-else class="no-namespace">-</span>
            </template>
          </el-table-column>
          
          <el-table-column prop="creationTimestamp" label="创建时间" width="200" sortable>
            <template #default="scope">
              <div class="time-cell">
                <el-icon class="time-icon"><Clock /></el-icon>
                <span>{{ formatTime(scope.row.creationTimestamp) }}</span>
              </div>
            </template>
          </el-table-column>
          
          <!-- 动态状态列 -->
          <el-table-column label="状态" width="120" align="center">
            <template #default="scope">
              <div v-if="getStatus(scope.row)" class="status-cell">
                <el-tag :type="getStatusType(scope.row)" size="small" effect="dark">
                  <el-icon class="status-icon">
                    <component :is="getStatusIcon(scope.row)" />
                  </el-icon>
                  {{ getStatus(scope.row) }}
                </el-tag>
              </div>
              <span v-else class="no-status">-</span>
            </template>
          </el-table-column>
          
          <!-- 操作列 -->
          <el-table-column label="操作" width="100" align="center" fixed="right">
            <template #default="scope">
              <el-popover
                placement="left"
                trigger="click"
                :width="700"
                popper-class="yaml-popover"
              >
                <template #reference>
                  <el-button size="small" type="primary" plain>
                    <el-icon><ViewIcon /></el-icon>
                    详情
                  </el-button>
                </template>
                <div class="yaml-content">
                  <div class="yaml-header">
                    <h4>{{ scope.row.name }} 详细信息</h4>
                    <el-button size="small" @click="copyToClipboard(scope.row)">
                      <el-icon><CopyDocument /></el-icon>
                      复制
                    </el-button>
                  </div>
                  <pre class="yaml-text">{{ formatJson(scope.row) }}</pre>
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
import { Search, Document, Clock, CopyDocument, View as ViewIcon, SuccessFilled, WarningFilled, CircleCloseFilled, InfoFilled, QuestionFilled } from '@element-plus/icons-vue'

export default {
  name: 'ResourceDetail',
  components: {
    Search,
    Document,
    Clock,
    CopyDocument,
    ViewIcon,
    SuccessFilled,
    WarningFilled,
    CircleCloseFilled,
    InfoFilled,
    QuestionFilled
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

    // 格式化时间显示
    const formatTime = (timestamp) => {
      if (!timestamp) return '-'
      const date = new Date(timestamp)
      const now = new Date()
      const diff = now - date
      
      // 计算时间差
      const minutes = Math.floor(diff / (1000 * 60))
      const hours = Math.floor(diff / (1000 * 60 * 60))
      const days = Math.floor(diff / (1000 * 60 * 60 * 24))
      
      if (days > 0) {
        return `${days}天前`
      } else if (hours > 0) {
        return `${hours}小时前`
      } else if (minutes > 0) {
        return `${minutes}分钟前`
      } else {
        return '刚刚'
      }
    }

    // 获取状态图标
    const getStatusIcon = (row) => {
      const status = getStatus(row)
      if (!status) return 'QuestionFilled'
      
      const statusLower = String(status).toLowerCase()
      
      if (statusLower.includes('running') || statusLower.includes('ready') || statusLower.includes('success') || statusLower.includes('true')) {
        return 'SuccessFilled'
      } else if (statusLower.includes('pending') || statusLower.includes('waiting')) {
        return 'WarningFilled'
      } else if (statusLower.includes('error') || statusLower.includes('failed') || statusLower.includes('false')) {
        return 'CircleCloseFilled'
      }
      
      return 'InfoFilled'
    }

    // 复制到剪贴板
    const copyToClipboard = async (obj) => {
      try {
        const text = JSON.stringify(obj, null, 2)
        await navigator.clipboard.writeText(text)
        // 这里可以添加成功提示
        console.log('复制成功')
      } catch (err) {
        console.error('复制失败:', err)
      }
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
      formatTime,
      getStatusIcon,
      copyToClipboard,
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

.name-cell {
  display: flex;
  align-items: center;
}

.resource-icon {
  color: #409eff;
  margin-right: 8px;
}

.resource-name {
  font-weight: 500;
  color: #303133;
}

.time-cell {
  display: flex;
  align-items: center;
}

.time-icon {
  color: #909399;
  margin-right: 6px;
}

.status-cell {
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-icon {
  margin-right: 4px;
}

.no-namespace, .no-status {
  color: #c0c4cc;
  font-style: italic;
}

.yaml-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
  padding-bottom: 10px;
  border-bottom: 1px solid #ebeef5;
}

.yaml-header h4 {
  margin: 0;
  color: #303133;
  font-size: 16px;
}

.yaml-text {
  margin: 0;
  padding: 15px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background-color: #f8f9fa;
  color: #2c3e50;
  border-radius: 6px;
  border: 1px solid #e9ecef;
  white-space: pre-wrap;
  word-wrap: break-word;
  max-height: 500px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.5;
}

/* 表格行悬停效果 */
:deep(.el-table__row:hover) {
  background-color: #f5f7fa !important;
}

/* 表格边框优化 */
:deep(.el-table) {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

/* 表格头部样式 */
:deep(.el-table__header-wrapper) {
  background: linear-gradient(135deg, #f5f7fa 0%, #e9ecef 100%);
}

/* 状态标签样式优化 */
:deep(.el-tag.el-tag--success.el-tag--dark) {
  background-color: #67c23a;
  border-color: #67c23a;
}

:deep(.el-tag.el-tag--warning.el-tag--dark) {
  background-color: #e6a23c;
  border-color: #e6a23c;
}

:deep(.el-tag.el-tag--danger.el-tag--dark) {
  background-color: #f56c6c;
  border-color: #f56c6c;
}

:deep(.el-tag.el-tag--info.el-tag--dark) {
  background-color: #909399;
  border-color: #909399;
}
</style> 