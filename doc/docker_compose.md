# Docker Compose 部署指南

## 概述

本项目使用 Docker Compose 进行容器化部署，包含以下核心服务：

- **MySQL 数据库服务** - 数据存储
- **主程序服务** - 核心业务逻辑
- **后端管理服务** - API 接口服务
- **前端管理服务** - Web 管理界面

## 服务架构

### 1. MySQL 数据库服务 (xiaozhi-mysql)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/mysql:8.0`
- 端口映射：`23306:3306`
- 数据库名：`xiaozhi_admin`
- 用户名：`root`
- 密码：`password`

**特性：**
- 使用 MySQL 8.0 版本
- 配置了健康检查机制
- 数据持久化存储
- 支持 MySQL 原生密码认证

### 2. 主程序服务 (xiaozhi-main-server)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_server:0.5`
- 端口映射：
  - `8989:8989` - WebSocket 服务端口
  - `2882:2883` - MQTT 服务端口
  - `8888:8888/udp` - UDP 服务端口

**依赖关系：**
- 依赖 MySQL 服务健康状态
- 依赖后端服务启动完成

**环境变量：**
- 数据库连接配置
- 后端服务地址配置

**配置文件支持：**
- 支持通过卷挂载导入自定义配置文件
- 配置文件路径：`../../config:/workspace/config`

**ten_vad 支持：**
- Docker 镜像已自动包含 ten_vad 库文件（位于 `/workspace/lib/ten-vad/`）
- 运行时库路径已自动配置（`LD_LIBRARY_PATH`）
- 如果使用 ten_vad 作为 VAD 提供商，无需额外配置

### 3. 后端管理服务 (xiaozhi-backend)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_manager_backend:0.5`
- 端口映射：`8081:8080`

**依赖关系：**
- 依赖 MySQL 服务健康状态

**功能：**
- 提供 RESTful API 接口
- 设备管理功能
- 用户管理功能

**配置文件支持：**
- 支持通过卷挂载导入自定义配置文件
- 配置文件路径：`../../manager/backend/config:/root/config`

### 4. 前端管理服务 (xiaozhi-frontend)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_manager_frontend:0.5`
- 端口映射：`8080:80`

**依赖关系：**
- 依赖后端服务

**功能：**
- Web 管理界面（内控入口）
- 设备状态监控
- 系统配置管理

## 部署流程

### 1. 环境准备

确保系统已安装 Docker 和 Docker Compose：

```bash
# 检查 Docker 版本
docker --version

# 检查 Docker Compose 版本
docker-compose --version
```

### 2. 配置文件准备

确保以下目录和文件存在：

```
xiaozhi-esp32-server-golang/
├── docker/docker-composer/
│   └── docker-compose.yml
├── config/
│   ├── config.yaml          # 主程序配置文件
│   ├── config.json          # 其他配置文件
│   └── (其他配置文件)
├── logs/
│   └── (日志目录)
└── manager/backend/config/
    ├── config.yaml          # 后端配置文件
    └── (其他后端配置)
```

**配置文件导入说明：**
- 主程序配置文件通过卷挂载 `../../config:/workspace/config` 导入
- 后端配置文件通过卷挂载 `../../manager/backend/config:/root/config` 导入
- 修改宿主机配置文件后，重启对应服务即可生效

### 3. 启动服务

**重要：必须先进入 `docker/docker-composer/` 目录再执行命令**

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看服务日志
docker-compose logs -f
```

### 4. 服务访问

启动成功后，可通过以下地址访问各服务：

- **前端管理界面（内控入口）**：http://localhost:8080
- **后端 API**：http://localhost:8081
- **WebSocket 服务**：ws://localhost:8989
- **MQTT 服务**：localhost:2882
- **UDP 服务**：localhost:8888
- **MySQL 数据库**：localhost:23306

## 常用操作

### 查看服务状态

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 查看所有服务状态
docker-compose ps

# 查看特定服务状态
docker-compose ps main-server
```

### 查看服务日志

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 查看所有服务日志
docker-compose logs

# 查看特定服务日志
docker-compose logs main-server

# 实时查看日志
docker-compose logs -f main-server
```

### 重启服务

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 重启所有服务
docker-compose restart

# 重启特定服务
docker-compose restart main-server
```

### 停止服务

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 停止所有服务
docker-compose down

# 停止并删除数据卷
docker-compose down -v
```

### 更新服务

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 拉取最新镜像并重启服务
docker-compose pull
docker-compose up -d
```

## 网络配置

项目使用自定义网络 `xiaozhi-network`，所有服务都在同一网络下，可以通过服务名进行内部通信：

