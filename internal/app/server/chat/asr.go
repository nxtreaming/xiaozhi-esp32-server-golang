package chat

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/asr"
	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/domain/vad/inter"
	"xiaozhi-esp32-server-golang/internal/pool"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

type ASRManagerOption func(*ASRManager)

// AsrMessageSaveCallback 消息保存回调函数类型
type AsrMessageSaveCallback func(userMsg *schema.Message, messageID string, audioData []float32)

type ASRManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	session         *ChatSession // 用于访问 speakerManager

	// ASR 资源作为私有字段管理
	asrResource *pool.ResourceWrapper[asr.AsrProvider]
	resourceMu  sync.RWMutex // 保护资源访问
}

func NewASRManager(clientState *ClientState, serverTransport *ServerTransport, opts ...ASRManagerOption) *ASRManager {
	asr := &ASRManager{
		clientState:     clientState,
		serverTransport: serverTransport,
		session:         nil, // 稍后通过 SetSession 设置
	}
	for _, opt := range opts {
		opt(asr)
	}
	return asr
}

// ProcessVadAudio 启动VAD音频处理
func (a *ASRManager) ProcessVadAudio(ctx context.Context, onClose func()) {
	state := a.clientState
	go func() {
		hasTriggeredCancel := true // 标志位，记录是否已触发过取消操作（当 voiceDuration > 120 时）
		audioFormat := state.InputAudioFormat
		// 使用一个足够大的缓冲区用于解码（假设最大帧时长为120ms）
		maxFrameSize := audioFormat.SampleRate * audioFormat.Channels * 120 / 1000
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, 20) // 传入一个默认值用于创建解码器
		if err != nil {
			log.Errorf("获取解码器失败: %v", err)
			return
		}

		// 从第一帧实际数据中获取帧大小和帧时长
		var frameSize int
		var frameDurationMs int
		var vadNeedGetCount int // VAD需要的帧数，会在第一帧后计算

		// 在循环外获取VAD资源（高频使用场景，避免频繁获取/归还）
		var vadWrapper *pool.ResourceWrapper[inter.VAD]
		var vadProvider inter.VAD
		needVad := !(state.Asr.AutoEnd || state.ListenMode == "manual")
		if needVad {
			var err error
			provider := state.DeviceConfig.Vad.Provider
			config := state.DeviceConfig.Vad.Config

			// 检查 provider 是否为空，如果为空则记录警告
			if provider == "" {
				log.Warnf("VAD provider 为空，尝试从 config 中获取")
			} else {
				log.Debugf("获取VAD资源: provider=%s", provider)
			}

			vadWrapper, err = pool.Acquire[inter.VAD](
				"vad",
				provider,
				config,
			)
			if err != nil {
				log.Errorf("获取VAD资源失败: provider=%s, config=%+v, error=%v", provider, config, err)
				return
			}
			defer pool.Release(vadWrapper) // 函数返回时归还资源
			vadProvider = vadWrapper.GetProvider()
		}

		for {
			// 使用最大帧大小作为缓冲区，解码后会得到实际帧大小
			pcmFrame := make([]float32, maxFrameSize)

			select {
			case opusFrame, ok := <-state.OpusAudioBuffer:
				//log.Debugf("processAsrAudio 收到音频数据, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio 音频通道已关闭")
					return
				}

				var skipVad bool
				var haveVoice bool
				clientHaveVoice := state.GetClientHaveVoice()
				if state.Asr.AutoEnd || state.ListenMode == "manual" {
					skipVad = true         //跳过vad
					clientHaveVoice = true //之前有声音
					haveVoice = true       //本次有声音
				}

				if state.GetClientVoiceStop() { //已停止 说话 则不接收音频数据
					//log.Infof("客户端停止说话, 跳过音频数据")
					continue
				}

				//log.Debugf("clientVoiceStop: %+v, asrDataSize: %d, listenMode: %s, isSkipVad: %v\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize(), state.ListenMode, skipVad)

				n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
				if err != nil {
					log.Errorf("解码失败: %v", err)
					continue
				}

				// 从实际解码后的数据动态计算帧大小和帧时长
				if frameSize == 0 {
					// 第一帧：从实际解码的数据计算帧信息
					frameSize = n
					samplesPerChannel := n / audioFormat.Channels
					frameDurationMs = samplesPerChannel * 1000 / audioFormat.SampleRate
					audioFormat.FrameDuration = frameDurationMs

					// 计算 VAD 需要的帧数
					vadNeedGetCount = 1
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						// silero_vad 需要至少 60ms 的音频数据
						vadNeedGetCount = 60 / frameDurationMs
						if vadNeedGetCount < 1 {
							vadNeedGetCount = 1
						}
					}
					log.Debugf("从实际音频数据计算帧信息: frameSize=%d, frameDurationMs=%d, vadNeedGetCount=%d", frameSize, frameDurationMs, vadNeedGetCount)
				}

				var vadPcmData []float32
				pcmData := pcmFrame[:n]

				// 检查帧大小是否一致（正常情况下应该一致，但不一致时使用实际值）
				if n != frameSize {
					log.Debugf("帧大小不一致: 期望=%d, 实际=%d，使用实际值", frameSize, n)
					// 重新计算这一帧的时长
					samplesPerChannel := n / audioFormat.Channels
					currentFrameDurationMs := samplesPerChannel * 1000 / audioFormat.SampleRate
					frameSize = n
					frameDurationMs = currentFrameDurationMs
					audioFormat.FrameDuration = frameDurationMs
				}

				if !skipVad && vadProvider != nil {
					//decode opus to pcm
					state.AsrAudioBuffer.AddAsrAudioData(pcmData)

					// 计算 VAD 需要的最小数据量（60ms for silero_vad）
					vadNeedMinSize := frameSize
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						vadNeedMinSize = vadNeedGetCount * frameSize
					}

					if state.AsrAudioBuffer.GetAsrDataSize() >= vadNeedMinSize {
						//如果要进行vad, 至少要取60ms的音频数据
						vadPcmData = state.AsrAudioBuffer.GetAsrData(vadNeedGetCount, frameSize)

						//如果已经检测到语音, 则不进行vad检测, 直接将pcmData传给asr
						// 使用循环外获取的VAD资源进行检测
						// 重置VAD状态
						if err := vadProvider.Reset(); err != nil {
							log.Errorf("重置vad失败: %v", err)
							continue
						}

						// 进行VAD检测
						haveVoice, err = vadProvider.IsVADExt(vadPcmData, audioFormat.SampleRate, frameSize)
						if err != nil {
							log.Errorf("processAsrAudio VAD检测失败: %v", err)
							continue
						}

						//首次触发识别到语音时,为了语音数据完整性 将vadPcmData赋值给pcmData, 之后的音频数据全部进入asr
						if haveVoice && !clientHaveVoice {
							//首次检测到语音时，最多只保留200ms的前静音数据
							allData := state.AsrAudioBuffer.GetAndClearAllData()
							pcmData = allData
						}
					}
					//log.Debugf("isVad, pcmData len: %d, vadPcmData len: %d, haveVoice: %v", len(pcmData), len(vadPcmData), haveVoice)
				}

				if !haveVoice || state.Asr.AutoEnd {
					state.Vad.AddIdleDuration(int64(frameDurationMs))
					idleDuration := state.Vad.GetIdleDuration()
					log.Infof("空闲时间: %dms", idleDuration)
					if idleDuration > state.GetMaxIdleDuration() {
						log.Infof("超出空闲时长: %dms, 断开连接", idleDuration)
						//断开连接
						onClose()
						return
					}
				}

				if haveVoice {
					//log.Infof("检测到语音, len: %d", len(pcmData))
					state.SetClientHaveVoice(true)
					state.SetClientHaveVoiceLastTime(time.Now().UnixMilli())
					if !state.Asr.AutoEnd {
						state.Vad.ResetIdleDuration()
					}
					// 累积检测到声音的时长（同时更新一次过程中的时长）
					state.Vad.AddVoiceDuration(int64(frameDurationMs))

					continuousVoiceDuration := state.Vad.GetVoiceContinuousDuration()
					if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 1 && continuousVoiceDuration > 360 {
						// 只有在未触发过的情况下才执行，确保只执行一次
						if !hasTriggeredCancel {
							//realtime模式下, 如果此时有正在进行的llm和tts则取消掉
							log.Debugf("realtime模式vad打断下 && 语音时长超过%d ms 如果此时有正在进行的llm和tts则取消掉", continuousVoiceDuration)
							state.AfterAsrSessionCtx.Cancel()
							if a.session != nil {
								a.session.InterruptAndClearTTSQueue()
							}
							hasTriggeredCancel = true // 标记为已触发
						}
					}
				} else {
					state.Vad.ResetVoiceContinuousDuration()

					// 没有声音时，如果之前也没有语音，则重置累积的声音时长
					// 如果之前有语音但本次没有，保留时长值，让后续逻辑判断是否应该重置
					if !clientHaveVoice {
						//保留近10帧
						/*
							if state.AsrAudioBuffer.GetFrameCount(frameSize) > vadNeedGetCount*3 {
								state.AsrAudioBuffer.RemoveAsrAudioData(1, frameSize)
							}*/
						continue
					}
				}

				if clientHaveVoice {
					//vad识别成功, 往asr音频通道里发送数据
					//log.Infof("vad识别成功, 往asr音频通道里发送数据, len: %d", len(pcmData))
					state.Asr.AddAudioData(pcmData)

					// 如果启用声纹识别，同时发送到声纹识别服务
					// 需要同时满足：全局开关启用、设备配置中有声纹组、speakerManager已初始化
					if state.IsSpeakerEnabled() && state.HasSpeakerGroups() &&
						a.session != nil && a.session.speakerManager != nil {
						// 首次检测到语音时，启动流式识别
						if !a.session.speakerManager.IsActive() {
							sampleRate := audioFormat.SampleRate
							agentId := a.session.clientState.AgentID
							if err := a.session.speakerManager.StartStreaming(ctx, sampleRate, agentId); err != nil {
								log.Warnf("启动声纹识别流失败: %v", err)
							}
						}

						// 发送音频块
						if err := a.session.speakerManager.SendAudioChunk(ctx, pcmData); err != nil {
							log.Warnf("发送音频块到声纹识别服务失败: %v", err)
						}
					}
				}

				//已经有语音了, 但本次没有检测到语音, 则需要判断是否已经停止说话
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					// 判断有音频的语音时长，如果小于300ms则重置clientHaveVoice，避免短时间语音造成的误判
					voiceDurationInSession := state.Vad.GetVoiceDurationInSession()
					if voiceDurationInSession < 100 {
						log.Debugf("语音时长过短 (%dms < 300ms)，重置clientHaveVoice", voiceDurationInSession)
						state.SetClientHaveVoice(false)
						state.Vad.ResetVoiceDuration()
						continue
					}

					idleDuration := state.Vad.GetIdleDuration()
					if state.IsSilence(idleDuration) { //从有声音到 静默的判断
						// 在 OnVoiceSilence 之前重置标志位，以便下次可以再次触发
						hasTriggeredCancel = false
						state.OnVoiceSilence()
						state.VoiceStatus.Reset()
						continue
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// releaseResource 释放ASR资源（内部方法）
func (a *ASRManager) releaseResource() {
	a.resourceMu.Lock()
	defer a.resourceMu.Unlock()
	if a.asrResource != nil {
		pool.Release(a.asrResource)
		a.asrResource = nil
		log.Debugf("ASR资源已归还")
	}
}

// Cleanup 清理ASR资源（供外部调用）
func (a *ASRManager) Cleanup() {
	a.releaseResource()
}

// restartAsrRecognition 重启ASR识别
func (a *ASRManager) RestartAsrRecognition(ctx context.Context) error {
	state := a.clientState
	log.Debugf("重启ASR识别开始")

	// 取消当前ASR上下文
	if state.Asr.Cancel != nil {
		state.Asr.Cancel()
	}

	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()
	state.Asr.ClearHistoryAudio() // 清空历史音频缓存

	// 检查是否已有资源，如果没有则获取
	a.resourceMu.Lock()
	var asrProvider asr.AsrProvider
	if a.asrResource == nil {
		// 需要获取新资源
		a.resourceMu.Unlock()

		asrWrapper, err := pool.Acquire[asr.AsrProvider](
			"asr",
			state.DeviceConfig.Asr.Provider,
			state.DeviceConfig.Asr.Config,
		)
		if err != nil {
			log.Errorf("获取ASR资源失败: %v", err)
			return fmt.Errorf("获取ASR资源失败: %v", err)
		}

		// 保存资源引用到私有字段
		a.resourceMu.Lock()
		a.asrResource = asrWrapper
		asrProvider = asrWrapper.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("获取新的ASR资源")
	} else {
		// 复用现有资源
		asrProvider = a.asrResource.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("复用现有ASR资源")
	}

	// 重新创建ASR上下文和通道
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)

	// 重新启动流式识别
	asrResultChannel, err := asrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		// 识别失败，归还资源（因为资源可能已损坏）
		a.releaseResource()
		log.Errorf("重启ASR流式识别失败: %v", err)
		return fmt.Errorf("重启ASR流式识别失败: %v", err)
	}

	state.AsrResultChannel = asrResultChannel
	// 设置ASR开始时间，用于统计识别耗时
	state.SetStartAsrTs()
	log.Debugf("重启ASR识别成功")
	return nil
}

