<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>VAD配置管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showDialog = true">
          <el-icon><Plus /></el-icon>
          添加配置
        </el-button>
      </div>
    </div>

    <el-table :data="configs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="配置名称" />
      <el-table-column prop="config_id" label="配置ID" width="150" />
      <el-table-column prop="provider" label="提供商" />
      <el-table-column prop="enabled" label="启用状态" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.enabled" 
            @change="toggleEnable(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="默认配置" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.is_default" 
            @change="toggleDefault(scope.row)"
            :disabled="scope.row.is_default && getEnabledConfigs().length === 1"
          />
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="scope">
          <el-button size="small" @click="editConfig(scope.row)">编辑</el-button>
          <el-button
            size="small"
            type="danger"
            @click="deleteConfig(scope.row.id)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑配置弹窗 -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? '编辑VAD配置' : '添加VAD配置'"
      width="600px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" placeholder="请选择提供商" style="width: 100%">
            <el-option label="WebRTC VAD" value="webrtc_vad" />
            <el-option label="Silero VAD" value="silero_vad" />
            <el-option label="TEN VAD" value="ten_vad" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="配置名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入配置名称" />
        </el-form-item>
        
        <el-form-item label="配置ID" prop="config_id">
          <el-input v-model="form.config_id" placeholder="请输入唯一的配置ID" />
        </el-form-item>
        
        <!-- 移除是否默认开关，现在在列表页操作 -->
        
        <!-- WebRTC VAD 配置 -->
        <template v-if="form.provider === 'webrtc_vad'">
          <el-divider content-position="left">WebRTC VAD 配置</el-divider>
          <el-form-item label="最小连接池大小" prop="webrtc_vad.pool_min_size">
            <el-input-number v-model="form.webrtc_vad.pool_min_size" :min="1" :max="1000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="最大连接池大小" prop="webrtc_vad.pool_max_size">
            <el-input-number v-model="form.webrtc_vad.pool_max_size" :min="1" :max="10000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="最大空闲连接数" prop="webrtc_vad.pool_max_idle">
            <el-input-number v-model="form.webrtc_vad.pool_max_idle" :min="1" :max="1000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="VAD采样率" prop="webrtc_vad.vad_sample_rate">
            <el-select v-model="form.webrtc_vad.vad_sample_rate" style="width: 100%">
              <el-option label="8000 Hz" :value="8000" />
              <el-option label="16000 Hz" :value="16000" />
              <el-option label="32000 Hz" :value="32000" />
              <el-option label="48000 Hz" :value="48000" />
            </el-select>
          </el-form-item>
          <el-form-item label="VAD模式" prop="webrtc_vad.vad_mode">
            <el-select v-model="form.webrtc_vad.vad_mode" style="width: 100%">
              <el-option label="模式0 (质量优先)" :value="0" />
              <el-option label="模式1 (低延迟)" :value="1" />
              <el-option label="模式2 (平衡)" :value="2" />
              <el-option label="模式3 (高精度)" :value="3" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Silero VAD 配置 -->
        <template v-if="form.provider === 'silero_vad'">
          <el-divider content-position="left">Silero VAD 配置</el-divider>
          <el-form-item label="模型路径" prop="silero_vad.model_path">
            <el-input v-model="form.silero_vad.model_path" placeholder="请输入模型文件路径" />
          </el-form-item>
          <el-form-item label="阈值" prop="silero_vad.threshold">
            <el-input-number v-model="form.silero_vad.threshold" :min="0" :max="1" :step="0.1" :precision="2" style="width: 100%" />
          </el-form-item>
          <el-form-item label="最小静音持续时间(ms)" prop="silero_vad.min_silence_duration_ms">
            <el-input-number v-model="form.silero_vad.min_silence_duration_ms" :min="10" :max="5000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="采样率" prop="silero_vad.sample_rate">
            <el-select v-model="form.silero_vad.sample_rate" style="width: 100%">
              <el-option label="8000 Hz" :value="8000" />
              <el-option label="16000 Hz" :value="16000" />
              <el-option label="32000 Hz" :value="32000" />
              <el-option label="48000 Hz" :value="48000" />
            </el-select>
          </el-form-item>
          <el-form-item label="声道数" prop="silero_vad.channels">
            <el-select v-model="form.silero_vad.channels" style="width: 100%">
              <el-option label="单声道" :value="1" />
              <el-option label="双声道" :value="2" />
            </el-select>
          </el-form-item>
          <el-form-item label="连接池大小" prop="silero_vad.pool_size">
            <el-input-number v-model="form.silero_vad.pool_size" :min="1" :max="100" style="width: 100%" />
          </el-form-item>
          <el-form-item label="获取超时时间(ms)" prop="silero_vad.acquire_timeout_ms">
            <el-input-number v-model="form.silero_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
          </el-form-item>
        </template>

        <!-- TEN VAD 配置 -->
        <template v-if="form.provider === 'ten_vad'">
          <el-divider content-position="left">TEN VAD 配置</el-divider>
          <el-form-item label="帧移大小" prop="ten_vad.hop_size">
            <el-input-number v-model="form.ten_vad.hop_size" :min="128" :max="1024" style="width: 100%" />
            <div style="font-size: 12px; color: #909399; margin-top: 4px;">推荐值：512</div>
          </el-form-item>
          <el-form-item label="VAD检测阈值" prop="ten_vad.threshold">
            <el-input-number v-model="form.ten_vad.threshold" :min="0" :max="1" :step="0.1" :precision="2" style="width: 100%" />
            <div style="font-size: 12px; color: #909399; margin-top: 4px;">推荐值：0.3</div>
          </el-form-item>
          <el-form-item label="连接池大小" prop="ten_vad.pool_size">
            <el-input-number v-model="form.ten_vad.pool_size" :min="1" :max="100" style="width: 100%" />
            <div style="font-size: 12px; color: #909399; margin-top: 4px;">推荐值：10</div>
          </el-form-item>
          <el-form-item label="获取超时时间(ms)" prop="ten_vad.acquire_timeout_ms">
            <el-input-number v-model="form.ten_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
            <div style="font-size: 12px; color: #909399; margin-top: 4px;">推荐值：3000</div>
          </el-form-item>
        </template>
      </el-form>
      
      <template #footer>
        <el-button @click="handleDialogClose">取消</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const configs = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()

