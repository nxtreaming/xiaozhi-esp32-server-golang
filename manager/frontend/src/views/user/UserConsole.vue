<template>
  <div class="user-console">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-bg"></div>
      <div class="header-content">
        <div class="welcome-section">
          <div class="avatar-section">
            <div class="user-avatar">
              <el-icon><Avatar /></el-icon>
            </div>
            <div class="welcome-text">
              <h1 class="welcome-title">欢迎回来！</h1>
              <p class="welcome-subtitle">管理您的智能设备和AI助手</p>
            </div>
          </div>
          <div class="quick-stats">
            <div class="stat-item online">
              <div class="stat-icon">
                <el-icon><Connection /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ onlineDevicesCount }}</div>
                <div class="stat-label">在线设备</div>
              </div>
            </div>
            <div class="stat-item agents">
              <div class="stat-icon">
                <el-icon><Monitor /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ agents.length }}</div>
                <div class="stat-label">智能体</div>
              </div>
            </div>
            <div class="stat-item active">
              <div class="stat-icon">
                <el-icon><CircleCheck /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ activeAgentsCount }}</div>
                <div class="stat-label">活跃助手</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 主要内容区域 -->
    <div class="main-content">
      <!-- 设备管理 -->
      <div class="content-section">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="title-icon"><Cpu /></el-icon>
            <span>智能设备</span>
            <span class="device-count">{{ allDevicesData.length }}</span>
          </div>
          <div class="action-buttons">
            <el-button type="success" @click="openInjectMessageDialog" class="add-btn">
              <el-icon><ChatDotRound /></el-icon>
              消息注入
            </el-button>
            <el-button type="primary" @click="addDevice" class="add-btn">
              <el-icon><Plus /></el-icon>
              添加设备
            </el-button>
          </div>
        </div>
        
        <div v-if="devices.length === 0" class="empty-container">
          <div class="empty-content">
            <div class="empty-icon">
              <el-icon><Monitor /></el-icon>
            </div>
            <h3>还没有设备</h3>
            <p>添加您的第一个智能设备，开始AI交互之旅</p>
            <el-button type="primary" size="large" @click="addDevice">
              <el-icon><Plus /></el-icon>
              添加设备
            </el-button>
          </div>
        </div>
        
        <div v-else class="devices-grid">
          <div v-for="device in devices" :key="device.id" class="device-item">
            <div class="device-card">
              <div class="device-status">
                <div class="status-indicator" :class="isDeviceOnline(device.last_active_at) ? 'online' : 'offline'"></div>
                <span class="status-text">{{ isDeviceOnline(device.last_active_at) ? '在线' : '离线' }}</span>
              </div>
              
              <div class="device-info">
                <div class="device-icon">
                  <el-icon><Monitor /></el-icon>
                </div>
                <div class="device-details">
                  <h3 class="device-name">{{ device.device_name || '未命名设备' }}</h3>
                  <p class="device-desc">{{ device.device_code }}</p>
                </div>
              </div>
              
              <div class="device-features">
                <div class="feature-item">
                  <el-icon class="feature-icon"><Microphone /></el-icon>
                  <span class="feature-label">语音识别</span>
                  <el-switch 
                    v-model="device.vad_status" 
                    @change="toggleVAD(device)"
                    :loading="device.loading"
                    size="small"
                  />
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><User /></el-icon>
                  <span class="feature-label">智能体</span>
                  <span class="feature-value">{{ device.agent_name || '未绑定' }}</span>
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><CircleCheck /></el-icon>
                  <span class="feature-label">激活状态</span>
                  <span class="feature-value">
                    <el-tag :type="device.activated ? 'success' : 'warning'" size="small">
                      {{ device.activated ? '已激活' : '未激活' }}
                    </el-tag>
                  </span>
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><Clock /></el-icon>
                  <span class="feature-label">活跃时间</span>
                  <span class="feature-value">{{ formatTime(device.last_active_at) }}</span>
                </div>
              </div>
              
              <div class="device-actions">
                <el-button type="primary" size="small" @click="openDeviceControl(device)" class="control-btn">
                  <el-icon><Setting /></el-icon>
                  控制面板
                </el-button>
              </div>
            </div>
          </div>
        </div>
        
        <!-- 查看更多 -->
        <div v-if="allDevicesData.length > 6" class="load-more">
          <el-button 
            type="text" 
            @click="toggleShowAllDevices"
            class="load-more-btn"
          >
            <span v-if="!showAllDevices">
              显示全部设备 ({{ allDevicesData.length - 6 }}+)
              <el-icon><ArrowDown /></el-icon>
            </span>
            <span v-else>
              收起设备列表
              <el-icon><ArrowUp /></el-icon>
            </span>
          </el-button>
        </div>
      </div>

      <!-- 智能体管理 -->
      <div class="content-section">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="title-icon"><User /></el-icon>
            <span>AI 智能体</span>
            <span class="device-count">{{ agents.length }}</span>
          </div>
          <el-button type="primary" @click="$router.push('/agents')" class="add-btn">
            <el-icon><Setting /></el-icon>
            管理智能体
          </el-button>
        </div>
        
        <div v-if="agents.length === 0" class="empty-container">
          <div class="empty-content">
            <div class="empty-icon">
              <el-icon><User /></el-icon>
            </div>
            <h3>还没有智能体</h3>
            <p>创建您的专属AI助手，享受个性化服务</p>
            <el-button type="primary" size="large" @click="$router.push('/agents')">
              <el-icon><Plus /></el-icon>
              创建智能体
            </el-button>
          </div>
        </div>
        
        <div v-else class="agents-grid">
          <div v-for="agent in agents.slice(0, 4)" :key="agent.id" class="agent-item">
            <div class="agent-card" @click="selectAgent(agent)">
              <div class="agent-status">
                <div class="status-indicator" :class="agent.status === 'active' ? 'online' : 'offline'"></div>
                <span class="status-text">{{ agent.status === 'active' ? '活跃' : '待机' }}</span>
              </div>
              
              <div class="agent-avatar">
                <div class="avatar-bg" :class="getAgentAvatarClass(agent)">
                  <el-icon><User /></el-icon>
                </div>
              </div>
              
              <div class="agent-info">
                <h3 class="agent-name">{{ agent.name }}</h3>
                <p class="agent-desc">{{ agent.description || '智能AI助手' }}</p>
              </div>
              
              <div class="agent-stats">
                <div class="stat-row">
                  <span class="stat-label">对话次数</span>
                  <span class="stat-value">{{ agent.conversation_count || 0 }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">创建时间</span>
                  <span class="stat-value">{{ formatDate(agent.created_at) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div v-if="agents.length > 4" class="load-more">
          <el-button type="text" @click="$router.push('/agents')" class="load-more-btn">
            查看全部智能体 ({{ agents.length - 4 }}+)
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </div>
    </div>



    <!-- 设备控制弹窗 -->
    <el-dialog
      v-model="showDeviceControl"
      :title="`控制设备: ${currentDevice?.name}`"
      width="600px"
    >
      <div v-if="currentDevice" class="device-control-panel">
        <div class="control-section">
          <h4>基础控制</h4>
          <div class="control-buttons">
            <el-button type="success" @click="sendCommand('wake_up')">
              <el-icon><VideoPlay /></el-icon>
              唤醒设备
            </el-button>
            <el-button type="warning" @click="sendCommand('sleep')">
              <el-icon><VideoPause /></el-icon>
              休眠设备
            </el-button>
            <el-button type="info" @click="sendCommand('restart')">
              <el-icon><Refresh /></el-icon>
              重启设备
            </el-button>
          </div>
        </div>
        
        <div class="control-section">
          <h4>语音控制</h4>
          <div class="voice-settings">
            <el-form label-width="100px">
              <el-form-item label="音量">
                <el-slider v-model="currentDevice.volume" :max="100" />
              </el-form-item>
              <el-form-item label="语音识别">
                <el-switch 
                  v-model="currentDevice.vad_status"
                  @change="toggleVAD(currentDevice)"
                />
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 消息注入弹窗 -->
    <el-dialog
      v-model="showInjectMessageDialog"
      title="消息注入"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="injectFormRef"
        :model="injectForm"
        :rules="injectRules"
        label-width="100px"
      >
        <el-form-item label="选择设备" prop="device_id">
          <el-select
            v-model="injectForm.device_id"
            placeholder="请选择要注入消息的设备"
            style="width: 100%"
            filterable
          >
            <el-option
              v-for="device in allDevicesData"
              :key="device.device_code"
              :label="`${device.device_name || '未命名设备'} (${device.device_code})`"
              :value="device.device_name || '未命名设备'"
            >
              <div class="device-option">
                <div class="device-option-header">
                  <span class="device-name">{{ device.device_name || '未命名设备' }}</span>
                  <el-tag 
                    :type="isDeviceOnline(device.last_active_at) ? 'success' : 'danger'" 
                    size="small"
                  >
                    {{ isDeviceOnline(device.last_active_at) ? '在线' : '离线' }}
                  </el-tag>
                </div>
                <div class="device-code">{{ device.device_code }}</div>
                <div class="device-agent">智能体: {{ device.agent_name || '未绑定' }}</div>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        
        <el-form-item label="消息内容" prop="message">
          <el-input
            v-model="injectForm.message"
            type="textarea"
            :rows="4"
            placeholder="请输入要注入的消息内容"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
        
        <el-form-item label="处理方式" prop="skip_llm">
          <el-radio-group v-model="injectForm.skip_llm">
            <el-radio :label="false">
              <div class="radio-option">
                <div class="radio-title">通过LLM处理</div>
                <div class="radio-desc">消息会经过AI智能体处理，生成智能回复</div>
              </div>
            </el-radio>
            <el-radio :label="true">
              <div class="radio-option">
                <div class="radio-title">直接播放</div>
                <div class="radio-desc">消息直接转换为语音播放，不经过AI处理</div>
              </div>
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseInjectMessage">取消</el-button>
          <el-button
            type="primary"
            :loading="injectingMessage"
            @click="handleInjectMessage"
          >
            {{ injectingMessage ? '注入中...' : '注入消息' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 添加设备弹窗 -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="添加设备"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="deviceFormRef"
        :model="deviceForm"
        :rules="deviceRules"
        label-width="100px"
      >
        <el-form-item label="设备名称" prop="device_name">
          <el-input
            v-model="deviceForm.device_name"
            placeholder="请输入设备名称"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
        
        <el-form-item label="关联智能体" prop="agent_id">
          <el-select
            v-model="deviceForm.agent_id"
            placeholder="请选择关联智能体"
            style="width: 100%"
          >
            <el-option
              v-for="agent in agents"
              :key="agent.id"
              :label="agent.name"
              :value="agent.id"
            >
              <div class="agent-option">
                <span class="agent-name">{{ agent.name }}</span>
                <span class="agent-desc">{{ agent.description || '智能AI助手' }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddDevice">取消</el-button>
          <el-button
            type="primary"
            :loading="addingDevice"
            @click="handleAddDevice"
          >
            {{ addingDevice ? '添加中...' : '添加设备' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Monitor,
  Connection,
  Plus,
  Setting,
  Microphone,
  VideoPlay,
  VideoPause,
  Refresh,
  ArrowDown,
  ArrowUp,
  ArrowRight,
  Avatar,
  CircleCheck,
  Cpu,
  User,
  Clock,
  ChatDotRound
} from '@element-plus/icons-vue'
import api from '../../utils/api'

const devices = ref([])
const agents = ref([])
const allDevicesData = ref([])
const showDeviceControl = ref(false)
const currentDevice = ref(null)
const showAllDevices = ref(false)

// 加载设备列表
const loadDevices = async () => {
  try {
    const response = await api.get('/user/devices')
    const allDevices = response.data.data || []
    // 保存所有设备数据
    allDevicesData.value = allDevices.map(device => ({
      ...device,
      loading: false,
      volume: device.volume || 80
    }))
    // 限制显示最多6个设备
    devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
    // 更新统计数据
    updateStats()
  } catch (error) {
    console.error('加载设备失败:', error)
    ElMessage.error('加载设备失败')
    devices.value = []
    allDevicesData.value = []
  }
}

// 加载智能体列表
const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
    // 更新统计数据
    updateStats()
  } catch (error) {
    console.error('加载智能体失败:', error)
    ElMessage.error('加载智能体失败')
    agents.value = []
  }
}

// 切换语音识别状态
const toggleVAD = async (device) => {
  device.loading = true
  try {
    // 模拟API调用
    await new Promise(resolve => setTimeout(resolve, 1000))
    device.vad_status = !device.vad_status
    ElMessage.success(`${device.vad_status ? '启用' : '禁用'}语音识别成功`)
  } catch (error) {
    console.error('切换语音识别失败:', error)
    ElMessage.error('操作失败')
  } finally {
    device.loading = false
  }
}

// 打开设备控制面板
const openDeviceControl = (device) => {
  currentDevice.value = device
  showDeviceControl.value = true
}

// 发送设备命令
const sendCommand = async (command) => {
  try {
    // 模拟API调用
    await new Promise(resolve => setTimeout(resolve, 500))
    ElMessage.success(`命令 ${command} 发送成功`)
  } catch (error) {
    console.error('发送命令失败:', error)
    ElMessage.error('发送命令失败')
  }
}

// 选择智能体
const selectAgent = (agent) => {
  ElMessage.info(`选择了智能体: ${agent.name}`)
  // 可以跳转到智能体详情页或执行其他操作
}

// 添加设备相关状态
const showAddDeviceDialog = ref(false)
const addingDevice = ref(false)
const deviceFormRef = ref()

// 消息注入相关状态
const showInjectMessageDialog = ref(false)
const injectingMessage = ref(false)
const injectFormRef = ref()

const deviceForm = reactive({
  device_name: '',
  agent_id: ''
})

const deviceRules = {
  device_name: [
    { required: true, message: '请输入设备名称', trigger: 'blur' },
    { min: 2, max: 50, message: '设备名称长度在2-50个字符之间', trigger: 'blur' }
  ],
  agent_id: [
    { required: true, message: '请选择关联智能体', trigger: 'change' }
  ]
}

const injectForm = reactive({
  device_id: '',
  message: '',
  skip_llm: false
})

const injectRules = {
  device_id: [
    { required: true, message: '请选择设备', trigger: 'change' }
  ],
  message: [
    { required: true, message: '请输入消息内容', trigger: 'blur' },
    { min: 1, max: 500, message: '消息长度在1-500个字符之间', trigger: 'blur' }
  ]
}

// 打开添加设备弹窗
const addDevice = () => {
  if (agents.value.length === 0) {
    ElMessage.warning('请先创建智能体，然后再添加设备')
    return
  }
  showAddDeviceDialog.value = true
}

// 处理添加设备
const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post('/user/devices', {
      device_name: deviceForm.device_name,
      agent_id: parseInt(deviceForm.agent_id)
    })
    
    if (response.data.success) {
      ElMessage.success('设备添加成功')
      handleCloseAddDevice()
      await loadDevices()
    }
  } catch (error) {
    console.error('添加设备失败:', error)
    ElMessage.error(error.response?.data?.error || '添加设备失败')
  } finally {
    addingDevice.value = false
  }
}

// 关闭添加设备弹窗
const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { device_name: '', agent_id: '' })
}

// 打开消息注入弹窗
const openInjectMessageDialog = () => {
  if (allDevicesData.value.length === 0) {
    ElMessage.warning('请先添加设备，然后再进行消息注入')
    return
  }
  
  showInjectMessageDialog.value = true
}

// 处理消息注入
const handleInjectMessage = async () => {
  if (!injectFormRef.value) return
  
  try {
    await injectFormRef.value.validate()
    injectingMessage.value = true
    
    const response = await api.post('/user/devices/inject-message', {
      device_id: injectForm.device_id,
      message: injectForm.message,
      skip_llm: injectForm.skip_llm
    })
    
    if (response.data.success) {
      ElMessage.success('消息注入成功')
      handleCloseInjectMessage()
    }
  } catch (error) {
    console.error('消息注入失败:', error)
    ElMessage.error(error.response?.data?.error || '消息注入失败')
  } finally {
    injectingMessage.value = false
  }
}

// 关闭消息注入弹窗
const handleCloseInjectMessage = () => {
  showInjectMessageDialog.value = false
  if (injectFormRef.value) {
    injectFormRef.value.resetFields()
  }
  Object.assign(injectForm, { device_id: '', message: '', skip_llm: false })
}

// 切换显示所有设备
const toggleShowAllDevices = () => {
  showAllDevices.value = !showAllDevices.value
  devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
}

// 计算属性
const onlineDevicesCount = ref(0)
const activeAgentsCount = ref(0)

// 判断设备是否在线（基于最后活跃时间）
const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  // 5分钟内有活动认为在线
  return (now - lastActive) < 5 * 60 * 1000
}

