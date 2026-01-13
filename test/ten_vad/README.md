# TEN-VAD 测试程序

这是一个用于测试和验证 TEN-VAD（语音活动检测）功能的测试程序。

## 功能

- 读取 WAV 音频文件
- 将 WAV 转换为 PCM 格式（float32）
- 使用 TEN-VAD 对每一帧进行语音活动检测
- 输出详细的检测结果和统计信息

## 使用方法

### 基本用法

```bash
# 使用默认参数（hop_size=512, threshold=0.3）
go run main.go audio_utils.go <wav文件路径>

# 示例
go run main.go audio_utils.go test.wav
```

### 自定义参数

```bash
# 指定 hop_size 和 threshold
go run main.go audio_utils.go <wav文件路径> <hop_size> <threshold>

# 示例：使用 hop_size=256, threshold=0.5
go run main.go audio_utils.go test.wav 256 0.5
```

### 编译后使用

```bash
# 编译
go build -o ten_vad_test main.go audio_utils.go

# 运行
./ten_vad_test test.wav
./ten_vad_test test.wav 512 0.3
```

## 参数说明

- **wav文件路径**（必需）：要测试的 WAV 音频文件路径
- **hop_size**（可选，默认512）：帧移大小，TEN-VAD 处理音频时的帧大小
- **threshold**（可选，默认0.3）：VAD 检测阈值，范围 0.0-1.0，值越大越严格

## 输出信息

程序会输出以下信息：

1. **文件信息**：WAV 文件大小、格式信息
2. **转换信息**：PCM 数据帧数
3. **检测过程**：每一帧的检测结果（是否有语音）
4. **统计结果**：
   - 总帧数
   - 语音帧数
   - 语音活动比例（%）
   - 最终结论

## 示例输出

```
成功读取WAV文件: test.wav (123456 字节)
WAV格式: {SampleRate:16000 NumChannels:1 BitDepth:16}
开始转换...
成功转换为PCM数据，共 100 帧
TEN-VAD创建成功 (hop_size=512, threshold=0.30)，开始测试...
开始进行语音活动检测...
第1帧: 无语音活动
第2帧: 检测到语音活动
...
第100帧: 无语音活动

=== TEN-VAD检测结果统计 ===
总帧数: 100
语音帧数: 45
语音活动比例: 45.00%
结论: 检测到语音活动
```

## WAV 文件格式要求

### 推荐格式（最佳兼容性）
- **采样率**：16000 Hz（推荐）或 8000/32000/48000 Hz
- **声道数**：单声道（Mono，1 channel）
- **位深度**：16 bit
- **编码格式**：PCM（未压缩）
- **文件格式**：标准 WAV 文件（.wav）

### 支持的格式
程序使用 `go-audio/wav` 库，支持大多数标准 WAV 格式：
- 不同采样率（程序会自动处理，但建议 16000Hz）
- 单声道或多声道（多声道会自动处理）
- 不同位深度（8/16/24/32 bit）

### 格式转换示例

如果您的音频文件不是推荐格式，可以使用以下工具转换：

**使用 ffmpeg 转换：**
```bash
# 转换为 16000Hz, 单声道, 16bit PCM WAV
ffmpeg -i input.wav -ar 16000 -ac 1 -sample_fmt s16 output.wav

# 从 MP3 转换
ffmpeg -i input.mp3 -ar 16000 -ac 1 -sample_fmt s16 output.wav

# 从其他格式转换
ffmpeg -i input.m4a -ar 16000 -ac 1 -sample_fmt s16 output.wav
```

**使用 sox 转换：**
```bash
sox input.wav -r 16000 -c 1 -b 16 output.wav
```

## 注意事项

1. **库文件要求**：确保 `lib/ten-vad` 目录下有正确的动态库文件：
   - Windows: `lib/ten-vad/lib/Windows/x64/ten_vad.dll` 和 `ten_vad.lib`
   - Linux: `lib/ten-vad/lib/Linux/x64/libten_vad.so`

2. **音频格式要求**：
   - 采样率：建议 16000Hz（程序会自动转换，但推荐使用标准格式）
   - 声道：单声道或多声道都可以（程序会自动处理）
   - 格式：标准 WAV 文件（PCM 编码）

3. **CGO 编译**：需要 C 编译器和正确的 CGO 环境

4. **文件大小**：没有严格限制，但建议测试文件不要太长（几秒到几分钟）

## 故障排除

如果遇到 "VAD初始化失败" 错误：

1. 检查 `lib/ten-vad` 目录是否存在且包含正确的库文件
2. 检查 CGO 环境是否正确配置
3. 在 Windows 上确保 DLL 文件在系统路径或程序目录中
4. 在 Linux 上确保 `.so` 文件在库路径中