const form = reactive({
  name: '',
  config_id: '',
  provider: '',
  is_default: false,
  enabled: true,
  webrtc_vad: {
    pool_min_size: 5,
    pool_max_size: 1000,
    pool_max_idle: 100,
    vad_sample_rate: 16000,
    vad_mode: 2
  },
  silero_vad: {
    model_path: 'config/models/vad/silero_vad.onnx',
    threshold: 0.5,
    min_silence_duration_ms: 100,
    sample_rate: 16000,
    channels: 1,
    pool_size: 10,
    acquire_timeout_ms: 3000
  },
  ten_vad: {
    hop_size: 512,
    threshold: 0.3,
    pool_size: 10,
    acquire_timeout_ms: 3000
  }
})

// 根据provider生成配置JSON
const generateConfig = () => {
  if (form.provider === 'webrtc_vad') {
    return JSON.stringify({ webrtc_vad: form.webrtc_vad })
  } else if (form.provider === 'silero_vad') {
    return JSON.stringify({ silero_vad: form.silero_vad })
  } else if (form.provider === 'ten_vad') {
    return JSON.stringify({ ten_vad: form.ten_vad })
  }
  return '{}'
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  config_id: [{ required: true, message: '请输入配置ID', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  'webrtc_vad.pool_min_size': [{ required: true, message: '请输入最小连接池大小', trigger: 'blur' }],
  'webrtc_vad.pool_max_size': [{ required: true, message: '请输入最大连接池大小', trigger: 'blur' }],
  'webrtc_vad.pool_max_idle': [{ required: true, message: '请输入最大空闲连接数', trigger: 'blur' }],
  'webrtc_vad.vad_sample_rate': [{ required: true, message: '请选择VAD采样率', trigger: 'change' }],
  'webrtc_vad.vad_mode': [{ required: true, message: '请选择VAD模式', trigger: 'change' }],
  'silero_vad.model_path': [{ required: true, message: '请输入模型路径', trigger: 'blur' }],
  'silero_vad.threshold': [{ required: true, message: '请输入阈值', trigger: 'blur' }],
  'silero_vad.min_silence_duration_ms': [{ required: true, message: '请输入最小静音持续时间', trigger: 'blur' }],
  'silero_vad.sample_rate': [{ required: true, message: '请选择采样率', trigger: 'change' }],
  'silero_vad.channels': [{ required: true, message: '请选择声道数', trigger: 'change' }],
  'silero_vad.pool_size': [{ required: true, message: '请输入连接池大小', trigger: 'blur' }],
  'silero_vad.acquire_timeout_ms': [{ required: true, message: '请输入获取超时时间', trigger: 'blur' }],
  'ten_vad.hop_size': [{ required: true, message: '请输入帧移大小', trigger: 'blur' }],
  'ten_vad.threshold': [{ required: true, message: '请输入VAD检测阈值', trigger: 'blur' }],
  'ten_vad.pool_size': [{ required: true, message: '请输入连接池大小', trigger: 'blur' }],
  'ten_vad.acquire_timeout_ms': [{ required: true, message: '请输入获取超时时间', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/vad-configs')
    configs.value = response.data.data || []
  } catch (error) {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

const editConfig = (config) => {
  editingConfig.value = config
  form.name = config.name
  form.config_id = config.config_id
  form.provider = config.provider
  form.is_default = config.is_default
  form.enabled = config.enabled
  
  // 解析配置JSON并填充到对应字段
  try {
    const configObj = JSON.parse(config.json_data || '{}')
    if (configObj.webrtc_vad) {
      form.webrtc_vad = { ...form.webrtc_vad, ...configObj.webrtc_vad }
    }
    if (configObj.silero_vad) {
      form.silero_vad = { ...form.silero_vad, ...configObj.silero_vad }
    }
    if (configObj.ten_vad) {
      form.ten_vad = { ...form.ten_vad, ...configObj.ten_vad }
    }
  } catch (error) {
    console.error('解析配置JSON失败:', error)
  }
  
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        // 如果是新增配置且当前没有任何配置，则自动设为默认配置
        const isFirstConfig = !editingConfig.value && configs.value.length === 0
        
        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // 首次添加时自动设为默认
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: generateConfig()
        }

        if (editingConfig.value) {
          await api.put(`/admin/vad-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('配置更新成功')
        } else {
          await api.post('/admin/vad-configs', configData)
          ElMessage.success('配置创建成功')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        ElMessage.error('保存失败: ' + (error.response?.data?.message || error.message))
      } finally {
        saving.value = false
      }
    }
  })
}

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`)
    ElMessage.success(`${config.enabled ? '启用' : '禁用'}成功`)
  } catch (error) {
    // 恢复开关状态
    config.enabled = !config.enabled
    ElMessage.error('操作失败')
  }
}

const toggleDefault = async (config) => {
  try {
    if (!config.enabled) {
      ElMessage.warning('请先启用该配置才能设为默认')
      config.is_default = false
      return
    }
    
    const configData = {
      name: config.name,
      config_id: config.config_id,
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data
    }
    
    await api.put(`/admin/vad-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? '设为默认成功' : '取消默认成功')
    
    // 刷新列表以更新其他配置的默认状态
    loadConfigs()
  } catch (error) {
    // 恢复开关状态
    config.is_default = !config.is_default
    ElMessage.error('操作失败')
  }
}

const getEnabledConfigs = () => {
  return configs.value.filter(config => config.enabled)
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('确定要删除这个配置吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.delete(`/admin/vad-configs/${id}`)
    ElMessage.success('删除成功')
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const resetForm = () => {
  editingConfig.value = null
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: '',
    is_default: false,
    enabled: true,
    webrtc_vad: {
      pool_min_size: 5,
      pool_max_size: 1000,
      pool_max_idle: 100,
      vad_sample_rate: 16000,
      vad_mode: 2
    },
    silero_vad: {
      model_path: 'config/models/vad/silero_vad.onnx',
      threshold: 0.5,
      min_silence_duration_ms: 100,
      sample_rate: 16000,
      channels: 1,
      pool_size: 10,
      acquire_timeout_ms: 3000
    },
    ten_vad: {
      hop_size: 512,
      threshold: 0.3,
      pool_size: 10,
      acquire_timeout_ms: 3000
    }
  })
}

const handleDialogClose = () => {
  showDialog.value = false
  resetForm()
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadConfigs()
})
</script>

<style scoped>
.config-page {
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left h2 {
  margin: 0;
  color: #333;
}
</style>