// 更新统计数据
const updateStats = () => {
  onlineDevicesCount.value = allDevicesData.value.filter(device => isDeviceOnline(device.last_active_at)).length
  activeAgentsCount.value = agents.value.filter(agent => agent.status === 'active').length
}

// 获取智能体头像类
const getAgentAvatarClass = (agent) => {
  const classes = ['avatar-blue', 'avatar-green', 'avatar-purple', 'avatar-orange']
  return classes[agent.id % classes.length] || 'avatar-blue'
}

// 格式化时间
const formatTime = (date) => {
  if (!date) return '未知'
  const now = new Date()
  const diff = now - new Date(date)
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 30) return `${days}天前`
  return `${Math.floor(days / 30)}个月前`
}

// 格式化日期
const formatDate = (dateString) => {
  if (!dateString) return '--'
  return new Date(dateString).toLocaleDateString('zh-CN')
}

onMounted(() => {
  loadDevices()
  loadAgents()
})
</script>

<style scoped>
.user-console {
  min-height: 100vh;
  background: #f8f9fa;
  padding: 0;
}

/* 页面头部样式 */
.page-header {
  background: #ffffff;
  padding: 24px 0;
  margin-bottom: 0;
  border-bottom: 1px solid #e9ecef;
}

.header-bg {
  display: none;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
  color: #495057;
}

