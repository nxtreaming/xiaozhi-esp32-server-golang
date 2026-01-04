<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>TTS配置管理</h2>
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
      :title="editingConfig ? '编辑TTS配置' : '添加TTS配置'"
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
            <el-option label="CosyVoice" value="cosyvoice" />
            <el-option label="豆包 TTS" value="doubao" />
            <el-option label="豆包 WebSocket" value="doubao_ws" />
            <el-option label="Edge TTS" value="edge" />
            <el-option label="Edge 离线" value="edge_offline" />
            <el-option label="OpenAI" value="openai" />
            <el-option label="小智 TTS" value="xiaozhi" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="配置名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入配置名称" />
        </el-form-item>
        
        <el-form-item label="配置ID" prop="config_id">
          <el-input v-model="form.config_id" placeholder="请输入唯一的配置ID" />
        </el-form-item>
        
        <!-- 移除是否默认开关，现在在列表页操作 -->
        
        <!-- CosyVoice 配置 -->
        <template v-if="form.provider === 'cosyvoice'">
          <el-form-item label="API URL" prop="cosyvoice.api_url">
            <el-input v-model="form.cosyvoice.api_url" placeholder="请输入API URL" />
          </el-form-item>
          <el-form-item label="说话人ID" prop="cosyvoice.spk_id">
            <el-input v-model="form.cosyvoice.spk_id" placeholder="请输入说话人ID" />
          </el-form-item>
          <el-form-item label="帧时长" prop="cosyvoice.frame_duration">
            <el-input-number v-model="form.cosyvoice.frame_duration" :min="1" :max="1000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="目标采样率" prop="cosyvoice.target_sr">
            <el-input-number v-model="form.cosyvoice.target_sr" :min="8000" :max="48000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="音频格式" prop="cosyvoice.audio_format">
            <el-select v-model="form.cosyvoice.audio_format" placeholder="请选择音频格式" style="width: 100%">
              <el-option label="MP3" value="mp3" />
              <el-option label="WAV" value="wav" />
              <el-option label="PCM" value="pcm" />
            </el-select>
          </el-form-item>
          <el-form-item label="指令文本" prop="cosyvoice.instruct_text">
            <el-input v-model="form.cosyvoice.instruct_text" placeholder="请输入指令文本" />
          </el-form-item>
        </template>

        <!-- 豆包 TTS 配置 -->
        <template v-if="form.provider === 'doubao'">
          <el-form-item label="应用ID" prop="doubao.appid">
            <el-input v-model="form.doubao.appid" placeholder="请输入应用ID" />
          </el-form-item>
          <el-form-item label="访问令牌" prop="doubao.access_token">
            <el-input v-model="form.doubao.access_token" placeholder="请输入访问令牌" type="password" show-password />
          </el-form-item>
          <el-form-item label="集群" prop="doubao.cluster">
            <el-input v-model="form.doubao.cluster" placeholder="请输入集群名称" />
          </el-form-item>
          <el-form-item label="音色" prop="doubao.voice">
            <el-input v-model="form.doubao.voice" placeholder="请输入音色" />
          </el-form-item>
          <el-form-item label="API URL" prop="doubao.api_url">
            <el-input v-model="form.doubao.api_url" placeholder="请输入API URL" />
          </el-form-item>
          <el-form-item label="授权信息" prop="doubao.authorization">
            <el-input v-model="form.doubao.authorization" placeholder="请输入授权信息" type="password" show-password />
          </el-form-item>
        </template>

        <!-- 豆包 WebSocket 配置 -->
        <template v-if="form.provider === 'doubao_ws'">
          <el-form-item label="应用ID" prop="doubao_ws.appid">
            <el-input v-model="form.doubao_ws.appid" placeholder="请输入应用ID" />
          </el-form-item>
          <el-form-item label="访问令牌" prop="doubao_ws.access_token">
            <el-input v-model="form.doubao_ws.access_token" placeholder="请输入访问令牌" type="password" show-password />
          </el-form-item>
          <el-form-item label="集群" prop="doubao_ws.cluster">
            <el-input v-model="form.doubao_ws.cluster" placeholder="请输入集群名称" />
          </el-form-item>
          <el-form-item label="音色" prop="doubao_ws.voice">
            <el-input v-model="form.doubao_ws.voice" placeholder="请输入音色" />
          </el-form-item>
          <el-form-item label="WebSocket主机" prop="doubao_ws.ws_host">
            <el-input v-model="form.doubao_ws.ws_host" placeholder="请输入WebSocket主机地址" />
          </el-form-item>
          <el-form-item label="使用流式" prop="doubao_ws.use_stream">
            <el-switch v-model="form.doubao_ws.use_stream" />
          </el-form-item>
        </template>

        <!-- Edge TTS 配置 -->
        <template v-if="form.provider === 'edge'">
          <el-form-item label="音色" prop="edge.voice">
            <el-input v-model="form.edge.voice" placeholder="请输入音色" />
          </el-form-item>
          <el-form-item label="语速" prop="edge.rate">
            <el-input v-model="form.edge.rate" placeholder="请输入语速（如：+0%）" />
          </el-form-item>
          <el-form-item label="音量" prop="edge.volume">
            <el-input v-model="form.edge.volume" placeholder="请输入音量（如：+0%）" />
          </el-form-item>
          <el-form-item label="音调" prop="edge.pitch">
            <el-input v-model="form.edge.pitch" placeholder="请输入音调（如：+0Hz）" />
          </el-form-item>
          <el-form-item label="连接超时" prop="edge.connect_timeout">
            <el-input-number v-model="form.edge.connect_timeout" :min="1" :max="60" style="width: 100%" />
          </el-form-item>
          <el-form-item label="接收超时" prop="edge.receive_timeout">
            <el-input-number v-model="form.edge.receive_timeout" :min="1" :max="300" style="width: 100%" />
          </el-form-item>
        </template>

        <!-- Edge 离线配置 -->
        <template v-if="form.provider === 'edge_offline'">
          <el-form-item label="服务器URL" prop="edge_offline.server_url">
            <el-input v-model="form.edge_offline.server_url" placeholder="请输入服务器URL" />
          </el-form-item>
          <el-form-item label="超时时间" prop="edge_offline.timeout">
            <el-input-number v-model="form.edge_offline.timeout" :min="1" :max="300" style="width: 100%" />
          </el-form-item>
          <el-form-item label="采样率" prop="edge_offline.sample_rate">
            <el-input-number v-model="form.edge_offline.sample_rate" :min="8000" :max="48000" style="width: 100%" />
          </el-form-item>
          <el-form-item label="声道数" prop="edge_offline.channels">
            <el-input-number v-model="form.edge_offline.channels" :min="1" :max="8" style="width: 100%" />
          </el-form-item>
          <el-form-item label="帧时长" prop="edge_offline.frame_duration">
            <el-input-number v-model="form.edge_offline.frame_duration" :min="1" :max="100" style="width: 100%" />
          </el-form-item>
        </template>

        <!-- OpenAI TTS 配置 -->
        <template v-if="form.provider === 'openai'">
          <el-form-item label="API Key" prop="openai.api_key">
            <el-input v-model="form.openai.api_key" placeholder="请输入API Key" type="password" show-password />
          </el-form-item>
          <el-form-item label="API URL" prop="openai.api_url">
            <el-input v-model="form.openai.api_url" placeholder="请输入API URL（默认：https://api.openai.com/v1/audio/speech）" />
          </el-form-item>
          <el-form-item label="模型" prop="openai.model">
            <el-input v-model="form.openai.model" placeholder="请输入模型（默认：tts-1）" />
          </el-form-item>
          <el-form-item label="音色" prop="openai.voice">
            <el-select v-model="form.openai.voice" placeholder="请选择音色" style="width: 100%">
              <el-option label="alloy" value="alloy" />
              <el-option label="echo" value="echo" />
              <el-option label="fable" value="fable" />
              <el-option label="onyx" value="onyx" />
              <el-option label="nova" value="nova" />
              <el-option label="shimmer" value="shimmer" />
            </el-select>
          </el-form-item>
          <el-form-item label="响应格式" prop="openai.response_format">
            <el-select v-model="form.openai.response_format" placeholder="请选择响应格式" style="width: 100%">
              <el-option label="MP3" value="mp3" />
              <el-option label="Opus" value="opus" />
              <el-option label="AAC" value="aac" />
              <el-option label="FLAC" value="flac" />
              <el-option label="WAV" value="wav" />
              <el-option label="PCM" value="pcm" />
            </el-select>
          </el-form-item>
          <el-form-item label="语速" prop="openai.speed">
            <el-input-number v-model="form.openai.speed" :min="0.25" :max="4.0" :step="0.1" style="width: 100%" placeholder="0.25-4.0，默认1.0" />
          </el-form-item>
          <el-form-item label="使用流式" prop="openai.stream">
            <el-switch v-model="form.openai.stream" />
          </el-form-item>
          <el-form-item label="帧时长" prop="openai.frame_duration">
            <el-input-number v-model="form.openai.frame_duration" :min="1" :max="1000" style="width: 100%" placeholder="毫秒" />
          </el-form-item>
        </template>

        <!-- 小智 TTS 配置 -->
        <template v-if="form.provider === 'xiaozhi'">
          <el-form-item label="服务器地址" prop="xiaozhi.server_addr">
            <el-input v-model="form.xiaozhi.server_addr" placeholder="请输入服务器地址" />
          </el-form-item>
          <el-form-item label="设备ID" prop="xiaozhi.device_id">
            <el-input v-model="form.xiaozhi.device_id" placeholder="请输入设备ID" />
          </el-form-item>
          <el-form-item label="客户端ID" prop="xiaozhi.client_id">
            <el-input v-model="form.xiaozhi.client_id" placeholder="请输入客户端ID" />
          </el-form-item>
          <el-form-item label="令牌" prop="xiaozhi.token">
            <el-input v-model="form.xiaozhi.token" placeholder="请输入令牌" type="password" show-password />
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
  provider: 'xiaozhi',
  is_default: false,
  enabled: true,
  cosyvoice: {
    api_url: 'https://tts.linkerai.cn/tts',
    spk_id: 'spk_id',
    frame_duration: 60,
    target_sr: 24000,
    audio_format: 'mp3',
    instruct_text: '你好'
  },
  doubao: {
    appid: '6886011847',
    access_token: 'access_token',
    cluster: 'volcano_tts',
    voice: 'BV001_streaming',
    api_url: 'https://openspeech.bytedance.com/api/v1/tts',
    authorization: 'Bearer;'
  },
  doubao_ws: {
    appid: '6886011847',
    access_token: 'access_token',
    cluster: 'volcano_tts',
    voice: 'zh_female_wanwanxiaohe_moon_bigtts',
    ws_host: 'openspeech.bytedance.com',
    use_stream: true
  },
  edge: {
    voice: 'zh-CN-XiaoxiaoNeural',
    rate: '+0%',
    volume: '+0%',
    pitch: '+0Hz',
    connect_timeout: 10,
    receive_timeout: 60
  },
  edge_offline: {
    server_url: 'ws://localhost:8080/tts',
    timeout: 30,
    sample_rate: 16000,
    channels: 1,
    frame_duration: 20
  },
  openai: {
    api_key: '',
    api_url: 'https://api.openai.com/v1/audio/speech',
    model: 'tts-1',
    voice: 'alloy',
    response_format: 'mp3',
    speed: 1.0,
    stream: true,
    frame_duration: 60
  },
  xiaozhi: {
    server_addr: 'wss://api.tenclass.net/xiaozhi/v1/',
    device_id: 'ba:8f:17:de:94:94',
    client_id: 'e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d',
    token: 'test-token'
  }
})