- MySQL 服务：`mysql:3306`
- 后端服务：`backend:8080`
- 前端服务：`frontend:80`
- 主程序服务：`main-server:8989` (WebSocket)、`main-server:2883` (MQTT)、`main-server:8888` (UDP)

**端口映射总结：**
- 宿主机 8080 → 前端管理界面（内控入口）
- 宿主机 8081 → 后端 API 服务
- 宿主机 8989 → WebSocket 服务
- 宿主机 2882 → MQTT 服务
- 宿主机 8888 → UDP 服务
- 宿主机 23306 → MySQL 数据库

## 数据持久化

### MySQL 数据

MySQL 数据通过 Docker 卷 `mysql_data` 进行持久化存储，数据不会因容器重启而丢失。

### 配置文件

主程序和后端服务的配置文件通过卷挂载方式映射到容器内：

- 主程序配置：`../../config:/workspace/config`
  - 支持 `config.yaml`、`config.json` 等配置文件
  - 修改宿主机配置文件后重启服务即可生效
- 后端配置：`../../manager/backend/config:/root/config`
  - 支持 `config.yaml` 等配置文件
  - 修改宿主机配置文件后重启服务即可生效

## 配置文件导入方法

### 1. 主程序配置文件

**配置文件位置：**
```
xiaozhi-esp32-server-golang/config/
├── config.yaml          # 主配置文件
├── config.json          # JSON格式配置
├── mqtt_config.json     # MQTT配置
└── (其他配置文件)
```

**导入方法：**
1. 将配置文件放置在 `config/` 目录下
2. 启动服务时自动挂载到容器内 `/workspace/config/` 目录
3. 修改配置文件后重启主程序服务：
   ```bash
   cd docker/docker-composer/
   docker-compose restart main-server
   ```

### 2. 后端管理配置文件

**配置文件位置：**
```
xiaozhi-esp32-server-golang/manager/backend/config/
├── config.yaml          # 后端配置文件
└── (其他配置文件)
```

**导入方法：**
1. 将配置文件放置在 `manager/backend/config/` 目录下
2. 启动服务时自动挂载到容器内 `/root/config/` 目录
3. 修改配置文件后重启后端服务：
   ```bash
   cd docker/docker-composer/
   docker-compose restart backend
   ```

### 3. 配置文件热更新

**注意事项：**
- 某些配置修改后需要重启服务才能生效
- 建议在修改配置文件前先备份原文件
- 可以通过查看服务日志确认配置是否正确加载：
  ```bash
  cd docker/docker-composer/
  查看主程序日志：docker-compose logs main-server
  查看后端日志：docker-compose logs backend
  ```

### 日志文件

日志文件通过卷挂载方式映射到容器内：

- 主程序日志：`../../logs:/workspace/logs`

### 4. ten_vad 库文件

**说明：**
- ten_vad 库文件已包含在 Docker 镜像中，位于 `/workspace/lib/ten-vad/`
- 运行时库路径已通过环境变量 `LD_LIBRARY_PATH` 自动配置
- 如果使用 ten_vad 作为 VAD 提供商，无需额外挂载或配置

**本地编译注意事项：**
- 如果需要在本地编译时使用 ten_vad，请确保 `lib/ten-vad` 目录存在
- 编译时需要设置 CGO 环境变量（详见 README.md）

## 健康检查

MySQL 服务配置了健康检查机制：

```yaml
healthcheck:
  test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-ppassword"]
  timeout: 20s
  retries: 10
  interval: 10s
  start_period: 30s
```

其他服务依赖 MySQL 的健康状态，确保数据库完全启动后才启动相关服务。

## 故障排除

### 1. 服务启动失败

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 查看详细错误信息
docker-compose logs [服务名]

# 检查端口占用
netstat -tulpn | grep [端口号]
```

### 2. 数据库连接失败

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 检查 MySQL 服务状态
docker-compose ps mysql

# 查看 MySQL 日志
docker-compose logs mysql

# 进入 MySQL 容器检查
docker-compose exec mysql mysql -u root -ppassword
```

### 3. 网络连接问题

```bash
# 进入 docker-compose 目录
cd docker/docker-composer/

# 检查网络配置
docker network ls
docker network inspect xiaozhi-network

# 测试服务间通信
docker-compose exec main-server ping mysql
```

## 性能优化建议

1. **资源限制**：在生产环境中，建议为各服务设置资源限制
2. **日志轮转**：配置日志轮转避免日志文件过大
3. **备份策略**：定期备份 MySQL 数据
4. **监控**：集成监控系统监控服务状态

## 安全注意事项

1. **密码安全**：生产环境中请修改默认密码
2. **端口暴露**：根据实际需要调整端口映射
3. **网络安全**：配置防火墙规则
4. **镜像安全**：使用官方或可信的镜像源