.welcome-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 40px;
}

.avatar-section {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-avatar {
  width: 48px;
  height: 48px;
  background: #e9ecef;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: #6c757d;
  border: 1px solid #dee2e6;
}

.welcome-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: #212529;
}

.welcome-subtitle {
  font-size: 14px;
  margin: 0;
  color: #6c757d;
}

.quick-stats {
  display: flex;
  gap: 20px;
}

.quick-stats .stat-item {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #ffffff;
  padding: 16px 20px;
  border-radius: 8px;
  border: 1px solid #dee2e6;
}

.quick-stats .stat-icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: #ffffff;
}

.quick-stats .stat-item.online .stat-icon {
  background: #28a745;
}

.quick-stats .stat-item.agents .stat-icon {
  background: #007bff;
}

.quick-stats .stat-item.active .stat-icon {
  background: #17a2b8;
}

.quick-stats .stat-number {
  font-size: 18px;
  font-weight: 600;
  line-height: 1;
  color: #212529;
}

.quick-stats .stat-label {
  font-size: 12px;
  color: #6c757d;
  margin-top: 2px;
}

/* 主要内容区域 */
.main-content {
  max-width: 1200px;
  margin: 24px auto 40px;
  padding: 0 24px;
}

.content-section {
  background: white;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 24px;
  border: 1px solid #dee2e6;
}

