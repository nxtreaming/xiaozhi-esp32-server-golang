
# 运行环境

#### 一. 部署funasr

参见 [funasr docker部署文档](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md)

#### 二. 克隆代码
>git clone 'https://github.com/hackers365/xiaozhi-esp32-server-golang'

#### 三. 配置config/config.yaml，详细参见 [config配置说明](config.md)

主要修改项如下：
```yaml
# 1. asr语音识别
asr:
  provider: "funasr"
  funasr:
    host: "127.0.0.1"      # 部署的funasr websocket服务的ip
    port: "10096"          # 部署的funasr websocket的port
    mode: "offline"        # 模式, 使用offline即可
    # ...

# 2. tts
tts:
  provider: "xiaozhi"      # 使用tts的类型, 建议doubao_ws, 也可以选择免费的edge
  doubao_ws:
    appid: "6886011847"                         # 你的appid
    access_token: "access_token"                # 你的access token
    cluster: "volcano_tts"
    voice: "zh_female_wanwanxiaohe_moon_bigtts" # 音色，默认是湾湾小何
    ws_host: "openspeech.bytedance.com"
    use_stream: true
  edge:
    voice: "zh-CN-XiaoxiaoNeural"
    rate: "+0%"
    volume: "+0%"
    pitch: "+0Hz"
    connect_timeout: 10
    receive_timeout: 60
  # ....

# 3. llm 大模型
llm:
  provider: "deepseek"                        # 提供商，对应下面的key
  deepseek:
    type: "openai"                            # 服务端接口兼容的类型
    model_name: "Pro/deepseek-ai/DeepSeek-V3" # 模型名称
    api_key: "api_key"                        # api key
    base_url: "https://api.siliconflow.cn/v1" # 服务接口，默认硅基流动
    max_tokens: 500
  # ...

```

#### 四. 启动docker
在项目根目录 启动docker并挂载config目录和端口(http/websocket:8989, 其它端口按需映射)

```
docker run -itd --name xiaozhi_server -v $(pwd)/config:/workspace/config -p 8989:8989 hackers365/xiaozhi_server:latest

国内连不上的话，使用如下源

docker run -itd --name xiaozhi_server -v $(pwd)/config:/workspace/config -p 8989:8989 docker.jsdelivr.fyi/hackers365/xiaozhi_server:latest
```

**ten_vad 支持说明：**
- Docker 镜像已自动包含 ten_vad 库文件，无需额外挂载
- 如果使用 ten_vad 作为 VAD 提供商，在配置文件中设置 `vad.provider: "ten_vad"` 即可

现在应该可以连上 
>ws://机器ip:8989/xiaozhi/v1/ 

进行聊天了


# 开发环境
```
docker run -itd --name xiaozhi_server_golang -v $(pwd):/workspace/ -p 8989:8989 hackers365/xiaozhi_golang:0.1
国内连不上的话，使用如下源
docker run -itd --name xiaozhi_server_golang -v $(pwd):/workspace/ -p 8989:8989 docker.jsdelivr.fyi/hackers365/xiaozhi_golang:0.1

go build -o xiaozhi_server cmd/server/*.go
```

**开发环境 ten_vad 说明：**
- 开发环境镜像已包含 ten_vad 编译和运行时依赖
- 如果需要在开发环境中使用 ten_vad，确保项目根目录的 `lib/ten-vad` 目录存在
- 编译时会自动使用 ten_vad 的头文件和库文件
