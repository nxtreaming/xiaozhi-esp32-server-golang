<template>
  <div class="agent-devices-page">
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" type="text" class="back-btn">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <div class="header-info">
          <h2>设备管理</h2>
          <p class="page-subtitle">管理智能体关联的设备</p>
        </div>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showAddDeviceDialog = true">
          <el-icon><Plus /></el-icon>
          添加设备
        </el-button>
      </div>
    </div>

    <div v-if="devices.length === 0" class="empty-section">
      <el-card class="empty-card">
        <div class="empty-content">
          <el-icon size="64" color="#909399"><Monitor /></el-icon>
          <h3>暂无设备</h3>
          <p>该智能体还没有关联任何设备。</p>
          <div class="empty-actions">
            <el-button type="primary" size="large" @click="showAddDeviceDialog = true">
              <el-icon><Plus /></el-icon>
              添加第一个设备
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="devices-grid">
      <div v-for="device in devices" :key="device.id" class="device-item">
        <div class="device-card">
          <div class="device-header">
            <div class="device-icon">
              <el-icon size="28"><Monitor /></el-icon>
            </div>
            <div class="device-info">
              <h3 class="device-name">{{ device.device_name || '未命名设备' }}</h3>
              <p class="device-code">{{ device.device_code }}</p>
            </div>
            <div class="device-status">
              <span :class="['status-dot', isDeviceOnline(device.last_active_at) ? 'online' : 'offline']"></span>
              <span class="status-text">{{ isDeviceOnline(device.last_active_at) ? '在线' : '离线' }}</span>
            </div>
          </div>
          
          <div class="device-meta">
            <div class="meta-row">
              <span class="meta-label">设备类型</span>
              <span class="meta-value">ESP32设备</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">激活状态</span>
              <span class="meta-value">
                <el-tag :type="device.activated ? 'success' : 'warning'" size="small">
                  {{ device.activated ? '已激活' : '未激活' }}
                </el-tag>
              </span>
            </div>
            <div class="meta-row">
              <span class="meta-label">最后活跃</span>
              <span class="meta-value">{{ formatDate(device.last_active_at) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">创建时间</span>
              <span class="meta-value">{{ formatDate(device.created_at) }}</span>
            </div>
          </div>
          
          <div class="device-actions">
            <el-button size="small" @click="handleDeviceRole(device.id)">
              <el-icon><User /></el-icon>
              角色
            </el-button>
            <el-button size="small" @click="handleDeviceConfig(device.id)">
              <el-icon><Setting /></el-icon>
              配置
            </el-button>
            <el-button size="small" type="danger" @click="handleRemoveDevice(device.id)">
              <el-icon><Delete /></el-icon>
              移除
            </el-button>
          </div>
        </div>
      </div>
    </div>

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

    <!-- 设备角色配置弹窗 -->
    <el-dialog
      v-model="showRoleConfigDialog"
      title="设备角色配置"
      width="700px"
      @close="handleCloseRoleConfig"
    >
      <div v-loading="roleConfigLoading">
        <div class="role-config-content">
          <el-alert
            title="配置说明"
            type="info"
            :closable="false"
            style="margin-bottom: 16px"
          >
            设备关联角色后，将使用角色的配置（Prompt、LLM、TTS）覆盖智能体的配置。如需使用智能体配置，请取消关联角色。
          </el-alert>

          <el-form label-width="120px">
            <el-form-item label="当前角色">
              <div v-if="currentDevice.role_id">
                <el-tag type="success" size="large">已关联角色</el-tag>
                <div class="current-role-info">
                  <p><strong>角色ID:</strong> {{ currentDevice.role_id }}</p>
                </div>
              </div>
              <el-tag v-else type="info" size="large">未关联角色（使用智能体配置）</el-tag>
            </el-form-item>

            <el-form-item label="选择角色">
              <el-select
                v-model="selectedRoleId"
                placeholder="选择角色（可选）"
                style="width: 100%"
                clearable
                filterable
                @change="handleRoleSelect"
              >
                <el-option
                  v-for="role in availableRoles"
                  :key="role.id"
                  :label="role.name"
                  :value="role.id"
                >
                  <div class="role-option-item">
                    <div class="role-option-main">
                      <span>{{ role.name }}</span>
                      <el-tag v-if="role.role_type === 'global'" size="small" type="success">全局</el-tag>
                    </div>
                    <el-tag size="small" type="info">LLM: {{ role.llm_config_id || '默认' }}</el-tag>
                  </div>
                </el-option>
              </el-select>
              <div class="form-help">
                选择角色后，设备将使用角色的配置。留空则取消角色关联。
              </div>
            </el-form-item>

            <el-form-item label="角色详情" v-if="selectedRole">
              <el-card class="role-preview-card">
                <div class="role-preview-content">
                  <p><strong>名称:</strong> {{ selectedRole.name }}</p>
                  <p v-if="selectedRole.description"><strong>描述:</strong> {{ selectedRole.description }}</p>
                  <el-divider />
                  <p><strong>Prompt:</strong></p>
                  <p class="prompt-preview">{{ selectedRole.prompt.substring(0, 200) }}{{ selectedRole.prompt.length > 200 ? '...' : '' }}</p>
                  <div class="role-configs-preview">
                    <el-tag size="small">LLM: {{ selectedRole.llm_config_id || '默认' }}</el-tag>
                    <el-tag size="small">TTS: {{ selectedRole.tts_config_id || '默认' }}</el-tag>
                    <el-tag v-if="selectedRole.voice" size="small">音色: {{ selectedRole.voice }}</el-tag>
                  </div>
                </div>
              </el-card>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <template #footer>
        <el-button @click="handleCloseRoleConfig">取消</el-button>
        <el-button
          type="primary"
          @click="handleApplyRole"
          :loading="roleConfigLoading"
          :disabled="!selectedRoleId && !currentDevice.role_id"
        >
          {{ selectedRoleId ? '应用角色' : '取消角色' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Plus, Monitor, Setting, Delete, User } from '@element-plus/icons-vue'
import api from '../../utils/api'

const router = useRouter()
const route = useRoute()

const agentId = route.params.id
const devices = ref([])
const showAddDeviceDialog = ref(false)
const addingDevice = ref(false)
const deviceFormRef = ref()

// 设备角色配置相关
const showRoleConfigDialog = ref(false)
const roleConfigLoading = ref(false)
const currentDevice = ref({})
const selectedRoleId = ref(null)
const selectedRole = ref(null)
const availableRoles = ref([])
const isRoleActive = (role) => role?.status === 'active' || !role?.status

const deviceForm = reactive({
  code: ''
})

const deviceRules = {
  code: [
    { required: true, message: '请输入设备验证码', trigger: 'blur' },
    { len: 6, message: '验证码长度为6位', trigger: 'blur' }
  ]
}

const loadDevices = async () => {
  try {
    const response = await api.get(`/user/agents/${agentId}/devices`)
    devices.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载设备列表失败')
  }
}

const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post(`/user/agents/${agentId}/devices`, {
      code: deviceForm.code
    })
    
    if (response.data.success) {
      ElMessage.success('设备添加成功')
      handleCloseAddDevice()
      await loadDevices()
    }
  } catch (error) {
    console.error('添加设备失败:', error)
    ElMessage.error('添加设备失败')
  } finally {
    addingDevice.value = false
  }
}

const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { code: '' })
}