// StartAsrRecognitionLoop 启动ASR识别结果处理循环
// onMessageSave: 消息保存回调函数
// onError: 错误处理回调函数（如关闭会话）
func (a *ASRManager) StartAsrRecognitionLoop(
	ctx context.Context,
	onMessageSave AsrMessageSaveCallback,
	onError func(error),
) {
	state := a.clientState

	// 启动一个goroutine处理asr结果
	go func() {
		// 使用 defer 确保 goroutine 退出时释放 ASR 资源
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("asr结果处理goroutine panic: %v, stack: %s", r, string(debug.Stack()))
			}
			// 无论正常退出还是 panic，都释放资源
			a.releaseResource()
		}()

		//最大空闲 60s
		var startIdleTime, maxIdleTime int64
		startIdleTime = time.Now().Unix()
		maxIdleTime = 60

		// 状态不允许重启时的等待计数（避免无限循环）
		var invalidStatusWaitCount int64
		maxInvalidStatusWaitCount := int64(10) // 最多等待10次（约1秒）

		for {
			select {
			case <-ctx.Done():
				log.Debugf("asr ctx done")
				return
			default:
			}

			text, isRetry, err := state.RetireAsrResult(ctx)
			if err != nil {
				log.Errorf("处理asr结果失败: %v", err)
				if onError != nil {
					onError(err)
				}
				return
			}
			if !isRetry {
				log.Debugf("asrResult is not retry, return")
				return
			}

			//统计asr耗时
			log.Debugf("处理asr结果: %s, 耗时: %d ms", text, state.GetAsrDuration())

			if text != "" {

				// 创建用户消息
				userMsg := &schema.Message{
					Role:    schema.User,
					Content: text,
				}

				// 生成 MessageID（使用 MD5 哈希缩短长度，避免超过数据库 varchar(64) 限制）
				// 原始格式：{SessionID}-{Role}-{Timestamp}
				rawMessageID := fmt.Sprintf("%s-%s-%d",
					state.SessionID,
					userMsg.Role,
					time.Now().UnixMilli())
				// 使用 MD5 哈希生成固定32字符的十六进制字符串
				hash := md5.Sum([]byte(rawMessageID))
				messageID := hex.EncodeToString(hash[:])

				// 同步添加到内存中（用于 LLM 上下文）
				state.AddMessage(userMsg)

				// 获取音频数据（ASR 历史音频）
				audioData := state.Asr.GetHistoryAudio()
				state.Asr.ClearHistoryAudio()

				// 通过回调保存消息
				if onMessageSave != nil {
					onMessageSave(userMsg, messageID, audioData)
				}

				//如果是realtime模式下，需要停止 当前的llm和tts
				if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 2 {
					log.Debugf("OnListenStart realtime模式下, 停止当前的llm和tts")
					state.AfterAsrSessionCtx.Cancel()
					if a.session != nil {
						a.session.InterruptAndClearTTSQueue()
					}
				}

				// 重置重试计数器
				startIdleTime = time.Now().Unix()

				//当获取到asr结果时, 结束语音输入（OnVoiceSilence 中会异步获取声纹结果）
				state.OnVoiceSilence()

				//发送asr消息
				err = a.serverTransport.SendAsrResult(text)
				if err != nil {
					log.Errorf("发送asr消息失败: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// 获取暂存的声纹结果（带超时）
				speakerResult := a.getSpeakerResult()

				// 添加到队列（迁移到 ASRManager 中处理）
				if err := a.addAsrResultToQueue(text, speakerResult); err != nil {
					log.Errorf("开始对话失败: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// 非 realtime 模式下，ASR 识别完成，归还资源
				// realtime 模式下，资源会在 RestartAsrRecognition 中自动管理（先归还旧资源再获取新资源）
				if !state.IsRealTime() {
					return
				}

				// realtime 模式下，重启 ASR 识别（RestartAsrRecognition 会先归还旧资源再获取新资源）
				if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
					log.Errorf("重启ASR识别失败: %v", restartErr)
					if onError != nil {
						onError(restartErr)
					}
					return
				}
				// realtime模式下, 继续循环处理下一个 ASR 结果
				continue
			} else {
				// text 为空的情况
				select {
				case <-ctx.Done():
					log.Debugf("asr ctx done")
					return
				default:
				}

				log.Debugf("ready Restart Asr, state.Status: %s", state.Status)
				// realtime 模式下，即使状态是 LLMStart 或 TTSStart，也应该继续监听（允许重启ASR）
				// 非 realtime 模式下，只有 Listening 或 ListenStop 状态才允许重启ASR
				isAllowedToRestart := state.Status == ClientStatusListening || state.Status == ClientStatusListenStop
				if state.IsRealTime() {
					// realtime 模式下，除了 Init 状态外，都允许重启（因为需要持续监听）
					isAllowedToRestart = state.Status != ClientStatusInit
				}

				if isAllowedToRestart {
					// 状态允许重启，重置等待计数
					invalidStatusWaitCount = 0
					// text 为空，检查是否需要重新启动ASR
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR识别结果为空，尝试重启ASR识别, diff ts: %d", diffTs)
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("重启ASR识别失败: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					} else {
						log.Warnf("ASR识别结果为空，已达到最大空闲时间: %d", maxIdleTime)
						if onError != nil {
							onError(fmt.Errorf("ASR识别结果为空，已达到最大空闲时间: %d", maxIdleTime))
						}
						return
					}
				} else {
					// 状态不允许重启的情况，短暂等待后继续循环，给状态恢复的机会
					invalidStatusWaitCount++
					if invalidStatusWaitCount >= maxInvalidStatusWaitCount {
						// 等待超时，退出循环
						log.Debugf("状态为 %s，realtime: %v，等待%d次后仍无变化，退出ASR识别循环", state.Status, state.IsRealTime(), maxInvalidStatusWaitCount)
						return
					}
					// 短暂等待后继续循环，等待状态恢复
					log.Debugf("状态为 %s，realtime: %v，不允许重启，等待状态恢复 (等待次数: %d/%d)", state.Status, state.IsRealTime(), invalidStatusWaitCount, maxInvalidStatusWaitCount)
					time.Sleep(200 * time.Millisecond) // 等待100ms
					continue
				}
			}
		}
	}()
}

// getSpeakerResult 获取暂存的声纹结果（带超时）
func (a *ASRManager) getSpeakerResult() *speaker.IdentifyResult {
	if a.session == nil || a.session.speakerManager == nil {
		return nil
	}

	log.Debugf("speakerManager: %+v, IsActive: %+v", a.session.speakerManager, a.session.speakerManager.IsActive())

	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()

	var speakerResult *speaker.IdentifyResult
	select {
	case <-a.session.speakerResultReady:
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
	case <-timeout.C:
		// 超时后读取当前结果（可能为 nil）
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
		log.Debugf("获取声纹识别结果超时，使用当前结果")
	}
	log.Debugf("获取声纹识别结果: %+v", speakerResult)
	return speakerResult
}

// addAsrResultToQueue 添加ASR结果到队列（迁移到 ASRManager 中处理）
func (a *ASRManager) addAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	if a.session == nil {
		return fmt.Errorf("session is nil")
	}
	return a.session.AddAsrResultToQueue(text, speakerResult)
}
