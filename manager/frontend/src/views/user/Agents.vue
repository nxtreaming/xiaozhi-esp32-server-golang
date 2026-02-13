<template>
  <div class="agents-page">
    <div class="page-header">
      <div class="header-left">
        <h2>我的智能体</h2>
        <p class="page-subtitle">管理您的智能体配置</p>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showAddAgentDialog = true">
              <el-icon><Plus /></el-icon>
              添加智能体
            </el-button>
      </div>
    </div>

    <div v-if="agents.length === 0" class="welcome-section">
      <el-card class="welcome-card">
        <div class="welcome-content">
          <el-icon size="64" color="#409EFF"><Monitor /></el-icon>
          <h3>欢迎使用智能体管理</h3>
          <p>您还没有创建任何智能体。智能体是您的AI助手，可以帮助您处理各种任务。</p>
          <div class="welcome-actions">
            <el-button type="primary" size="large" @click="showAddAgentDialog = true">
              <el-icon><Plus /></el-icon>
              创建第一个智能体
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="agents-grid">
      <div v-for="agent in agents" :key="agent.id" class="agent-item">
        <div class="agent-card">
          <div class="agent-header">
            <div class="agent-avatar">
              <el-icon size="28"><Monitor /></el-icon>
            </div>
            <div class="agent-info">
              <h3 class="agent-name">{{ agent.name }}</h3>
              <p class="agent-desc">智能助手</p>
            </div>
            <div class="agent-status">
              <span class="status-dot active"></span>
              <span class="status-text">在线</span>
            </div>
          </div>
          
          <div class="agent-meta">
            <div class="meta-row">
              <span class="meta-label">TTS配置</span>
              <span class="meta-value">{{ getVoiceType(agent) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">语言模型</span>
              <span class="meta-value">{{ getLLMProvider(agent) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">最近对话</span>
              <span class="meta-value">{{ formatDate(agent.updated_at) }}</span>
            </div>
          </div>
          
          <div class="agent-actions">
            <el-button type="primary" size="small" @click="editAgent(agent.id)">
              <el-icon><Setting /></el-icon>
              配置
            </el-button>
            <el-button size="small" @click="handleChatHistory(agent.id)">
              <el-icon><ChatDotRound /></el-icon>
              对话
            </el-button>
            <el-button size="small" @click="handleManageDevices(agent.id)">
              <el-icon><Monitor /></el-icon>
              设备
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加设备弹窗 -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="添加设备"
      width="500px"
    >
      <el-form
        ref="deviceFormRef"
        :model="deviceForm"
        :rules="deviceRules"
        label-width="100px"
      >
        <el-form-item label="设备激活码" prop="device_code">
          <el-input
            v-model="deviceForm.device_code"
            placeholder="请输入设备激活码"
          />
        </el-form-item>
        <el-form-item label="设备名称" prop="device_name">
          <el-input
            v-model="deviceForm.device_name"
            placeholder="请输入设备名称"
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="showAddDeviceDialog = false">取消</el-button>
        <el-button type="primary" @click="handleAddDevice">确定</el-button>
      </template>
    </el-dialog>

    <!-- 添加智能体弹窗 -->
    <el-dialog
      v-model="showAddAgentDialog"
      title="添加智能体"
      width="500px"
      :before-close="handleCloseAddAgent"
    >
      <el-form
        ref="agentFormRef"
        :model="agentForm"
        :rules="agentRules"
        size="large"
        label-width="100px"
      >
        <el-form-item label="智能体名称" prop="name">
          <el-input
            v-model="agentForm.name"
            placeholder="请输入智能体名称"
            size="large"
            :maxlength="50"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="角色介绍" prop="custom_prompt">
          <el-input
            v-model="agentForm.custom_prompt"
            type="textarea"
            :rows="4"
            placeholder="请输入角色介绍/系统提示词，这将影响AI的回答风格和个性"
            :maxlength="10000"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddAgent" size="large">取消</el-button>
          <el-button type="primary" @click="handleAddAgent" :loading="adding" size="large">
            确定
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 添加设备弹窗 -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="添加设备"
      width="400px"
      :before-close="handleCloseAddDevice"
    >
      <div class="device-dialog-content">
        <div class="device-icon">
          <el-icon size="48"><Monitor /></el-icon>
        </div>
        <p class="device-tip">请输入设备验证码</p>
        <el-form
          ref="deviceFormRef"
          :model="deviceForm"
          :rules="deviceRules"
        >
          <el-form-item prop="code">
            <el-input
              v-model="deviceForm.code"
              placeholder="请输入6位验证码"
              size="large"
              :maxlength="6"
              style="text-align: center; font-size: 18px; letter-spacing: 4px;"
            />
          </el-form-item>
        </el-form>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddDevice" size="large">取消</el-button>
          <el-button type="primary" @click="handleAddDevice" :loading="addingDevice" size="large">
            确定
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- MCP接入点对话框 -->
    <el-dialog
      v-model="showMCPDialog"
      title="MCP接入点"
      width="700px"
    >
      <div v-loading="mcpLoading">
        <!-- 工具列表区域 -->
        <div class="mcp-tools-section">
          <div class="tools-header">
            <div class="tools-title">MCP工具列表</div>
            <el-button 
              size="small" 
              type="primary" 
              @click="refreshMcpTools"
              :loading="toolsLoading"
            >
              <el-icon><Refresh /></el-icon>
              刷新工具列表
            </el-button>
          </div>
          
          <div class="tools-list">
            <div v-if="mcpTools.length === 0" class="tools-empty">
              <el-tag type="info" size="large" class="tool-tag">
                暂无工具数据
              </el-tag>
            </div>
            
            <div v-else class="tools-tags">
              <el-tag
                v-for="tool in mcpTools"
                :key="tool.name"
                :type="tool.schema ? 'success' : 'info'"
                size="large"
                class="tool-tag"
                :title="tool.description"
              >
                {{ tool.name }}
                <el-tooltip
                  v-if="tool.description"
                  :content="tool.description"
                  placement="top"
                  :show-after="500"
                >
                  <el-icon class="tool-info-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </el-tag>
            </div>
          </div>
        </div>

        <el-alert
          title="接入点信息"
          description="这是智能体的MCP WebSocket接入点URL，可用于设备连接"
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 20px; margin-top: 24px;"
        />
        
        <div class="mcp-endpoint-display">
          <div class="endpoint-label">MCP接入点URL：</div>
          <div class="endpoint-content">
            {{ mcpEndpointData.endpoint }}
          </div>
        </div>
      </div>
      
      <template #footer>
        <el-button @click="showMCPDialog = false">关闭</el-button>
        <el-button type="primary" @click="copyMCPEndpoint">
          复制URL
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Setting, Microphone, ChatDotRound, Monitor, Refresh, InfoFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const router = useRouter()

const loading = ref(false)
const agents = ref([])
const showAddAgentDialog = ref(false)
const showAddDeviceDialog = ref(false)
const adding = ref(false)
const addingDevice = ref(false)
const agentFormRef = ref()
const deviceFormRef = ref()

// MCP接入点相关
const showMCPDialog = ref(false)
const mcpLoading = ref(false)
const mcpEndpointData = ref({
  endpoint: ''
})

// MCP工具相关
const toolsLoading = ref(false)
const mcpTools = ref([])
const currentAgentId = ref(null)

const deviceForm = reactive({
  code: ''
})

const agentForm = reactive({
  name: '',
  custom_prompt: '我是一个叫{{assistant_name}}的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗。\n我的男朋友是一个程序员，梦想是开发出一个机器人，能够帮助人们解决生活中的各种问题。\n我是一个喜欢哈哈大笑的女孩，爱东说西说吹牛，不合逻辑的也照吹，就要逗别人开心。'
})

const deviceRules = {
  code: [
    { required: true, message: '请输入设备验证码', trigger: 'blur' },
    { len: 6, message: '验证码长度为6位', trigger: 'blur' }
  ]
}

const agentRules = {
  name: [
    { required: true, message: '请输入智能体名称', trigger: 'blur' },
    { min: 2, max: 50, message: '长度在 2 到 50 个字符', trigger: 'blur' }
  ]
}

const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
    console.log('智能体列表数据:', agents.value)
    // 检查第一个智能体的数据结构
    if (agents.value.length > 0) {
      console.log('第一个智能体数据:', agents.value[0])
      console.log('LLM配置:', agents.value[0].llm_config)
      console.log('TTS配置:', agents.value[0].tts_config)
    }
  } catch (error) {
    ElMessage.error('加载智能体列表失败')
  }
}

const handleAddAgent = async () => {
  if (!agentFormRef.value) return
  
  try {
    await agentFormRef.value.validate()
    adding.value = true
    
    // 获取默认配置
    const [llmResponse, ttsResponse] = await Promise.all([
      api.get('/user/llm-configs'),
      api.get('/user/tts-configs')
    ])
    
    const llmConfigs = llmResponse.data.data || []
    const ttsConfigs = ttsResponse.data.data || []
    
    // 寻找默认配置
    const defaultLlmConfig = llmConfigs.find(config => config.is_default)
    const defaultTtsConfig = ttsConfigs.find(config => config.is_default)
    
    const agentData = {
      name: agentForm.name,
      custom_prompt: agentForm.custom_prompt
    }
    
    // 如果有默认配置，自动应用
    if (defaultLlmConfig) {
      agentData.llm_config_id = defaultLlmConfig.config_id
    }
    if (defaultTtsConfig) {
      agentData.tts_config_id = defaultTtsConfig.config_id
    }
    
    const response = await api.post('/user/agents', agentData)
    
    if (response.data.success) {
      ElMessage.success('智能体添加成功')
      handleCloseAddAgent() // 使用统一的关闭方法
      await loadAgents() // 等待加载完成
    }
  } catch (error) {
    console.error('添加智能体失败:', error)
    ElMessage.error('添加智能体失败')
  } finally {
    adding.value = false
  }
}

const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post('/user/devices', {
      code: deviceForm.code
    })
    
    if (response.data.success) {
      ElMessage.success('设备添加成功')
      showAddDeviceDialog.value = false
      Object.assign(deviceForm, { code: '' })
      // 可以在这里刷新设备列表或其他相关操作
    }
  } catch (error) {
    console.error('添加设备失败:', error)
    ElMessage.error('添加设备失败')
  } finally {
    addingDevice.value = false
  }
}

