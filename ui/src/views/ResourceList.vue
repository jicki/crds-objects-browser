<template>
  <div class="resource-list">
    <h2>Kubernetes CRD 对象浏览器</h2>
    
    <div class="welcome-message">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>欢迎使用</span>
          </div>
        </template>
        <div class="card-content">
          <p>这是一个用于浏览Kubernetes集群中自定义资源(CRD)的工具。</p>
          <p>请从左侧菜单中选择一个CRD资源以查看其实例。</p>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script>
export default {
  name: 'ResourceList'
}
</script>

<style scoped>
.resource-list {
  padding: 20px;
}

h2 {
  margin-bottom: 30px;
  font-weight: 500;
  color: #303133;
}

.welcome-message {
  max-width: 800px;
  margin: 0 auto;
}

.card-header {
  font-size: 18px;
  font-weight: 500;
}

.card-content {
  padding: 20px 0;
  line-height: 1.6;
}
</style> 