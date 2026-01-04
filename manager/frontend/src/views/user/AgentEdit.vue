<template>
  <div class="agent-config">
    <div class="config-header">
      <div class="header-left">
        <el-button 
          @click="$router.back()" 
          :icon="ArrowLeft" 
          circle 
          size="large"
        />
        <h1>智能体配置</h1>
      </div>
      <el-button type="primary" @click="handleSave" :loading="saving" size="large">
        保存配置
      </el-button>
    </div>

    <div class="config-content">
      <div class="config-form">
        <!-- 基础信息 -->
        <div class="form-section">
          <h3 class="section-title">基础信息</h3>
          
          <div class="form-group">
            <label class="form-label">昵称</label>
            <el-input 
              v-model="form.name" 
              placeholder="请输入智能体昵称" 
              size="large"
              :maxlength="50"
              show-word-limit
            />
          </div>

          <div class="form-group">
            <label class="form-label">角色介绍(prompt)</label>
            <el-input
              v-model="form.custom_prompt"
              type="textarea"
              :rows="4"
              placeholder="请输入角色介绍/系统提示词，这将影响AI的回答风格和个性"
              :maxlength="1000"
              show-word-limit
            />
          </div>
        </div>

        <!-- 配置设置 -->
        <div class="form-section">
          <h3 class="section-title">配置设置</h3>
          
          <div class="form-group">
            <label class="form-label">语言模型</label>
            <el-select 
              v-model="form.llm_config_id" 
              placeholder="请选择语言模型" 
              size="large" 
              style="width: 100%"
              clearable
            >
              <el-option
                v-for="llmConfig in llmConfigs"
                :key="llmConfig.config_id"
                :label="llmConfig.is_default ? `${llmConfig.name} (默认)` : llmConfig.name"
                :value="llmConfig.config_id"
              >
                <div class="config-option">
                  <span class="config-name">
                    {{ llmConfig.name }}
                    <el-tag v-if="llmConfig.is_default" type="success" size="small" style="margin-left: 8px;">默认</el-tag>
                  </span>
                  <span class="config-desc">{{ llmConfig.provider || '暂无描述' }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="form-help" v-if="getCurrentLlmConfigName()">
              {{ getCurrentLlmConfigInfo() }}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">TTS配置</label>
            <el-select 
              v-model="form.tts_config_id" 
              placeholder="请选择TTS配置" 
              size="large" 
              style="width: 100%"
              clearable
              @change="handleTtsConfigChange"
            >
              <el-option
                v-for="ttsConfig in ttsConfigs"
                :key="ttsConfig.config_id"
                :label="ttsConfig.is_default ? `${ttsConfig.name} (默认)` : ttsConfig.name"
                :value="ttsConfig.config_id"
              >
                <div class="config-option">
                  <span class="config-name">
                    {{ ttsConfig.name }}
                    <el-tag v-if="ttsConfig.is_default" type="success" size="small" style="margin-left: 8px;">默认</el-tag>
                  </span>
                  <span class="config-desc">{{ ttsConfig.provider || '暂无描述' }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="form-help" v-if="getCurrentTtsConfigName()">
              {{ getCurrentTtsConfigInfo() }}
            </div>
          </div>

          <div class="form-group" v-if="form.tts_config_id">
            <label class="form-label">音色</label>
            <el-select 
              v-model="form.voice" 
              placeholder="请选择或输入音色（支持搜索和自定义输入）" 
              size="large" 
              style="width: 100%"
              filterable
              allow-create
              default-first-option
              reserve-keyword
              clearable
              :loading="voiceLoading"
              :filter-method="filterVoice"
            >
              <el-option
                v-for="voice in filteredVoices"
                :key="voice.value"
                :label="voice.label"
                :value="voice.value"
              >
                <span>{{ voice.label }}</span>
                <span style="color: #8492a6; font-size: 13px; margin-left: 8px;">{{ voice.value }}</span>
              </el-option>
            </el-select>
            <div class="form-help">
              当前TTS配置: {{ getCurrentTtsConfigName() }}，可以搜索音色名称或值，也可以手动输入自定义音色值。
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">语音识别速度</label>
            <el-select v-model="form.asr_speed" placeholder="请选择语音识别速度" size="large" style="width: 100%">
              <el-option label="正常" value="normal" />
              <el-option label="耐心" value="patient" />
              <el-option label="快速" value="fast" />
            </el-select>
            <div class="form-help">设置语音识别的响应速度</div>
          </div>

          <div class="form-group">
            <label class="form-label">MCP接入点</label>
            <el-button 
              type="primary" 
              @click="showMCPEndpoint" 
              size="large"
              style="width: 100%"
            >
              查看MCP接入点
            </el-button>
            <div class="form-help">获取智能体的MCP WebSocket接入点URL，可用于设备连接</div>
          </div>
        </div>
      </div>
    </div>

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
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, VideoPlay, Refresh, InfoFilled } from '@element-plus/icons-vue'
import api from '@/utils/api'

const route = useRoute()
const router = useRouter()
const saving = ref(false)

// 表单数据
const form = reactive({
  name: '',
  custom_prompt: '',
  llm_config_id: null,
  tts_config_id: null,
  voice: null,
  asr_speed: 'normal'
})

// 角色模板数据
const roleTemplates = ref([])

// LLM配置数据
const llmConfigs = ref([])

// TTS配置数据
const ttsConfigs = ref([])

// 音色相关数据
const availableVoices = ref([])
const filteredVoices = ref([])
const voiceSearchKeyword = ref('')
const voiceLoading = ref(false)
const previousTtsConfigId = ref(null) // 用于跟踪TTS配置变化

// MCP接入点相关
const showMCPDialog = ref(false)
const mcpLoading = ref(false)
const mcpEndpointData = ref({
  endpoint: ''
})
const toolsLoading = ref(false)
const mcpTools = ref([])

// 加载LLM配置
const loadLlmConfigs = async () => {
  try {
    const response = await api.get('/user/llm-configs')
    llmConfigs.value = response.data.data || []
    // 不在这里自动选择默认配置，交给具体的使用场景处理
  } catch (error) {
    console.error('加载LLM配置失败:', error)
  }
}

// 加载TTS配置
const loadTtsConfigs = async () => {
  try {
    const response = await api.get('/user/tts-configs')
    ttsConfigs.value = response.data.data || []
    // 不在这里自动选择默认配置，交给具体的使用场景处理
  } catch (error) {
    console.error('加载TTS配置失败:', error)
  }
}



// 加载智能体数据
const loadAgent = async () => {
  try {
    const response = await api.get(`/user/agents/${route.params.id}`)
    const agent = response.data.data
    
    // 映射基本字段
    Object.assign(form, {
      name: agent.name || '',
      custom_prompt: agent.custom_prompt || '',
      asr_speed: agent.asr_speed || 'normal',
      voice: agent.voice || null
    })
    
    // 处理LLM配置关联
    const hasValidLlmConfigId = agent.llm_config_id && 
                               agent.llm_config_id !== '' && 
                               agent.llm_config_id !== 'null' && 
                               agent.llm_config_id !== 'undefined'
    
    if (hasValidLlmConfigId) {
      // 验证config_id是否在可用配置中
      const llmConfig = llmConfigs.value.find(config => config.config_id === agent.llm_config_id)
      if (llmConfig) {
        form.llm_config_id = agent.llm_config_id
        console.log(`✅ 智能体使用LLM配置: ${llmConfig.name}`)
      } else {
        console.warn(`⚠️ 智能体的LLM配置ID ${agent.llm_config_id} 不存在，将使用默认配置`)
        // 如果config_id无效，使用默认配置
        const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
        form.llm_config_id = defaultLlmConfig ? defaultLlmConfig.config_id : null
        if (defaultLlmConfig) {
          console.log(`🔄 已切换到默认LLM配置: ${defaultLlmConfig.name}`)
        }
      }
    } else {
      // 如果没有配置，使用默认配置
      const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
      form.llm_config_id = defaultLlmConfig ? defaultLlmConfig.config_id : null
      if (defaultLlmConfig) {
        console.log(`🎯 智能体LLM配置为空，使用默认配置: ${defaultLlmConfig.name}`)
      } else {
        console.warn(`❌ 没有找到默认LLM配置`)
      }
    }
    
    // 处理TTS配置关联
    const hasValidTtsConfigId = agent.tts_config_id && 
                               agent.tts_config_id !== '' && 
                               agent.tts_config_id !== 'null' && 
                               agent.tts_config_id !== 'undefined'
    
    if (hasValidTtsConfigId) {
      // 验证config_id是否在可用配置中
      const ttsConfig = ttsConfigs.value.find(config => config.config_id === agent.tts_config_id)
      if (ttsConfig) {
        form.tts_config_id = agent.tts_config_id
        console.log(`✅ 智能体使用TTS配置: ${ttsConfig.name}`)
      } else {
        console.warn(`⚠️ 智能体的TTS配置ID ${agent.tts_config_id} 不存在，将使用默认配置`)
        // 如果config_id无效，使用默认配置
        const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
        form.tts_config_id = defaultTtsConfig ? defaultTtsConfig.config_id : null
        if (defaultTtsConfig) {
          console.log(`🔄 已切换到默认TTS配置: ${defaultTtsConfig.name}`)
        }
      }
    } else {
      // 如果没有配置，使用默认配置
      const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
      form.tts_config_id = defaultTtsConfig ? defaultTtsConfig.config_id : null
      if (defaultTtsConfig) {
        console.log(`🎯 智能体TTS配置为空，使用默认配置: ${defaultTtsConfig.name}`)
      } else {
        console.warn(`❌ 没有找到默认TTS配置`)
      }
    }
  } catch (error) {
    console.error('加载智能体失败:', error)
    ElMessage.error('加载智能体失败')
  }
}

// 加载角色模板
const loadRoleTemplates = async () => {
  try {
    const response = await api.get('/user/role-templates')
    roleTemplates.value = response.data.data || []
  } catch (error) {
    console.error('加载角色模板失败:', error)
  }
}

// 保存智能体
const handleSave = async () => {
  if (!form.name.trim()) {
    ElMessage.error('请输入智能体昵称')
    return
  }
  
  try {
    saving.value = true
    
    const response = await api.put(`/user/agents/${route.params.id}`, form)
    
    ElMessage.success('保存成功')
    router.push('/user/agents')
  } catch (error) {
    console.error('保存失败:', error)
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}



// 获取当前LLM配置名称
const getCurrentLlmConfigName = () => {
  if (!form.llm_config_id) return null
  const config = llmConfigs.value.find(c => c.config_id === form.llm_config_id)
  return config ? config.name : null
}

// 获取当前LLM配置信息
const getCurrentLlmConfigInfo = () => {
  if (!form.llm_config_id) return ''
  const config = llmConfigs.value.find(c => c.config_id === form.llm_config_id)
  if (!config) return ''
  
  if (config.is_default) {
    return `当前使用默认LLM配置: ${config.name}`
  } else {
    return `当前使用LLM配置: ${config.name}`
  }
}

// 获取当前TTS配置名称
const getCurrentTtsConfigName = () => {
  if (!form.tts_config_id) return null
  const config = ttsConfigs.value.find(c => c.config_id === form.tts_config_id)
  return config ? config.name : null
}

// 获取当前TTS配置信息
const getCurrentTtsConfigInfo = () => {
  if (!form.tts_config_id) return ''
  const config = ttsConfigs.value.find(c => c.config_id === form.tts_config_id)
  if (!config) return ''
  
  if (config.is_default) {
    return `当前使用默认TTS配置: ${config.name}`
  } else {
    return `当前使用TTS配置: ${config.name}`
  }
}

// 自动选择默认配置
const autoSelectDefaultConfigs = () => {
  // 选择默认LLM配置
  if (!form.llm_config_id && llmConfigs.value.length > 0) {
    const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
    if (defaultLlmConfig) {
      form.llm_config_id = defaultLlmConfig.config_id
    }
  }
  
  // 选择默认TTS配置
  if (!form.tts_config_id && ttsConfigs.value.length > 0) {
    const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
    if (defaultTtsConfig) {
      form.tts_config_id = defaultTtsConfig.config_id
    }
  }
}

// 显示MCP接入点
const showMCPEndpoint = async () => {
  showMCPDialog.value = true
  mcpLoading.value = true
  
  try {
    const response = await api.get(`/user/agents/${route.params.id}/mcp-endpoint`)
    mcpEndpointData.value = response.data.data
    
    // 获取工具列表
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
  toolsLoading.value = true
  try {
    const response = await api.get(`/user/agents/${route.params.id}/mcp-tools`)
    mcpTools.value = response.data.data.tools || []
  } catch (error) {
    console.error('获取MCP工具列表失败:', error)
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

// 处理TTS配置变化，加载对应的音色列表
const handleTtsConfigChange = async () => {
  // 获取之前的provider（如果有）
  let previousProvider = null
  if (previousTtsConfigId.value) {
    const prevConfig = ttsConfigs.value.find(config => config.config_id === previousTtsConfigId.value)
    previousProvider = prevConfig?.provider
  }
  
  if (!form.tts_config_id) {
    availableVoices.value = []
    filteredVoices.value = []
    form.voice = null // 清空音色
    previousTtsConfigId.value = null
    return
  }
  
  // 获取当前TTS配置的provider
  const ttsConfig = ttsConfigs.value.find(config => config.config_id === form.tts_config_id)
  if (!ttsConfig || !ttsConfig.provider) {
    availableVoices.value = []
    filteredVoices.value = []
    form.voice = null // 清空音色
    previousTtsConfigId.value = form.tts_config_id
    return
  }
  
  // 如果provider发生变化，清空当前的voice值
  if (previousProvider && previousProvider !== ttsConfig.provider) {
    form.voice = null
  }
  
  // 加载音色列表
  await loadVoices(ttsConfig.provider)
  
  // 如果当前voice值在新列表中不存在，也清空它
  if (form.voice && availableVoices.value.length > 0) {
    const voiceExists = availableVoices.value.some(v => v.value === form.voice)
    if (!voiceExists) {
      form.voice = null
    }
  }
  
  // 更新previousTtsConfigId
  previousTtsConfigId.value = form.tts_config_id
}

// 音色搜索过滤函数
const filterVoice = (val) => {
  voiceSearchKeyword.value = val
  if (!val) {
    filteredVoices.value = availableVoices.value
    return
  }
  
  const keyword = val.toLowerCase()
  filteredVoices.value = availableVoices.value.filter(voice => {
    // 同时搜索 label 和 value
    return voice.label.toLowerCase().includes(keyword) || 
           voice.value.toLowerCase().includes(keyword)
  })
}

// 加载音色列表
const loadVoices = async (provider) => {
  if (!provider) {
    availableVoices.value = []
    filteredVoices.value = []
    return
  }
  
  voiceLoading.value = true
  try {
    const response = await api.get(`/user/voice-options?provider=${provider}`)
    availableVoices.value = response.data.data || []
    filteredVoices.value = availableVoices.value
  } catch (error) {
    console.error('加载音色列表失败:', error)
    availableVoices.value = []
    filteredVoices.value = []
  } finally {
    voiceLoading.value = false
  }
}

onMounted(async () => {
  // 先加载配置数据
  await Promise.all([
    loadLlmConfigs(),
    loadTtsConfigs()
  ])
  
  if (route.params.id) {
    // 编辑现有智能体，加载智能体数据
    await loadAgent()
    // 如果已有TTS配置，加载对应的音色列表
    if (form.tts_config_id) {
      previousTtsConfigId.value = form.tts_config_id
      const ttsConfig = ttsConfigs.value.find(config => config.config_id === form.tts_config_id)
      if (ttsConfig && ttsConfig.provider) {
        await loadVoices(ttsConfig.provider)
      }
    }
  } else {
    // 新建智能体，自动选择默认配置
    autoSelectDefaultConfigs()
    // 如果自动选择了TTS配置，记录它
    if (form.tts_config_id) {
      previousTtsConfigId.value = form.tts_config_id
    }
  }
})
</script>

<style scoped>
.agent-config {
  min-height: 100vh;
  background: #f8fafc;
  padding: 24px;
}

.config-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
  background: white;
  padding: 20px 24px;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #1f2937;
}

.config-content {
  max-width: 800px;
  margin: 0 auto;
}

.config-form {
  background: white;
  border-radius: 12px;
  padding: 32px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.form-section {
  margin-bottom: 40px;
  padding-bottom: 32px;
  border-bottom: 1px solid #e5e7eb;
}

.form-section:last-child {
  margin-bottom: 0;
  border-bottom: none;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
  margin: 0 0 24px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid #3b82f6;
  display: inline-block;
}

.form-group {
  margin-bottom: 24px;
}

.form-group:last-child {
  margin-bottom: 0;
}

.form-label {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 8px;
}

.form-help {
  font-size: 12px;
  color: #6b7280;
  margin-top: 4px;
}

.switch-group {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.switch-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f9fafb;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
}

.switch-item span {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
}

.template-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 16px;
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #fafafa;
}

.template-card:hover {
  border-color: #3b82f6;
  background: #f0f9ff;
}

.template-card.active {
  border-color: #3b82f6;
  background: #eff6ff;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.template-icon {
  font-size: 32px;
  margin-bottom: 8px;
}

.template-name {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  text-align: center;
}



.memory-settings {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.memory-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  background: #f9fafb;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
}

.memory-item span {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.config-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-name {
  font-weight: 500;
  color: #374151;
}

.config-desc {
  font-size: 12px;
  color: #6b7280;
}

/* MCP工具列表相关样式 */
.mcp-tools-section {
  margin-bottom: 24px;
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
  color: #1f2937;
}

.tools-list {
  min-height: 60px;
}

.tools-empty {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
}

.tools-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tool-tag {
  position: relative;
  padding: 8px 12px;
  font-size: 13px;
  border-radius: 6px;
  cursor: default;
}

.tool-info-icon {
  margin-left: 6px;
  font-size: 12px;
  color: #6b7280;
  cursor: help;
}

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

@media (max-width: 768px) {
  .agent-config {
    padding: 16px;
  }
  
  .config-header {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }
  
  .header-left {
    justify-content: center;
  }
  
  .config-form {
    padding: 24px 16px;
  }
  
  .template-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .memory-item {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
}
</style>