/* 区域头部 */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e9ecef;
}

.action-buttons {
  display: flex;
  gap: 12px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  color: #212529;
}

.title-icon {
  width: 24px;
  height: 24px;
  background: #007bff;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 12px;
}

.device-count {
  background: #f8f9fa;
  color: #6c757d;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  margin-left: 8px;
}

.add-btn {
  height: 36px;
  padding: 0 16px;
  border-radius: 4px;
  font-weight: 500;
  font-size: 14px;
}

/* 设备网格 */
.devices-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px 12px;
}

.device-item {
  position: relative;
}

.device-card {
  background: white;
  border-radius: 6px;
  padding: 16px;
  border: 1px solid #dee2e6;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.device-card:hover {
  border-color: #007bff;
}

.device-status {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.online {
  background: #28a745;
}

.status-indicator.offline {
  background: #dc3545;
}

.status-text {
  font-size: 12px;
  font-weight: 500;
  color: #6c757d;
}

.device-info {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.device-icon {
  width: 40px;
  height: 40px;
  background: #007bff;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 18px;
}

.device-name {
  font-size: 16px;
  font-weight: 600;
  color: #212529;
  margin: 0 0 2px 0;
}

.device-desc {
  font-size: 12px;
  color: #6c757d;
  margin: 0;
}

.device-features {
  flex: 1;
  margin-bottom: 16px;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid #f8f9fa;
}

.feature-item:last-child {
  border-bottom: none;
}

.feature-icon {
  width: 20px;
  height: 20px;
  background: #f8f9fa;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #007bff;
  font-size: 12px;
}

.feature-label {
  font-size: 12px;
  color: #6c757d;
  flex: 1;
}

.feature-value {
  font-size: 12px;
  color: #212529;
  font-weight: 500;
}

.device-actions {
  margin-top: auto;
}

.control-btn {
  width: 100%;
  height: 32px;
  border-radius: 4px;
  font-weight: 500;
  font-size: 12px;
}

/* 智能体网格 */
.agents-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px 12px;
}

.agent-item {
  position: relative;
}

.agent-card {
  background: white;
  border-radius: 6px;
  padding: 16px;
  border: 1px solid #dee2e6;
  cursor: pointer;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.agent-card:hover {
  border-color: #007bff;
}

.agent-status {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.agent-avatar {
  display: flex;
  justify-content: center;
  margin-bottom: 12px;
}

.avatar-bg {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 20px;
}

.avatar-bg.avatar-blue {
  background: #007bff;
}

.avatar-bg.avatar-green {
  background: #28a745;
}

.avatar-bg.avatar-purple {
  background: #6f42c1;
}

.avatar-bg.avatar-orange {
  background: #fd7e14;
}

.agent-info {
  text-align: center;
  margin-bottom: 12px;
}

.agent-name {
  font-size: 14px;
  font-weight: 600;
  color: #212529;
  margin: 0 0 4px 0;
}

.agent-desc {
  font-size: 12px;
  color: #6c757d;
  margin: 0;
  line-height: 1.4;
}

.agent-stats {
  flex: 1;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid #f8f9fa;
}

.stat-row:last-child {
  border-bottom: none;
}

.stat-label {
  font-size: 12px;
  color: #6c757d;
}

.stat-value {
  font-size: 12px;
  color: #212529;
  font-weight: 500;
}

/* 空状态 */
.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
}

.empty-content {
  text-align: center;
  max-width: 300px;
}

.empty-icon {
  width: 64px;
  height: 64px;
  background: #f8f9fa;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #adb5bd;
  margin: 0 auto 16px;
}

.empty-content h3 {
  font-size: 16px;
  color: #212529;
  margin: 0 0 8px 0;
  font-weight: 600;
}

.empty-content p {
  font-size: 14px;
  color: #6c757d;
  margin: 0 0 16px 0;
  line-height: 1.4;
}

/* 加载更多 */
.load-more {
  text-align: center;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #e9ecef;
}

.load-more-btn {
  font-size: 14px;
  color: #007bff;
  font-weight: 500;
}

.load-more-btn:hover {
  color: #0056b3;
}

/* 响应式设计 */
@media (max-width: 1024px) {
  .welcome-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  
  .quick-stats {
    flex-wrap: wrap;
    gap: 12px;
  }
  
  .devices-grid {
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  }
}

@media (max-width: 768px) {
  .page-header {
    padding: 16px 0;
  }
  
  .header-content {
    padding: 0 16px;
  }
  
  .welcome-title {
    font-size: 20px;
  }
  
  .main-content {
    margin: 16px auto 24px;
    padding: 0 16px;
  }
  
  .content-section {
    padding: 16px;
    margin-bottom: 16px;
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}

/* 智能体选项样式 */
.agent-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.agent-option .agent-name {
  font-weight: 500;
  color: #212529;
}

.agent-option .agent-desc {
  font-size: 12px;
  color: #6c757d;
}

/* 弹窗样式 */
.dialog-footer {
  text-align: right;
}

/* 消息注入弹窗样式 */
.device-option {
  padding: 8px 0;
}

.device-option-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.device-option .device-name {
  font-weight: 500;
  color: #212529;
}

.device-code {
  font-size: 12px;
  color: #6c757d;
  margin-bottom: 2px;
}

.device-agent {
  font-size: 12px;
  color: #6c757d;
}

.radio-option {
  margin-left: 8px;
}

.radio-title {
  font-weight: 500;
  color: #212529;
  margin-bottom: 2px;
}

.radio-desc {
  font-size: 12px;
  color: #6c757d;
  line-height: 1.4;
}

@media (max-width: 480px) {
  .quick-stats {
    flex-direction: column;
    gap: 8px;
  }
  
  .devices-grid {
    grid-template-columns: 1fr;
    gap: 10px;
  }
  
  .agents-grid {
    grid-template-columns: 1fr;
  }
}
</style>
