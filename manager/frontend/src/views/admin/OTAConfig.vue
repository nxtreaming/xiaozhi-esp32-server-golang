<template>
  <div class="ota-config">
    <!-- 页面标题 -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon"><Setting /></el-icon>
          <h1 class="page-title">OTA配置管理</h1>
        </div>
      </div>
    </div>

    <!-- 配置说明 -->
    <div class="config-description">
      <el-alert
        title="配置说明"
        description="配置OTA升级相关参数，包括Test和External环境设置。WebSocket配置是指下发给终端连接的websocket地址，MQTT配置是指下发给终端mqtt连接(需要确保启用mqtt server和udp server),固件默认优先使用mqtt"
        type="info"
        :closable="false"
        show-icon
      />
    </div>

    <!-- 配置表单 -->
    <div class="form-container">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="140px"
        class="config-form"
        label-position="left"
      >
        <!-- 基础配置卡片 -->
        <el-card class="config-card basic-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Tools /></el-icon>
              <span class="card-title">基础配置</span>
            </div>
          </template>
          
          <el-form-item label="签名密钥" prop="signature_key" class="form-item full-width">
            <el-input 
              v-model="form.signature_key" 
              placeholder="请输入签名密钥"
              size="large"
              :prefix-icon="Key"
              show-password
            />
            <div class="form-item-hint">
              用来生成连接mqtt server的用户名和密码，必须与 mqtt server配置页面中的'签名密钥' 中的一致
            </div>
          </el-form-item>
        </el-card>
        
        <!-- Test环境配置卡片 -->
        <el-card class="config-card test-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon test-icon"><Monitor /></el-icon>
              <span class="card-title">Test环境配置</span>
              <el-tag type="warning" size="small">测试环境</el-tag>
            </div>
          </template>
          
          <!-- WebSocket配置 -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Connection /></el-icon>
              <span>WebSocket配置</span>
              <el-tooltip content="下发给终端连接的websocket地址" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="WebSocket URL" prop="test.websocket.url" class="form-item full-width">
                 <el-input 
                   v-model="form.test.websocket.url" 
                   placeholder="请输入Test环境WebSocket URL"
                   size="large"
                   :prefix-icon="Link"
                 />
               </el-form-item>
            </div>
          </div>
          
          <!-- MQTT配置 -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Message /></el-icon>
              <span>MQTT配置</span>
              <el-tooltip content="下发给终端mqtt连接(需要确保启用mqtt server和udp server),固件默认优先使用mqtt" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="MQTT启用状态" class="form-item">
                <el-switch 
                  v-model="form.test.mqtt.enable" 
                  size="large"
                  active-text="启用"
                  inactive-text="禁用"
                />
              </el-form-item>
               
              <el-form-item label="MQTT端点" prop="test.mqtt.endpoint" class="form-item" v-if="form.test.mqtt.enable">
                <el-input 
                  v-model="form.test.mqtt.endpoint" 
                  placeholder="请输入Test环境MQTT端点，格式：ip:port"
                  size="large"
                  :prefix-icon="Link"
                />
              </el-form-item>
            </div>
          </div>
        </el-card>
        
        <!-- External环境配置卡片 -->
        <el-card class="config-card external-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon external-icon"><Platform /></el-icon>
              <span class="card-title">External环境配置</span>
              <el-tag type="success" size="small">生产环境</el-tag>
            </div>
          </template>
          
          <!-- WebSocket配置 -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Connection /></el-icon>
              <span>WebSocket配置</span>
              <el-tooltip content="下发给终端连接的websocket地址" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="WebSocket URL" prop="external.websocket.url" class="form-item full-width">
                 <el-input 
                   v-model="form.external.websocket.url" 
                   placeholder="请输入External环境WebSocket URL"
                   size="large"
                   :prefix-icon="Link"
                 />
               </el-form-item>
            </div>
          </div>
          
          <!-- MQTT配置 -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Message /></el-icon>
              <span>MQTT配置</span>
              <el-tooltip content="下发给终端mqtt连接(需要确保启用mqtt server和udp server),固件默认优先使用mqtt" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="MQTT启用状态" class="form-item">
                <el-switch 
                  v-model="form.external.mqtt.enable" 
                  size="large"
                  active-text="启用"
                  inactive-text="禁用"
                />
              </el-form-item>
               
              <el-form-item label="MQTT端点" prop="external.mqtt.endpoint" class="form-item" v-if="form.external.mqtt.enable">
                <el-input 
                  v-model="form.external.mqtt.endpoint" 
                  placeholder="请输入External环境MQTT端点，格式：ip:port"
                  size="large"
                  :prefix-icon="Link"
                />
              </el-form-item>
            </div>
          </div>
        </el-card>
        
        <!-- 操作按钮 -->
        <div class="action-section">
          <el-button 
            type="primary" 
            @click="saveConfig" 
             :loading="saving"
            size="large"
            class="save-button"
          >
            <el-icon><Check /></el-icon>
            保存配置
          </el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { 
  Setting, Tools, Monitor, Platform, Connection, Message, 
  Edit, Key, Link, User, Lock, Check, QuestionFilled 
} from '@element-plus/icons-vue'
import api from '@/utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref()