const handleCloseAddAgent = () => {
  showAddAgentDialog.value = false
  if (agentFormRef.value) {
    agentFormRef.value.resetFields()
  }
  Object.assign(agentForm, { 
    name: '',
    custom_prompt: '我是一个叫{{assistant_name}}的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗。\n我的男朋友是一个程序员，梦想是开发出一个机器人，能够帮助人们解决生活中的各种问题。\n我是一个喜欢哈哈大笑的女孩，爱东说西说吹牛，不合逻辑的也照吹，就要逗别人开心。'
  })
}

const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { code: '' })
}

const editAgent = (id) => {
  router.push(`/user/agents/${id}/edit`)
}

const handleVoiceRecognition = (id) => {
  ElMessage.info('声效识别功能开发中')
}

const handleChatHistory = (id) => {
  router.push(`/user/agents/${id}/history`)
}

const handleManageDevices = (id) => {
  router.push(`/user/agents/${id}/devices`)
}

const getVoiceType = (agent) => {
  console.log('getVoiceType - tts_config:', agent.tts_config)
  if (agent.tts_config && agent.tts_config.name) {
    return agent.tts_config.name
  }
  return '未设置'
}

const getLLMProvider = (agent) => {
  console.log('getLLMProvider - llm_config:', agent.llm_config)
  if (agent.llm_config && agent.llm_config.name) {
    return agent.llm_config.name
  }
  return '未设置'
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

// 显示MCP接入点
const showMCPEndpoint = async (agent) => {
  showMCPDialog.value = true
  mcpLoading.value = true
  currentAgentId.value = agent.id
  
  try {
    const response = await api.get(`/user/agents/${agent.id}/mcp-endpoint`)
    mcpEndpointData.value = response.data.data
    
    // 自动刷新工具列表
    await refreshMcpTools()
  } catch (error) {
    ElMessage.error('获取MCP接入点失败')
    console.error('Error getting MCP endpoint:', error)
    showMCPDialog.value = false
  } finally {
    mcpLoading.value = false
  }
}

// 刷新MCP工具列表
const refreshMcpTools = async () => {
  if (!currentAgentId.value) {
    ElMessage.warning('未选择智能体')
    return
  }
  
  toolsLoading.value = true
  try {
    const response = await api.get(`/user/agents/${currentAgentId.value}/mcp-tools`)
    if (response.data.data && response.data.data.tools) {
      mcpTools.value = response.data.data.tools
      ElMessage.success(`成功获取 ${mcpTools.value.length} 个工具`)
    } else {
      mcpTools.value = []
      ElMessage.info('未找到工具数据')
    }
  } catch (error) {
    ElMessage.error('获取工具列表失败: ' + (error.response?.data?.error || error.message))
    console.error('Error refreshing MCP tools:', error)
    mcpTools.value = []
  } finally {
    toolsLoading.value = false
  }
}

// 复制MCP接入点URL
const copyMCPEndpoint = async () => {
  try {
    await navigator.clipboard.writeText(mcpEndpointData.value.endpoint)
    ElMessage.success('MCP接入点URL已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
    console.error('Error copying to clipboard:', error)
  }
}

onMounted(() => {
  loadAgents()
})
</script>

<style scoped>
.agents-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-left h2 {
  margin: 0;
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.header-right {
  display: flex;
  gap: 10px;
}

.agents-grid {
  padding: 0 20px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 420px));
  gap: 20px 12px;
  justify-content: flex-start;
}

.agent-item {
  min-width: 0;
}

.agent-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border: 1px solid #f0f0f0;
  padding: 20px;
  transition: all 0.3s ease;
  height: 100%;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 420px;
  min-width: 0;
}