const handleDeviceConfig = (deviceId) => {
  ElMessage.info('设备配置功能开发中')
}

// 加载角色列表
const loadRoles = async () => {
  try {
    const response = await api.get('/user/roles')
    const globalRoles = response.data.data?.global_roles || []
    const userRoles = response.data.data?.user_roles || []
    availableRoles.value = [...globalRoles, ...userRoles].filter(isRoleActive)
  } catch (error) {
    console.error('加载角色列表失败:', error)
  }
}

// 打开设备角色配置弹窗
const handleDeviceRole = async (deviceId) => {
  const device = devices.value.find(d => d.id === deviceId)
  if (!device) return

  currentDevice.value = { ...device }
  selectedRoleId.value = device.role_id || null
  selectedRole.value = null

  // 加载角色列表（如果还没有加载）
  if (availableRoles.value.length === 0) {
    await loadRoles()
  }

  // 如果已有关联角色，查找角色信息
  if (device.role_id) {
    const role = availableRoles.value.find(r => r.id === device.role_id)
    if (role) {
      selectedRole.value = role
    }
  }

  showRoleConfigDialog.value = true
}

// 处理角色选择变化
const handleRoleSelect = (roleId) => {
  if (!roleId) {
    selectedRole.value = null
    return
  }
  const role = availableRoles.value.find(r => r.id === roleId)
  if (role) {
    selectedRole.value = role
  }
}