const generateConfig = () => {
  const config = {}
  
  switch (form.provider) {
    case 'cosyvoice':
      config.api_url = form.cosyvoice.api_url
      config.spk_id = form.cosyvoice.spk_id
      config.frame_duration = form.cosyvoice.frame_duration
      config.target_sr = form.cosyvoice.target_sr
      config.audio_format = form.cosyvoice.audio_format
      config.instruct_text = form.cosyvoice.instruct_text
      break
    case 'doubao':
      config.appid = form.doubao.appid
      config.access_token = form.doubao.access_token
      config.cluster = form.doubao.cluster
      config.voice = form.doubao.voice
      config.api_url = form.doubao.api_url
      config.authorization = form.doubao.authorization
      break
    case 'doubao_ws':
      config.appid = form.doubao_ws.appid
      config.access_token = form.doubao_ws.access_token
      config.cluster = form.doubao_ws.cluster
      config.voice = form.doubao_ws.voice
      config.ws_host = form.doubao_ws.ws_host
      config.use_stream = form.doubao_ws.use_stream
      break
    case 'edge':
      config.voice = form.edge.voice
      config.rate = form.edge.rate
      config.volume = form.edge.volume
      config.pitch = form.edge.pitch
      config.connect_timeout = form.edge.connect_timeout
      config.receive_timeout = form.edge.receive_timeout
      break
    case 'edge_offline':
      config.server_url = form.edge_offline.server_url
      config.timeout = form.edge_offline.timeout
      config.sample_rate = form.edge_offline.sample_rate
      config.channels = form.edge_offline.channels
      config.frame_duration = form.edge_offline.frame_duration
      break
    case 'openai':
      config.api_key = form.openai.api_key
      config.api_url = form.openai.api_url
      config.model = form.openai.model
      config.voice = form.openai.voice
      config.response_format = form.openai.response_format
      config.speed = form.openai.speed
      config.stream = form.openai.stream
      config.frame_duration = form.openai.frame_duration
      break
    case 'xiaozhi':
      config.server_addr = form.xiaozhi.server_addr
      config.device_id = form.xiaozhi.device_id
      config.client_id = form.xiaozhi.client_id
      config.token = form.xiaozhi.token
      break
  }
  
  return JSON.stringify(config)
}

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  config_id: [{ required: true, message: '请输入配置ID', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  // CosyVoice 验证规则
  'cosyvoice.api_url': [{ required: true, message: '请输入API URL', trigger: 'blur' }],
  'cosyvoice.spk_id': [{ required: true, message: '请输入说话人ID', trigger: 'blur' }],
  // 豆包 TTS 验证规则
  'doubao.appid': [{ required: true, message: '请输入应用ID', trigger: 'blur' }],
  'doubao.access_token': [{ required: true, message: '请输入访问令牌', trigger: 'blur' }],
  'doubao.cluster': [{ required: true, message: '请输入集群', trigger: 'blur' }],
  'doubao.voice': [{ required: true, message: '请输入音色', trigger: 'blur' }],
  'doubao.api_url': [{ required: true, message: '请输入API URL', trigger: 'blur' }],
  // 豆包 WebSocket 验证规则
  'doubao_ws.appid': [{ required: true, message: '请输入应用ID', trigger: 'blur' }],
  'doubao_ws.access_token': [{ required: true, message: '请输入访问令牌', trigger: 'blur' }],
  'doubao_ws.cluster': [{ required: true, message: '请输入集群', trigger: 'blur' }],
  'doubao_ws.voice': [{ required: true, message: '请输入音色', trigger: 'blur' }],
  'doubao_ws.ws_host': [{ required: true, message: '请输入WebSocket主机', trigger: 'blur' }],
  // Edge TTS 验证规则
  'edge.voice': [{ required: true, message: '请输入音色', trigger: 'blur' }],
  'edge.rate': [{ required: true, message: '请输入语速', trigger: 'blur' }],
  'edge.volume': [{ required: true, message: '请输入音量', trigger: 'blur' }],
  // Edge 离线验证规则
  'edge_offline.server_url': [{ required: true, message: '请输入服务器URL', trigger: 'blur' }],
  // OpenAI TTS 验证规则
  'openai.api_key': [{ required: true, message: '请输入API Key', trigger: 'blur' }],
  // 小智 TTS 验证规则
  'xiaozhi.server_addr': [{ required: true, message: '请输入服务器地址', trigger: 'blur' }],
  'xiaozhi.device_id': [{ required: true, message: '请输入设备ID', trigger: 'blur' }],
  'xiaozhi.client_id': [{ required: true, message: '请输入客户端ID', trigger: 'blur' }],
  'xiaozhi.token': [{ required: true, message: '请输入令牌', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/tts-configs')
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
  
  // 解析配置JSON并填充到对应的表单字段
  try {
    const configData = JSON.parse(config.json_data || '{}')
    
    switch (config.provider) {
      case 'cosyvoice':
        form.cosyvoice.api_url = configData.api_url || ''
        form.cosyvoice.spk_id = configData.spk_id || ''
        form.cosyvoice.frame_duration = configData.frame_duration || 60
        form.cosyvoice.target_sr = configData.target_sr || 24000
        form.cosyvoice.audio_format = configData.audio_format || 'mp3'
        form.cosyvoice.instruct_text = configData.instruct_text || ''
        break
      case 'doubao':
        form.doubao.appid = configData.appid || ''
        form.doubao.access_token = configData.access_token || ''
        form.doubao.cluster = configData.cluster || ''
        form.doubao.voice = configData.voice || ''
        form.doubao.api_url = configData.api_url || ''
        form.doubao.authorization = configData.authorization || ''
        break
      case 'doubao_ws':
        form.doubao_ws.appid = configData.appid || ''
        form.doubao_ws.access_token = configData.access_token || ''
        form.doubao_ws.cluster = configData.cluster || ''
        form.doubao_ws.voice = configData.voice || ''
        form.doubao_ws.ws_host = configData.ws_host || ''
        form.doubao_ws.use_stream = configData.use_stream !== undefined ? configData.use_stream : true
        break
      case 'edge':
        form.edge.voice = configData.voice || ''
        form.edge.rate = configData.rate || '+0%'
        form.edge.volume = configData.volume || '+0%'
        form.edge.pitch = configData.pitch || '+0Hz'
        form.edge.connect_timeout = configData.connect_timeout || 10
        form.edge.receive_timeout = configData.receive_timeout || 60
        break
      case 'edge_offline':
        form.edge_offline.server_url = configData.server_url || ''
        form.edge_offline.timeout = configData.timeout || 30
        form.edge_offline.sample_rate = configData.sample_rate || 16000
        form.edge_offline.channels = configData.channels || 1
        form.edge_offline.frame_duration = configData.frame_duration || 20
        break
      case 'openai':
        form.openai.api_key = configData.api_key || ''
        form.openai.api_url = configData.api_url || 'https://api.openai.com/v1/audio/speech'
        form.openai.model = configData.model || 'tts-1'
        form.openai.voice = configData.voice || 'alloy'
        form.openai.response_format = configData.response_format || 'mp3'
        form.openai.speed = configData.speed || 1.0
        form.openai.stream = configData.stream !== undefined ? configData.stream : true
        form.openai.frame_duration = configData.frame_duration || 60
        break
      case 'xiaozhi':
        form.xiaozhi.server_addr = configData.server_addr || ''
        form.xiaozhi.device_id = configData.device_id || ''
        form.xiaozhi.client_id = configData.client_id || ''
        form.xiaozhi.token = configData.token || ''
        break
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
          await api.put(`/admin/tts-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('配置更新成功')
        } else {
          await api.post('/admin/tts-configs', configData)
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
    
    await api.put(`/admin/tts-configs/${config.id}`, configData)
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
    
    await api.delete(`/admin/tts-configs/${id}`)
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
    provider: 'xiaozhi',
    is_default: false,
    enabled: true,
    cosyvoice: {
      api_url: 'https://tts.linkerai.top/tts',
      spk_id: 'spk_id',
      frame_duration: 60,
      target_sr: 24000,
      audio_format: 'mp3',
      instruct_text: '你好'
    },
    doubao: {
      appid: '6886011847',
      access_token: 'access_token',
      cluster: 'volcano_tts',
      voice: 'BV001_streaming',
      api_url: 'https://openspeech.bytedance.com/api/v1/tts',
      authorization: 'Bearer;'
    },
    doubao_ws: {
      appid: '6886011847',
      access_token: 'access_token',
      cluster: 'volcano_tts',
      voice: 'zh_female_wanwanxiaohe_moon_bigtts',
      ws_host: 'openspeech.bytedance.com',
      use_stream: true
    },
    edge: {
      voice: 'zh-CN-XiaoxiaoNeural',
      rate: '+0%',
      volume: '+0%',
      pitch: '+0Hz',
      connect_timeout: 10,
      receive_timeout: 60
    },
    edge_offline: {
      server_url: 'ws://localhost:8080/tts',
      timeout: 30,
      sample_rate: 16000,
      channels: 1,
      frame_duration: 20
    },
    openai: {
      api_key: '',
      api_url: 'https://api.openai.com/v1/audio/speech',
      model: 'tts-1',
      voice: 'alloy',
      response_format: 'mp3',
      speed: 1.0,
      stream: true,
      frame_duration: 60
    },
    xiaozhi: {
      server_addr: 'wss://api.tenclass.net/xiaozhi/v1/',
      device_id: 'ba:8f:17:de:94:94',
      client_id: 'e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d',
      token: 'test-token'
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