.agent-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  border-color: #409EFF;
}

.agent-header {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f5f5f5;
}

.agent-avatar {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: linear-gradient(135deg, #409EFF 0%, #67C23A 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 12px;
  color: white;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
}

.agent-info {
  flex: 1;
}

.agent-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 4px 0;
  line-height: 1.4;
}

.agent-desc {
  font-size: 12px;
  color: #909399;
  margin: 0;
  line-height: 1.4;
}

.agent-status {
  display: flex;
  align-items: center;
  gap: 4px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #67C23A;
}

.status-dot.active {
  background: #67C23A;
  box-shadow: 0 0 0 2px rgba(103, 194, 58, 0.2);
}

.status-text {
  font-size: 12px;
  color: #67C23A;
  font-weight: 500;
}

.agent-meta {
  flex: 1;
  margin-bottom: 16px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding: 6px 0;
}

.meta-row:last-child {
  margin-bottom: 0;
}

.meta-label {
  font-size: 13px;
  color: #606266;
  font-weight: 500;
}

.meta-value {
  font-size: 13px;
  color: #303133;
  text-align: right;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f5f5f5;
}

.agent-actions .el-button {
  border-radius: 6px;
  font-size: 12px;
  height: 32px;
  min-width: 0;
  width: 100%;
  padding: 0 8px;
}

