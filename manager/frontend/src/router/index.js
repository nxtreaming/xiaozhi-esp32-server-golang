import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('../views/Setup.vue')
  },
  {
    path: '/test',
    name: 'Test',
    component: () => import('../views/Test.vue')
  },
  {
    path: '/test-route',
    name: 'TestRoute',
    component: () => import('../views/TestRoute.vue')
  },
  {
    path: '/simple-login',
    name: 'SimpleLogin',
    component: () => import('../views/SimpleLogin.vue')
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/',
    name: 'Layout',
    component: () => import('../components/Layout.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: '/dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: '仪表板' }
      },
      // 管理员路由
      {
        path: '/admin',
        name: 'Admin',
        meta: { requiresAuth: true, requiresAdmin: true },
        children: [
          {
            path: 'vad-config',
            name: 'VADConfig',
            component: () => import('../views/admin/VADConfig.vue'),
            meta: { title: 'VAD配置管理' }
          },
          {
            path: 'asr-config',
            name: 'ASRConfig',
            component: () => import('../views/admin/ASRConfig.vue'),
            meta: { title: 'ASR配置管理' }
          },
          {
            path: 'llm-config',
            name: 'LLMConfig',
            component: () => import('../views/admin/LLMConfig.vue'),
            meta: { title: 'LLM配置管理' }
          },
          {
            path: 'tts-config',
            name: 'TTSConfig',
            component: () => import('../views/admin/TTSConfig.vue'),
            meta: { title: 'TTS配置管理' }
          },
          {
            path: 'ota-config',
            name: 'OTAConfig',
            component: () => import('../views/admin/OTAConfig.vue'),
            meta: { title: 'OTA配置管理' }
          },
          {
            path: 'mqtt-config',
            name: 'MQTTConfig',
            component: () => import('../views/admin/MQTTConfig.vue'),
            meta: { title: 'MQTT配置管理' }
          },
          {
            path: 'udp-config',
            name: 'UDPConfig',
            component: () => import('../views/admin/UDPConfig.vue'),
            meta: { title: 'UDP配置管理' }
          },
          {
            path: 'mqtt-server-config',
            name: 'MQTTServerConfig',
            component: () => import('../views/admin/MQTTServerConfig.vue'),
            meta: { title: 'MQTT Server配置管理' }
          },
          {
            path: 'mcp-config',
            name: 'MCPConfig',
            component: () => import('../views/admin/MCPConfig.vue'),
            meta: { title: 'MCP配置管理' }
          },
          {
            path: 'memory-config',
            name: 'MemoryConfig',
            component: () => import('../views/admin/MemoryConfig.vue'),
            meta: { title: 'Memory配置管理' }
          },
          		{
			path: 'vision-config',
			name: 'VisionConfig',
			component: () => import('../views/admin/VisionConfig.vue'),
			meta: { title: 'Vision配置管理' }
		},
          {
            path: 'global-roles',
            name: 'GlobalRoles',
            component: () => import('../views/admin/GlobalRoles.vue'),
            meta: { title: '全局角色管理' }
          },
          {
            path: 'users',
            name: 'Users',
            component: () => import('../views/admin/Users.vue'),
            meta: { title: '用户管理' }
          },
          {
            path: 'devices',
            name: 'AdminDevices',
            component: () => import('../views/admin/Devices.vue'),
            meta: { title: '设备管理' }
          },
          {
            path: 'agents',
            name: 'AdminAgents',
            component: () => import('../views/admin/Agents.vue'),
            meta: { title: '智能体管理' }
          }
        ]
      },
      // 用户路由
      {
        path: '/console',
        name: 'UserConsole',
        component: () => import('../views/user/UserConsole.vue'),
        meta: { title: '用户控制台' }
      },
      {
        path: '/agents',
        name: 'Agents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: '我的智能体' }
      },
      {
        path: '/user/agents',
        name: 'UserAgents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: '我的智能体' }
      },
      {
        path: '/agents/:id/edit',
        name: 'AgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: '编辑智能体' }
      },
      {
        path: '/user/agents/:id/edit',
        name: 'UserAgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: '编辑智能体' }
      },
      {
        path: '/user/agents/:id/devices',
        name: 'AgentDevices',
        component: () => import('../views/user/AgentDevices.vue'),
        meta: { title: '智能体设备管理' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  
  // 如果访问引导页面，直接通过
  if (to.path === '/setup') {
    next()
    return
  }
  
  // 如果访问登录页且已登录，根据角色跳转
  if (to.path === '/login' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      next('/dashboard')
    } else {
      next('/console')
    }
    return
  }
  
  // 如果需要认证
  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      // 没有token，跳转到登录页
      next('/login')
      return
    }
    
    // 有token但没有用户信息，验证token有效性
    if (!authStore.user && !authStore.isValidating) {
      try {
        await authStore.getProfile()
      } catch (error) {
        // token无效，跳转到登录页
        next('/login')
        return
      }
    }
  }
  
  // 如果访问根路径，根据角色跳转
  if (to.path === '/' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      next('/dashboard')
    } else {
      next('/console')
    }
    return
  }
  
  // 如果普通用户访问管理员页面，跳转到用户控制台
  if (to.meta.requiresAdmin && authStore.user?.role !== 'admin') {
    next('/console')
    return
  }
  
  next()
})

export default router