// 应用角色到设备
const handleApplyRole = async () => {
  if (!currentDevice.value.id) return

  roleConfigLoading.value = true
  try {
    const data = {
      role_id: selectedRoleId.value || null
    }

    await api.post(`/devices/${currentDevice.value.id}/apply-role`, data)
    ElMessage.success(selectedRoleId.value ? '角色已应用到设备' : '已取消设备角色')
    showRoleConfigDialog.value = false
    await loadDevices()
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
  } finally {
    roleConfigLoading.value = false
  }
}

// 关闭角色配置弹窗
const handleCloseRoleConfig = () => {
  showRoleConfigDialog.value = false
  currentDevice.value = {}
  selectedRoleId.value = null
  selectedRole.value = null
}

const handleRemoveDevice = async (deviceId) => {
  try {
    await ElMessageBox.confirm(
      '确定要移除这个设备吗？',
      '确认移除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    const response = await api.delete(`/user/agents/${agentId}/devices/${deviceId}`)
    if (response.data.success) {
      ElMessage.success('设备移除成功')
      await loadDevices()
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('移除设备失败')
    }
  }
}

const goBack = () => {
  router.push('/agents')
}

const formatDate = (dateString) => {
  if (!dateString) return '从未'
  return new Date(dateString).toLocaleString('zh-CN')
}

// 判断设备是否在线（基于最后活跃时间）
const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  // 5分钟内有活动认为在线
  return (now - lastActive) < 5 * 60 * 1000
}

onMounted(() => {
  loadDevices()
  loadRoles()
})
</script>

<style scoped>
.agent-devices-page {
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

.header-left {
  display: flex;
  align-items: center;
  gap: 15px;
}

.back-btn {
  padding: 8px;
  color: #409EFF;
}

.header-info h2 {
  margin: 0;
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.empty-section {
  margin-top: 40px;
}

.empty-card {
  text-align: center;
  padding: 40px 20px;
}

.empty-content h3 {
  margin: 20px 0 10px 0;
  color: #333;
}

.empty-content p {
  color: #666;
  margin-bottom: 30px;
}

.devices-grid {
  margin-top: 20px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 440px));
  gap: 20px 12px;
  justify-content: flex-start;
}

.device-item {
  min-width: 0;
}

.device-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  height: 100%;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 440px;
  min-width: 0;
}

.device-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.device-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.device-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.device-info {
  flex: 1;
  min-width: 0;
}

.device-name {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.device-code {
  margin: 0;
  font-size: 12px;
  color: #999;
  font-family: monospace;
}

.device-status {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ddd;
}

.status-dot.online {
  background: #67c23a;
}

.status-dot.offline {
  background: #f56c6c;
}

.status-text {
  font-size: 12px;
  color: #666;
}

.device-meta {
  flex: 1;
  margin-bottom: 16px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.meta-row:last-child {
  margin-bottom: 0;
}

.meta-label {
  font-size: 12px;
  color: #999;
}

.meta-value {
  font-size: 12px;
  color: #666;
  font-weight: 500;
}

.device-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: auto;
}

.device-actions .el-button {
  min-width: 0;
  width: 100%;
  padding: 0 8px;
}

.device-actions :deep(.el-button > span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-dialog-content {
  text-align: center;
  padding: 20px 0;
}

.device-dialog-content .device-icon {
  margin: 0 auto 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.device-tip {
  margin-bottom: 20px;
  color: #666;
  font-size: 14px;
}

.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.dialog-footer .el-button {
  min-width: 80px;
}

/* 设备角色配置相关样式 */
.role-config-content {
  padding: 20px 0;
}

.current-role-info {
  margin-bottom: 16px;
}

.current-role-info p {
  margin: 4px 0;
  color: #666;
}

.role-option-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
}

.role-option-main {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.role-preview-card {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
}

.role-preview-content {
  font-size: 14px;
}

.role-preview-content p {
  margin: 8px 0;
}

.role-preview-content strong {
  color: #333;
  margin-right: 8px;
}

.role-configs-preview {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.prompt-preview {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #666;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: 15px;
  }
  
  .header-left {
    justify-content: flex-start;
  }
  
  .header-right {
    align-self: flex-end;
  }

  .devices-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .device-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .device-actions .el-button:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .devices-grid {
    gap: 10px;
  }

  .device-actions {
    grid-template-columns: 1fr;
  }
}
</style>