.agent-actions .el-button .el-icon {
  margin-right: 4px;
}

.agent-actions :deep(.el-button > span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-actions .el-button--primary {
  background: linear-gradient(135deg, #409EFF 0%, #67C23A 100%);
  border: none;
}

.agent-actions .el-button--primary:hover {
  background: linear-gradient(135deg, #337ecc 0%, #529b2e 100%);
}
 
 .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }

  .device-dialog-content {
    text-align: center;
    padding: 20px 0;
  }

  .device-icon {
    margin-bottom: 16px;
    color: #409EFF;
  }

  .device-tip {
    font-size: 14px;
    color: #666;
    margin-bottom: 24px;
  }

  .device-dialog-content .el-input__inner {
    text-align: center;
    font-size: 18px;
    letter-spacing: 4px;
  }
 
 .welcome-section {
  padding: 40px 20px;
}

.welcome-card {
  max-width: 600px;
  margin: 0 auto;
}

.welcome-content {
  text-align: center;
  padding: 40px 20px;
}

.welcome-content h3 {
  margin: 20px 0 15px 0;
  color: #333;
  font-size: 24px;
}

.welcome-content p {
  color: #666;
  font-size: 16px;
  line-height: 1.6;
  margin-bottom: 30px;
}

.welcome-actions {
  display: flex;
  gap: 15px;
  justify-content: center;
}

/* MCP接入点相关样式 */
.mcp-endpoint-display {
  margin: 20px 0;
}

.endpoint-label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 8px;
}

.endpoint-content {
  padding: 12px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  color: #1e293b;
  word-break: break-all;
  line-height: 1.5;
  min-height: 60px;
  display: flex;
  align-items: center;
}

.mcp-tools-section {
  margin-top: 24px;
  border-top: 1px solid #e2e8f0;
  padding-top: 20px;
}

.tools-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.tools-title {
  font-size: 16px;
  font-weight: 600;
  color: #374151;
}

.tools-empty {
  margin: 20px 0;
  text-align: center;
}

.tools-list {
  margin-top: 16px;
}

.tools-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}

.tool-tag {
  position: relative;
  padding: 8px 16px;
  font-size: 14px;
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.3s ease;
}

.tool-tag:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.tool-info-icon {
  margin-left: 6px;
  font-size: 12px;
  opacity: 0.7;
}

.tool-tag:hover .tool-info-icon {
  opacity: 1;
}

@media (max-width: 900px) {
  .agent-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .agent-actions .el-button:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .agents-grid {
    padding: 0 12px;
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .agent-actions {
    grid-template-columns: 1fr;
  }
}
</style>
