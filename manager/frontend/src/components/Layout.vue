<template>
  <el-container class="layout-container">
    <el-aside width="250px" class="sidebar">
      <div class="logo">
        <h3>小智管理系统</h3>
      </div>
      <el-menu
        :default-active="$route.path"
        class="sidebar-menu"
        router
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409EFF"
      >
        <el-menu-item index="/dashboard">
          <el-icon><House /></el-icon>
          <span>仪表板</span>
        </el-menu-item>
        
        <el-menu-item v-if="!authStore.isAdmin" index="/console">
          <el-icon><Monitor /></el-icon>
          <span>用户控制台</span>
        </el-menu-item>
        
        <el-menu-item v-if="!authStore.isAdmin" index="/agents">
          <el-icon><Monitor /></el-icon>
          <span>智能体管理</span>
        </el-menu-item>
        
        <!-- 服务配置 -->
        <el-sub-menu v-if="authStore.isAdmin" index="/admin/service-config">
          <template #title>
            <el-icon><Tools /></el-icon>
            <span>服务配置</span>
          </template>
          <el-menu-item index="/admin/ota-config">OTA配置</el-menu-item>
          <el-menu-item index="/admin/mqtt-config">MQTT配置</el-menu-item>
          <el-menu-item index="/admin/mqtt-server-config">MQTT Server配置</el-menu-item>
          <el-menu-item index="/admin/udp-config">UDP配置</el-menu-item>
          <el-menu-item index="/admin/mcp-config">MCP配置</el-menu-item>
        </el-sub-menu>
        
        <!-- AI配置 -->
        <el-sub-menu v-if="authStore.isAdmin" index="/admin/ai-config">
          <template #title>
            <el-icon><Cpu /></el-icon>
            <span>AI配置</span>
          </template>
          <el-menu-item index="/admin/vad-config">VAD配置</el-menu-item>
          <el-menu-item index="/admin/asr-config">ASR配置</el-menu-item>
          <el-menu-item index="/admin/llm-config">LLM配置</el-menu-item>
          <el-menu-item index="/admin/tts-config">TTS配置</el-menu-item>
          <el-menu-item index="/admin/vision-config">Vision配置</el-menu-item>
          <el-menu-item index="/admin/memory-config">Memory配置</el-menu-item>
        </el-sub-menu>
        
        <!-- 系统管理 -->
        <el-menu-item v-if="authStore.isAdmin" index="/admin/global-roles">
          <el-icon><Setting /></el-icon>
          <span>全局角色</span>
        </el-menu-item>
        
        <el-menu-item v-if="authStore.isAdmin" index="/admin/users">
          <el-icon><UserFilled /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
        
        <el-menu-item v-if="authStore.isAdmin" index="/admin/devices">
          <el-icon><Iphone /></el-icon>
          <span>设备管理</span>
        </el-menu-item>
        
        <el-menu-item v-if="authStore.isAdmin" index="/admin/agents">
          <el-icon><Connection /></el-icon>
          <span>智能体管理</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <span class="page-title">{{ currentPageTitle }}</span>
        </div>
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-icon><User /></el-icon>
              {{ authStore.user?.username }}
              <el-icon class="el-icon--right"><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import {
  House,
  Monitor,
  Setting,
  User,
  ArrowDown,
  Tools,
  Cpu,
  UserFilled,
  Iphone,
  Connection
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const currentPageTitle = computed(() => {
  return route.meta?.title || '仪表板'
})

const handleCommand = async (command) => {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      
      authStore.logout()
      ElMessage.success('已退出登录')
      router.push('/login')
    } catch {
      // 用户取消
    }
  }
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.sidebar {
  background-color: #304156;
  overflow: hidden;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #2b3a4b;
  color: white;
  margin-bottom: 0;
}

.logo h3 {
  margin: 0;
  font-size: 16px;
}

.sidebar-menu {
  border: none;
  height: calc(100vh - 60px);
  overflow-y: auto;
}

.header {
  background-color: #fff;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.header-left .page-title {
  font-size: 18px;
  font-weight: 500;
  color: #333;
}

.header-right .user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  color: #666;
}

.header-right .user-info:hover {
  color: #409EFF;
}

.main-content {
  background-color: #f5f5f5;
  padding: 20px;
}
</style>