const form = reactive({
  signature_key: 'xiaozhi_ota_signature_key',
  test: {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: true,
      endpoint: '127.0.0.1:1883'
    }
  },
  external: {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: false,
      endpoint: '127.0.0.1:1883'
    }
  }
})

const generateConfig = () => {
  return JSON.stringify({
    signature_key: form.signature_key,
    test: {
      websocket: {
        url: form.test.websocket.url
      },
      mqtt: {
        enable: form.test.mqtt.enable,
        endpoint: form.test.mqtt.endpoint
      }
    },
    external: {
      websocket: {
        url: form.external.websocket.url
      },
      mqtt: {
        enable: form.external.mqtt.enable,
        endpoint: form.external.mqtt.endpoint
      }
    }
  }, null, 2)
}

const rules = {
  signature_key: [
    { required: true, message: '请输入签名密钥', trigger: 'blur' }
  ],
  'test.websocket.url': [
    { required: true, message: '请输入Test环境WebSocket URL', trigger: 'blur' }
  ],
  'test.mqtt.endpoint': [
    {
      validator: (rule, value, callback) => {
        if (form.test.mqtt.enable && !value) {
          callback(new Error('启用MQTT时端点不能为空'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'external.websocket.url': [
    { required: true, message: '请输入External环境WebSocket URL', trigger: 'blur' }
  ],
  'external.mqtt.endpoint': [
    {
      validator: (rule, value, callback) => {
        if (form.external.mqtt.enable && !value) {
          callback(new Error('启用MQTT时端点不能为空'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadConfig = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/ota-configs')
    const configs = response.data.data || []
    
    if (configs.length > 0) {
      const config = configs[0]
      configId.value = config.id
      
      try {
        const configData = JSON.parse(config.json_data || '{}')
        form.signature_key = configData.signature_key || 'xiaozhi_ota_signature_key'
        
        // Test环境配置
        if (configData.test) {
          form.test.websocket.url = configData.test.websocket?.url || 'ws://127.0.0.1:8989/xiaozhi/v1/'
          form.test.mqtt.enable = configData.test.mqtt?.enable !== undefined ? configData.test.mqtt.enable : true
          form.test.mqtt.endpoint = configData.test.mqtt?.endpoint || '127.0.0.1:1883'
        }
        
        // External环境配置
        if (configData.external) {
          form.external.websocket.url = configData.external.websocket?.url || 'ws://127.0.0.1:8989/xiaozhi/v1/'
          form.external.mqtt.enable = configData.external.mqtt?.enable !== undefined ? configData.external.mqtt.enable : false
          form.external.mqtt.endpoint = configData.external.mqtt?.endpoint || '127.0.0.1:1883'
        }
      } catch (error) {
        console.error('解析配置失败:', error)
        ElMessage.error('配置格式错误')
      }
    }
  } catch (error) {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
    saving.value = true
    
    // 如果MQTT被禁用，清空端点值
    if (!form.test.mqtt.enable) {
      form.test.mqtt.endpoint = ''
    }
    if (!form.external.mqtt.enable) {
      form.external.mqtt.endpoint = ''
    }
    
    const configData = {
      name: 'OTA配置',
      config_id: 'ota_ota_config',
      provider: form.provider || 'default',
      json_data: generateConfig(),
      enabled: true,
      is_default: true
    }
    
    if (configId.value) {
      await api.put(`/admin/ota-configs/${configId.value}`, configData)
      ElMessage.success('配置更新成功')
    } else {
      const response = await api.post('/admin/ota-configs', configData)
      configId.value = response.data.data.id
      ElMessage.success('配置创建成功')
    }
  } catch (error) {
    if (error.message) {
      ElMessage.error('保存失败: ' + error.message)
    }
  } finally {
    saving.value = false
  }
}



// 监听provider变化，重置表单为默认值
watch(() => form.provider, (newProvider) => {
  if (newProvider) {
    // 重置表单为默认值
    form.signature_key = 'your_signature_key_here'
    form.test = {
      websocket: {
        url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
      },
      mqtt: {
        enable: false,
        endpoint: '127.0.0.1:1883'
      }
    }
    form.external = {
      websocket: {
        url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
      },
      mqtt: {
        enable: true,
        endpoint: '127.0.0.1:1883'
      }
    }
  }
})

// 监听MQTT开关状态变化，重置相关验证
watch(() => form.test.mqtt.enable, (enabled) => {
  if (!enabled) {
    // 当MQTT禁用时，清空端点并重置验证
    form.test.mqtt.endpoint = ''
    formRef.value?.clearValidate('test.mqtt.endpoint')
  }
})

watch(() => form.external.mqtt.enable, (enabled) => {
  if (!enabled) {
    // 当MQTT禁用时，清空端点并重置验证
    form.external.mqtt.endpoint = ''
    formRef.value?.clearValidate('external.mqtt.endpoint')
  }
})

const resetForm = () => {
  editingConfig.value = null
  form.provider = ''
  form.signature_key = 'your_signature_key_here'
  form.test = {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: false,
      endpoint: '127.0.0.1:1883'
    }
  }
  form.external = {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: true,
      endpoint: '127.0.0.1:1883'
    }
  }
  
  // 清除表单验证状态
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.ota-config {
  min-height: 100vh;
  background: #f8fafc;
  padding: 0;
}

/* 页面标题区域 */
.page-header {
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  padding: 2rem 0;
  margin-bottom: 2rem;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 2rem;
}

.title-section {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.5rem;
}

.title-icon {
  font-size: 2rem;
  color: #667eea;
}

.page-title {
  font-size: 2.5rem;
  font-weight: 700;
  color: #1f2937;
  margin: 0;
}

/* 配置说明 */
.config-description {
  max-width: 1200px;
  margin: 0 auto 2rem;
  padding: 0 2rem;
}

/* 表单容器 */
.form-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 2rem 2rem;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

/* 配置卡片 */
.config-card {
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  overflow: hidden;
}

.config-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.15);
}

.config-card.basic-config {
  border-left: 4px solid #3b82f6;
}

.config-card.test-config {
  border-left: 4px solid #f59e0b;
}

.config-card.external-config {
  border-left: 4px solid #10b981;
}

/* 卡片头部 */
.card-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-weight: 600;
  font-size: 1.1rem;
  color: #1f2937;
}

.card-icon {
  font-size: 1.25rem;
}

.card-icon.test-icon {
  color: #f59e0b;
}

.card-icon.external-icon {
  color: #10b981;
}

.card-title {
  flex: 1;
}

/* 表单网格布局 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1.5rem;
  margin-bottom: 1.5rem;
}

.form-item.full-width {
  grid-column: 1 / -1;
}

/* 配置区域 */
.config-section {
  margin-bottom: 2rem;
}

.config-section:last-child {
  margin-bottom: 0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 2px solid #e5e7eb;
}

.section-title .el-icon {
  color: #6366f1;
}

.help-icon {
  color: #9ca3af;
  cursor: help;
  font-size: 0.875rem;
}

.help-icon:hover {
  color: #6366f1;
}

/* 表单项样式 */
.form-item {
  margin-bottom: 0;
}

.form-item-hint {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
  line-height: 1.5;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
  line-height: 1.5;
}

:deep(.el-input) {
  border-radius: 8px;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.25);
}

:deep(.el-input-number) {
  width: 100%;
}

:deep(.el-input-number .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-switch) {
  --el-switch-on-color: #10b981;
  --el-switch-off-color: #d1d5db;
}

/* 操作按钮区域 */
.action-section {
  display: flex;
  justify-content: center;
  padding: 2rem 0;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.save-button {
  padding: 12px 32px;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 8px;
  background: #3b82f6;
  border: none;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.2);
  transition: all 0.3s ease;
}

.save-button:hover {
  background: #2563eb;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.save-button:active {
  transform: translateY(0);
}

/* 响应式设计 */
@media (max-width: 1024px) {
  .page-title {
    font-size: 2.2rem;
  }
}

@media (max-width: 768px) {
  .header-content {
    padding: 0 1rem;
  }
  
  .form-container {
    padding: 0 1rem 1rem;
  }
  
  .page-title {
    font-size: 1.6rem;
    max-width: calc(100vw - 5rem);
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
  
  .config-form {
    gap: 1.5rem;
  }
}

@media (max-width: 600px) {
  .title-section {
    gap: 0.75rem;
  }
  
  .page-title {
    font-size: 1.6rem;
    max-width: calc(100vw - 5rem);
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
}

@media (max-width: 480px) {
  .title-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }
  
  .page-title {
    font-size: 1.5rem;
    max-width: 100%;
    white-space: normal;
    word-break: keep-all;
    overflow-wrap: break-word;
  }
}
</style>