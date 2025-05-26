#!/bin/bash

echo "🔧 性能优化和弃用API修复验证脚本"
echo "=================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检查服务器是否运行
echo -e "\n${BLUE}1. 检查服务器状态${NC}"
if curl -s http://localhost:8080/healthz > /dev/null; then
    echo -e "${GREEN}✅ 服务器运行正常${NC}"
else
    echo -e "${RED}❌ 服务器未运行，请先启动服务器${NC}"
    exit 1
fi

# 测试API性能
echo -e "\n${BLUE}2. 测试API性能${NC}"
echo "测试资源列表API响应时间..."

# 第一次请求（可能需要初始化缓存）
echo "首次请求:"
time_output=$(time curl -s http://localhost:8080/api/crds | jq '. | length' 2>&1)
resource_count=$(echo "$time_output" | head -1)
first_time=$(echo "$time_output" | grep "total" | awk '{print $NF}')
echo -e "资源数量: ${GREEN}$resource_count${NC}"
echo -e "响应时间: ${GREEN}$first_time${NC}"

# 第二次请求（应该命中缓存）
echo -e "\n缓存命中请求:"
time_output=$(time curl -s http://localhost:8080/api/crds | jq '. | length' 2>&1)
second_time=$(echo "$time_output" | grep "total" | awk '{print $NF}')
echo -e "响应时间: ${GREEN}$second_time${NC}"

# 检查弃用API过滤
echo -e "\n${BLUE}3. 检查弃用API过滤${NC}"
deprecated_cronjobs=$(curl -s http://localhost:8080/api/crds | jq '.[] | select(.group == "batch" and .version == "v1beta1" and .name == "cronjobs")')

if [ -z "$deprecated_cronjobs" ]; then
    echo -e "${GREEN}✅ 弃用的 batch/v1beta1 cronjobs 已被过滤${NC}"
else
    echo -e "${YELLOW}⚠️  弃用的 batch/v1beta1 cronjobs 仍然存在${NC}"
    echo "返回的数据: $deprecated_cronjobs"
fi

# 检查batch组资源
echo -e "\n检查batch组资源:"
batch_resources=$(curl -s http://localhost:8080/api/crds | jq '.[] | select(.group == "batch") | {group, version, name}')
echo "$batch_resources"

# 测试资源对象API性能
echo -e "\n${BLUE}4. 测试资源对象API性能${NC}"
echo "测试Node对象API..."
time_output=$(time curl -s http://localhost:8080/api/crds/core/v1/nodes/objects | jq '. | length' 2>&1)
nodes_count=$(echo "$time_output" | head -1)
nodes_time=$(echo "$time_output" | grep "total" | awk '{print $NF}')
echo -e "Node数量: ${GREEN}$nodes_count${NC}"
echo -e "响应时间: ${GREEN}$nodes_time${NC}"

# 测试命名空间API性能
echo -e "\n测试Pod命名空间API..."
time_output=$(time curl -s http://localhost:8080/api/crds/core/v1/pods/namespaces | jq '. | length' 2>&1)
namespaces_count=$(echo "$time_output" | head -1)
namespaces_time=$(echo "$time_output" | grep "total" | awk '{print $NF}')
echo -e "命名空间数量: ${GREEN}$namespaces_count${NC}"
echo -e "响应时间: ${GREEN}$namespaces_time${NC}"

# 检查前端页面
echo -e "\n${BLUE}5. 检查前端页面${NC}"
if curl -s http://localhost:8080/ui/ | grep -q "Kubernetes CRD 对象浏览器"; then
    echo -e "${GREEN}✅ 前端页面加载正常${NC}"
else
    echo -e "${RED}❌ 前端页面加载失败${NC}"
fi

# 检查缓存状态
echo -e "\n${BLUE}6. 检查缓存状态${NC}"
cache_stats=$(curl -s http://localhost:8080/api/cache/status)
echo "缓存状态:"
echo "$cache_stats" | jq '.'

# 性能总结
echo -e "\n${BLUE}7. 性能总结${NC}"
echo "=================================="
echo -e "资源列表API首次请求: ${GREEN}$first_time${NC}"
echo -e "资源列表API缓存命中: ${GREEN}$second_time${NC}"
echo -e "Node对象API响应时间: ${GREEN}$nodes_time${NC}"
echo -e "命名空间API响应时间: ${GREEN}$namespaces_time${NC}"

# 判断性能是否达标
echo -e "\n${BLUE}8. 性能评估${NC}"
if [[ "$first_time" < "1.000s" ]]; then
    echo -e "${GREEN}✅ 首次请求性能优秀 (<1秒)${NC}"
elif [[ "$first_time" < "3.000s" ]]; then
    echo -e "${YELLOW}⚠️  首次请求性能良好 (<3秒)${NC}"
else
    echo -e "${RED}❌ 首次请求性能需要改进 (>3秒)${NC}"
fi

if [[ "$second_time" < "0.500s" ]]; then
    echo -e "${GREEN}✅ 缓存命中性能优秀 (<0.5秒)${NC}"
else
    echo -e "${YELLOW}⚠️  缓存命中性能可以改进${NC}"
fi

# 建议
echo -e "\n${BLUE}9. 优化建议${NC}"
echo "=================================="
echo "1. 如果首次请求仍然较慢，考虑增加预加载资源"
echo "2. 如果缓存命中仍然较慢，考虑调整缓存策略"
echo "3. 监控日志中的慢请求警告，确认阈值调整效果"
echo "4. 定期检查弃用API过滤是否生效"

echo -e "\n${GREEN}🎉 性能验证完成！${NC}"
echo "详细报告请查看: PERFORMANCE_FIX_REPORT.md" 