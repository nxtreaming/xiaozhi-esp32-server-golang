# 一键启动包部署教程

## 下载

访问 [Release 页面](https://github.com/hackers365/xiaozhi-esp32-server-golang/releases) 下载对应平台：

| 平台 | 文件名 |
|-----|-------|
| Windows | `xiaozhi-server-windows-xxx.zip` |
| Linux | `xiaozhi-server-linux-xxx.tar.gz` |
| macOS | `xiaozhi-server-macos-xxx.tar.gz` |

---

## 解压与目录结构

解压后目录结构：

```
xiaozhi-aio/
├── xiaozhi_server          # 主程序
├── config/                 # 配置文件目录
├── models/                 # 模型文件目录（如使用本地ASR/TTS）
└── data/                   # 数据目录
```

---

## 启动服务

### Windows
双击 `start.bat`

### Linux
```bash
chmod +x xiaozhi_server
LD_LIBRARY_PATH="$PWD/ten-vad/lib/Linux/x64:${LD_LIBRARY_PATH:-}" ./xiaozhi_server
```

### macOS
```bash
chmod +x xiaozhi_server
DYLD_FRAMEWORK_PATH="$PWD/ten-vad/lib/macOS" ./xiaozhi_server
```

---

## 下一步

### 1. 访问Web控制台

浏览器访问：**http://<服务器IP或域名>:8080**

<!-- 截图位置：登录界面 -->
> 图：Web控制台登录界面

### 2. 配置服务

首次使用请按照配置向导完成设置，详见：

**[管理后台使用指南 →](manager_console_guide.md)**

---

## 声纹识别服务（可选）

程序中已集成声纹服务

---

## 常见问题

### Q1: 启动后无法访问Web控制台？

检查防火墙设置，确保8080端口可访问。

### Q2: 如何重启服务？

关闭程序后重新运行即可。配置文件保存在 `config/` 目录。

### Q3: 如何查看日志？

控制台输出实时日志，如需保存可重定向：

```bash
./xiaozhi_server > server.log 2>&1
```
