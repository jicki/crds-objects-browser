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
          <el-select v-model="currentNamespace" placeholder="选择命名空间" @change="handleNamespaceChange">
            <el-option key="all" label="所有命名空间" value="all" />
            <el-option
              v-for="ns in availableNamespaces"
              :key="ns"
              :label="ns"
              :value="ns"
            />
          </el-select>
        </div>
      </div>
      
      <!-- 加载中提示 -->
      <el-skeleton v-if="loading" :rows="10" animated />
      
      <!-- 资源对象表格 -->
      <div v-else-if="resourceObjects.length === 0" class="no-objects">
        <p>没有{{ selectedResource.name }}资源对象</p>
      </div>
      <div v-else class="resource-table">
        <el-table :data="resourceObjects" style="width: 100%" border stripe>
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
      </div>
    </div>
  </div>
</template>

<script>
import { computed, onMounted, ref, watch } from 'vue'
import { useStore } from 'vuex'
import { useRoute } from 'vue-router'

export default {
  name: 'ResourceDetail',
  setup() {
    const store = useStore()
    const route = useRoute()
    const currentNamespace = ref('all')
    const availableNamespaces = ref([])

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

    // 组件挂载时，如果有参数则加载资源
    onMounted(() => {
      if (route.params.resource) {
        loadResourceFromRoute()
      }
    })

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
      formatJson
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
}

.resource-header h2 {
  margin: 0;
  font-weight: 500;
}

.no-resource, .no-objects {
  text-align: center;
  padding: 50px 0;
  color: #909399;
}

.namespace-selector {
  margin-left: 20px;
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