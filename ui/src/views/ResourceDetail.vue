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
            style="width: 250px;"
            filterable
            clearable
            :filter-method="filterNamespaces"
            :no-data-text="namespaceSearchQuery ? '未找到匹配的命名空间' : '暂无命名空间'"
            @visible-change="handleNamespaceDropdownVisible"
          >
            <el-option key="all" label="🌐 所有命名空间" value="all" />
            <el-option
              v-for="ns in filteredNamespaces"
              :key="ns"
              :label="`📁 ${ns}`"
              :value="ns"
            >
              <div class="namespace-option">
                <span class="namespace-icon">📁</span>
                <span class="namespace-name">{{ ns }}</span>
                <el-tag v-if="getNamespaceObjectCount(ns) > 0" size="small" type="info" class="namespace-count">
                  {{ getNamespaceObjectCount(ns) }}
                </el-tag>
              </div>
            </el-option>
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
            style="width: 300px; margin-right: 15px;"
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          
          <!-- 状态过滤器 -->
          <el-select
            v-model="statusFilter"
            placeholder="📊 状态过滤"
            clearable
            size="large"
            style="width: 200px;"
            @change="handleStatusFilter"
          >
            <el-option
              v-for="status in availableStatuses"
              :key="status.value"
              :label="status.label"
              :value="status.value"
            >
              <div style="display: flex; align-items: center;">
                <el-tag :type="status.type" size="small" effect="dark" style="margin-right: 8px;">
                  <el-icon style="margin-right: 4px;">
                    <component :is="status.icon" />
                  </el-icon>
                  {{ status.label }}
                </el-tag>
                <span style="color: #909399; font-size: 12px;">({{ status.count }})</span>
              </div>
            </el-option>
          </el-select>
        </div>
      </div>
      
      <!-- 加载中提示 -->
      <el-skeleton v-if="loading" :rows="10" animated />
      
      <!-- 错误提示 -->
      <el-alert
        v-else-if="error"
        :title="error"
        type="error"
        :closable="false"
        show-icon
        style="margin-bottom: 15px;"
      />
      
      <!-- 资源对象表格 -->
      <div v-else-if="paginatedObjects.length === 0" class="no-objects">
        <el-empty description="没有找到资源对象">
          <template #image>
            <div style="font-size: 60px; color: #909399;">📦</div>
          </template>
          <template #description>
            <p>没有{{ selectedResource.name }}资源对象</p>
            <p style="color: #909399; font-size: 14px;">
              可能原因：资源不存在、权限不足或网络问题
            </p>
          </template>
          <el-button type="primary" @click="refreshData">
            <el-icon><Refresh /></el-icon>
            重新加载
          </el-button>
        </el-empty>
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
          
          <!-- Pod资源的Request/Limits列 -->
          <el-table-column 
            v-if="selectedResource && selectedResource.kind === 'Pod'" 
            label="Request/Limits" 
            width="280" 
            align="left"
          >
            <template #default="scope">
              <div v-if="getPodResourceInfo(scope.row)" class="resource-info">
                <div v-for="(container, index) in getPodResourceInfo(scope.row)" :key="index" class="container-resources">
                  <div class="container-name">{{ container.name }}</div>
                  <div class="resource-details">
                    <div v-if="container.requests" class="resource-row">
                      <span class="resource-label">Request:</span>
                      <div class="resource-values">
                        <el-tag v-if="container.requests.cpu" size="small" type="info" class="resource-tag">
                          CPU: {{ container.requests.cpu }}
                        </el-tag>
                        <el-tag v-if="container.requests.memory" size="small" type="info" class="resource-tag">
                          内存: {{ container.requests.memory }}
                        </el-tag>
                      </div>
                    </div>
                    <div v-if="container.limits" class="resource-row">
                      <span class="resource-label">Limits:</span>
                      <div class="resource-values">
                        <el-tag v-if="container.limits.cpu" size="small" type="warning" class="resource-tag">
                          CPU: {{ container.limits.cpu }}
                        </el-tag>
                        <el-tag v-if="container.limits.memory" size="small" type="warning" class="resource-tag">
                          内存: {{ container.limits.memory }}
                        </el-tag>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <span v-else class="no-resource-info">-</span>
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
import { computed, onMounted, onUnmounted, ref, watch, nextTick } from 'vue'
import { useStore } from 'vuex'
import { useRoute } from 'vue-router'
import { Search, Document, Clock, CopyDocument, View as ViewIcon, SuccessFilled, WarningFilled, CircleCloseFilled, InfoFilled, QuestionFilled, Refresh } from '@element-plus/icons-vue'

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
    QuestionFilled,
    Refresh
  },
  setup() {
    const store = useStore()
    const route = useRoute()
    const currentNamespace = ref('all')
    const availableNamespaces = ref([])
    const currentPage = ref(1)
    const pageSize = ref(100)
    const searchQuery = ref('')
    const statusFilter = ref('')
    const namespaceSearchQuery = ref('')
    const namespaceDropdownVisible = ref(false)

    // 从Store获取数据
    const selectedResource = computed(() => store.state.selectedResource)
    const resourceObjects = computed(() => store.state.resourceObjects)
    const loading = computed(() => store.state.loading)
    const error = computed(() => store.state.error)

    // 计算资源标题
    const resourceTitle = computed(() => {
      if (!selectedResource.value) return ''
      return `${selectedResource.value.kind} (${selectedResource.value.group}/${selectedResource.value.version})`
    })

    // 路由参数变化时加载资源
    const loadResourceFromRoute = () => {
      const { group, version, resource } = route.params
      console.log('=== loadResourceFromRoute 开始 ===')
      console.log('路由参数:', { group, version, resource })
      console.log('当前路由:', route)
      console.log('路由名称:', route.name)
      
      if (version && resource) {
        // 根据路由名称确定group值
        let actualGroup
        if (route.name === 'CoreResourceDetail') {
          // Kubernetes Core资源路由，group为空字符串
          actualGroup = ''
          console.log('检测到Core资源路由，设置group为空字符串')
        } else {
          // 普通资源路由，使用路由参数中的group
          actualGroup = group
          console.log('检测到普通资源路由，group:', actualGroup)
        }
        
        console.log('loadResourceFromRoute 被调用，参数:', { group: actualGroup, version, resource })
        console.log('当前 store.state.resources:', store.state.resources)
        console.log('当前 store.state.resources 长度:', store.state.resources ? store.state.resources.length : 'null/undefined')
        console.log('当前 selectedResource:', store.state.selectedResource)
        
        // 查找资源的函数
        const findAndSelectResource = () => {
          console.log('开始查找资源...')
          
          const resourceItem = store.state.resources.find(r => 
            r.group === actualGroup && r.version === version && r.name === resource
          )
          
          console.log('查找条件:', { group: actualGroup, version, resource })
          console.log('查找到的资源:', resourceItem)
          
          if (resourceItem) {
            console.log('找到资源，选择资源:', resourceItem)
            store.dispatch('selectResource', resourceItem)
            fetchData()
            return true
          } else {
            console.log('未找到匹配的资源')
            console.log('可用资源列表:')
            if (store.state.resources) {
              store.state.resources.forEach((r, index) => {
                console.log(`  ${index}: ${r.group}/${r.version}/${r.name}`)
              })
            }
          }
          return false
        }
        
        // 如果资源数据已经加载，直接查找
        if (store.state.resources && store.state.resources.length > 0) {
          console.log('资源数据已加载，直接查找')
          if (findAndSelectResource()) {
            console.log('=== loadResourceFromRoute 成功结束 ===')
            return
          }
        }
        
        // 如果资源数据还没有加载，等待加载完成
        console.log('资源数据未加载，等待加载完成...')
        
        // 监听资源数据变化
        const unwatch = watch(() => store.state.resources, (resources) => {
          console.log('监听到资源数据变化:', resources ? resources.length : 'null/undefined')
          if (resources && resources.length > 0) {
            console.log('资源数据已加载，尝试查找资源')
            if (findAndSelectResource()) {
              console.log('找到资源，停止监听')
              unwatch() // 停止监听
            }
          }
        }, { immediate: true })
        
        // 如果资源数据还没有开始加载，主动触发加载
        if (!store.state.resources || store.state.resources.length === 0) {
          console.log('主动触发资源数据加载')
          store.dispatch('fetchResources')
        }
        
        // 设置超时，避免无限等待
        setTimeout(() => {
          unwatch()
          console.log('loadResourceFromRoute 超时，停止等待')
        }, 10000) // 10秒超时
      } else {
        console.log('路由参数不完整，跳过加载')
      }
      
      console.log('=== loadResourceFromRoute 结束 ===')
    }

    // 获取数据
    const fetchData = async () => {
      if (selectedResource.value) {
        // 保存当前滚动位置
        saveScrollPosition()
        
        // 获取资源对象
        await store.dispatch('fetchResourceObjects')
        
        // 如果是命名空间资源，获取可用的命名空间
        if (selectedResource.value.namespaced) {
          await store.dispatch('fetchResourceNamespaces')
          availableNamespaces.value = store.state.resourceNamespaces
        }
        
        // 恢复滚动位置
        nextTick(() => {
          restoreScrollPosition()
        })
      }
    }

    // 保存滚动位置
    const saveScrollPosition = () => {
      const mainContainer = document.querySelector('.el-main')
      if (mainContainer && selectedResource.value) {
        const scrollKey = `scroll_${selectedResource.value.group}_${selectedResource.value.version}_${selectedResource.value.name}`
        localStorage.setItem(scrollKey, mainContainer.scrollTop.toString())
      }
    }

    // 恢复滚动位置
    const restoreScrollPosition = () => {
      if (selectedResource.value) {
        const scrollKey = `scroll_${selectedResource.value.group}_${selectedResource.value.version}_${selectedResource.value.name}`
        const savedScrollTop = localStorage.getItem(scrollKey)
        
        if (savedScrollTop) {
          const mainContainer = document.querySelector('.el-main')
          if (mainContainer) {
            setTimeout(() => {
              mainContainer.scrollTop = parseInt(savedScrollTop, 10)
            }, 100)
          }
        }
      }
    }

    // 处理命名空间变化
    const handleNamespaceChange = (namespace) => {
      // 保存当前滚动位置
      saveScrollPosition()
      
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
      
      // 检查特殊的状态字段
      if (row.status.replicas !== undefined && row.status.readyReplicas !== undefined) {
        if (row.status.readyReplicas === row.status.replicas && row.status.replicas > 0) {
          return 'Ready'
        } else if (row.status.readyReplicas === 0) {
          return 'NotReady'
        } else {
          return 'Partial'
        }
      }
      
      // 检查Pod特有状态
      if (row.kind === 'Pod') {
        if (row.status.containerStatuses) {
          const allReady = row.status.containerStatuses.every(c => c.ready)
          return allReady ? 'Running' : 'NotReady'
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

    // 获取Pod资源信息
    const getPodResourceInfo = (pod) => {
      if (!pod.spec || !pod.spec.containers) {
        return null
      }
      
      return pod.spec.containers.map(container => {
        const containerInfo = {
          name: container.name,
          requests: null,
          limits: null
        }
        
        if (container.resources) {
          if (container.resources.requests) {
            containerInfo.requests = {
              cpu: container.resources.requests.cpu || null,
              memory: container.resources.requests.memory || null
            }
          }
          
          if (container.resources.limits) {
            containerInfo.limits = {
              cpu: container.resources.limits.cpu || null,
              memory: container.resources.limits.memory || null
            }
          }
        }
        
        return containerInfo
      }).filter(container => container.requests || container.limits) // 只返回有资源配置的容器
    }

    // 监听路由变化
    watch(() => route.params, loadResourceFromRoute, { immediate: true })

    // 监听选中资源变化，重新获取数据
    watch(selectedResource, fetchData)

    // 监听资源对象变化，重置分页和搜索
    watch(resourceObjects, () => {
      currentPage.value = 1
      searchQuery.value = ''
      statusFilter.value = ''
    })

    // 监听选中资源变化，重置搜索
    watch(selectedResource, () => {
      searchQuery.value = ''
      statusFilter.value = ''
      currentPage.value = 1
    })

    // 组件挂载时，如果有参数则加载资源
    onMounted(() => {
      console.log('=== ResourceDetail 组件挂载 ===')
      console.log('当前路由参数:', route.params)
      console.log('当前 store 状态:')
      console.log('  - resources:', store.state.resources ? store.state.resources.length : 'null/undefined')
      console.log('  - selectedResource:', store.state.selectedResource)
      console.log('  - loading:', store.state.loading)
      
      if (route.params.resource) {
        console.log('检测到路由参数，调用 loadResourceFromRoute')
        loadResourceFromRoute()
      } else {
        console.log('没有路由参数，跳过加载')
      }
    })

    // 分页相关逻辑
    const totalObjects = computed(() => resourceObjects.value.length)
    
    // 搜索过滤逻辑
    const filteredObjects = computed(() => {
      let filtered = resourceObjects.value
      
      // 搜索过滤
      if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase()
        filtered = filtered.filter(obj => {
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
      }
      
      // 状态过滤
      if (statusFilter.value) {
        filtered = filtered.filter(obj => {
          const status = getStatus(obj)
          return status && normalizeStatus(status) === statusFilter.value
        })
      }
      
      return filtered
    })

    // 标准化状态值
    const normalizeStatus = (status) => {
      if (!status) return 'unknown'
      
      const statusLower = String(status).toLowerCase()
      
      // 成功/正常状态
      if (statusLower.includes('running') || 
          statusLower.includes('ready') || 
          statusLower.includes('success') || 
          statusLower.includes('true') ||
          statusLower.includes('active') ||
          statusLower.includes('bound') ||
          statusLower.includes('available')) {
        return 'success'
      } 
      
      // 警告状态
      if (statusLower.includes('pending') || 
          statusLower.includes('waiting') ||
          statusLower.includes('partial') ||
          statusLower.includes('progressing') ||
          statusLower.includes('creating') ||
          statusLower.includes('updating')) {
        return 'warning'
      } 
      
      // 错误状态
      if (statusLower.includes('error') || 
          statusLower.includes('failed') || 
          statusLower.includes('false') ||
          statusLower.includes('notready') ||
          statusLower.includes('crashloopbackoff') ||
          statusLower.includes('imagepullbackoff') ||
          statusLower.includes('evicted') ||
          statusLower.includes('terminated')) {
        return 'danger'
      }
      
      // 停止状态
      if (statusLower.includes('stopped') ||
          statusLower.includes('completed') ||
          statusLower.includes('succeeded') ||
          statusLower.includes('finished')) {
        return 'info'
      }
      
      return 'info'
    }

    // 计算可用状态列表
    const availableStatuses = computed(() => {
      const statusMap = new Map()
      
      resourceObjects.value.forEach(obj => {
        const status = getStatus(obj)
        if (status) {
          const normalizedStatus = normalizeStatus(status)
          const statusKey = normalizedStatus
          
          if (!statusMap.has(statusKey)) {
            statusMap.set(statusKey, {
              value: statusKey,
              label: getStatusLabel(normalizedStatus),
              type: normalizedStatus,
              icon: getStatusIconName(normalizedStatus),
              count: 0
            })
          }
          
          statusMap.get(statusKey).count++
        }
      })
      
      return Array.from(statusMap.values()).sort((a, b) => b.count - a.count)
    })

    // 获取状态标签
    const getStatusLabel = (normalizedStatus) => {
      const labels = {
        'success': '正常',
        'warning': '处理中',
        'danger': '异常',
        'info': '完成'
      }
      return labels[normalizedStatus] || '未知'
    }

    // 获取状态图标名称
    const getStatusIconName = (normalizedStatus) => {
      const icons = {
        'success': 'SuccessFilled',
        'warning': 'WarningFilled',
        'danger': 'CircleCloseFilled',
        'info': 'InfoFilled'
      }
      return icons[normalizedStatus] || 'QuestionFilled'
    }

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

    const handleStatusFilter = () => {
      // 状态过滤时重置到第一页
      currentPage.value = 1
    }

    // 组件卸载时保存滚动位置
    onUnmounted(() => {
      saveScrollPosition()
    })

    // 过滤命名空间
    const filterNamespaces = (query) => {
      namespaceSearchQuery.value = query
    }

    // 计算过滤后的命名空间列表
    const filteredNamespaces = computed(() => {
      if (!namespaceSearchQuery.value) {
        return availableNamespaces.value
      }
      
      const query = namespaceSearchQuery.value.toLowerCase()
      return availableNamespaces.value.filter(ns => 
        ns.toLowerCase().includes(query)
      )
    })

    // 获取命名空间中的对象数量
    const getNamespaceObjectCount = (namespace) => {
      if (!resourceObjects.value || resourceObjects.value.length === 0) {
        return 0
      }
      
      return resourceObjects.value.filter(obj => obj.namespace === namespace).length
    }

    // 处理命名空间下拉框显示/隐藏
    const handleNamespaceDropdownVisible = (visible) => {
      namespaceDropdownVisible.value = visible
      if (!visible) {
        // 下拉框关闭时清空搜索查询
        namespaceSearchQuery.value = ''
      }
    }

    const refreshData = () => {
      // 重置分页和搜索
      currentPage.value = 1
      searchQuery.value = ''
      statusFilter.value = ''
      // 重新获取数据
      fetchData()
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
      handleSearch,
      statusFilter,
      handleStatusFilter,
      availableStatuses,
      getPodResourceInfo,
      namespaceSearchQuery,
      namespaceDropdownVisible,
      filterNamespaces,
      filteredNamespaces,
      getNamespaceObjectCount,
      handleNamespaceDropdownVisible,
      error,
      refreshData
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

/* Pod资源信息样式 */
.resource-info {
  padding: 4px 0;
}

.container-resources {
  margin-bottom: 8px;
  padding: 6px;
  background-color: #f8f9fa;
  border-radius: 4px;
  border-left: 3px solid #409eff;
}

.container-resources:last-child {
  margin-bottom: 0;
}

.container-name {
  font-weight: 600;
  color: #303133;
  font-size: 12px;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
}

.container-name::before {
  content: "📦";
  margin-right: 4px;
}

.resource-details {
  font-size: 11px;
}

.resource-row {
  margin-bottom: 3px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.resource-row:last-child {
  margin-bottom: 0;
}

.resource-label {
  font-weight: 500;
  color: #606266;
  min-width: 60px;
  font-size: 11px;
}

.resource-values {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.resource-tag {
  font-size: 10px !important;
  padding: 2px 6px !important;
  height: auto !important;
  line-height: 1.2 !important;
}

.no-resource-info {
  color: #c0c4cc;
  font-style: italic;
}

/* 命名空间选项样式 */
.namespace-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 2px 0;
}

.namespace-icon {
  margin-right: 8px;
  font-size: 14px;
}

.namespace-name {
  flex: 1;
  font-size: 14px;
  color: #303133;
}

.namespace-count {
  margin-left: 8px;
  font-size: 11px !important;
  padding: 1px 6px !important;
  height: auto !important;
  line-height: 1.2 !important;
  border-radius: 10px;
}

/* 命名空间下拉框样式优化 */
:deep(.el-select-dropdown__item) {
  padding: 8px 12px;
  line-height: 1.4;
}

:deep(.el-select-dropdown__item:hover) {
  background-color: #f5f7fa;
}

:deep(.el-select-dropdown__item.selected) {
  background-color: #ecf5ff;
  color: #409eff;
  font-weight: 500;
}

/* 命名空间选择器输入框样式 */
.namespace-selector :deep(.el-input__wrapper) {
  transition: all 0.3s ease;
}

.namespace-selector :deep(.el-input__wrapper:focus-within) {
  border-color: rgba(255, 255, 255, 0.8);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.8) inset, 0 0 8px rgba(255, 255, 255, 0.3);
}
</style> 