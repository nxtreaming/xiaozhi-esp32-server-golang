package chat

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

const (
	MaxMessageCount = 10

	McpReadResourcePageSize       = 100 * 1024
	McpReadResourceStreamDoneFlag = "[DONE]"
)

// Context key 类型用于避免冲突
type contextKey int

const (
	fullTextKey contextKey = iota
)

// GetLastMessageID 获取最近保存的消息的 MessageID（用于两阶段保存）
func (l *LLMManager) GetLastMessageID(role string) (string, bool) {
	l.lastMessageIDMu.RLock()
	defer l.lastMessageIDMu.RUnlock()
	id, ok := l.lastMessageID[role]
	return id, ok
}

type LLMResponseChannelItem struct {
	ctx          context.Context
	userMessage  *schema.Message
	responseChan chan llm_common.LLMResponseStruct
	onStartFunc  func(args ...any)
	onEndFunc    func(err error, args ...any)
}

type LLMManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	ttsManager      *TTSManager

	einoTools []*schema.ToolInfo

	llmResponseQueue *util.Queue[LLMResponseChannelItem]

	// 存储最近保存的消息的 MessageID（用于两阶段保存）
	// key: role (user/assistant), value: MessageID
	lastMessageID   map[string]string
	lastMessageIDMu sync.RWMutex // 保护 lastMessageID 的并发访问
}

func NewLLMManager(clientState *ClientState, serverTransport *ServerTransport, ttsManager *TTSManager) *LLMManager {
	return &LLMManager{
		clientState:      clientState,
		serverTransport:  serverTransport,
		ttsManager:       ttsManager,
		llmResponseQueue: util.NewQueue[LLMResponseChannelItem](10),
		lastMessageID:    make(map[string]string),
	}
}

func (l *LLMManager) Start(ctx context.Context) {
	l.processLLMResponseQueue(ctx)
}

func (l *LLMManager) processLLMResponseQueue(ctx context.Context) {
	for {
		item, err := l.llmResponseQueue.Pop(ctx, 0) // 阻塞式
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			// 其他错误
			continue
		}

		log.Debugf("processLLMResponseQueue item: %+v", item)
		if item.onStartFunc != nil {
			item.onStartFunc()
		}

		// 调用 handleLLMResponse，它会从 context 中获取 fullText 和 toolCalls 并填充
		_, err = l.handleLLMResponse(item.ctx, item.userMessage, item.responseChan)

		if item.onEndFunc != nil {
			item.onEndFunc(err)
		}
	}
}

func (l *LLMManager) ClearLLMResponseQueue() {
	l.llmResponseQueue.Clear()
}

func (l *LLMManager) AddTextToTTSQueue(text string) error {
	log.Debugf("AddTextToTTSQueue text: %s", text)
	msg := &schema.Message{
		Role:    schema.User,
		Content: text,
	}
	llmResponseChan := make(chan llm_common.LLMResponseStruct, 10)
	llmResponseChan <- llm_common.LLMResponseStruct{
		IsStart: true,
		IsEnd:   true,
		Text:    text,
	}
	close(llmResponseChan)

	sessionCtx := l.clientState.SessionCtx.Get(l.clientState.Ctx)
	ctx := l.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	l.HandleLLMResponseChannelAsync(ctx, msg, llmResponseChan)

	return nil
}

func (l *LLMManager) HandleLLMResponseChannelAsync(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct) error {
	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}

	// 在 context 中初始化或复用 fullText（用于聊天历史）
	// 如果 context 中已有 fullText（工具调用后继续LLM请求），则复用；否则创建新的
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("复用已有的 fullText，当前长度: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("创建新的 fullText")
	}

	var onStartFunc func(...any)
	var onEndFunc func(err error, args ...any)

	if needSendTtsCmd {
		onStartFunc = func(...any) {
			// 判断是否为首次LLM调用（通过context的nest值），仅首次调用时清空TTS音频缓存
			val := ctx.Value("nest")
			if nest, ok := val.(int); !ok || nest <= 1 {
				// 首次调用或没有nest值，清空TTS音频缓存
				l.ttsManager.ClearAudioHistory()
				log.Debugf("onStartFunc 首次调用，已清空TTS音频缓存")
			}
			l.serverTransport.SendTtsStart()
		}
		onEndFunc = func(err error, args ...any) {
			l.serverTransport.SendTtsStop()

			// 从 closure 中获取 fullText
			audioData := l.ttsManager.GetAndClearAudioHistory()
			strFullText := fullText.String()

			// 计算总音频大小（所有帧的字节数之和）
			audioSize := 0
			for _, frame := range audioData {
				audioSize += len(frame)
			}

			// 只有在首次调用（nest<=1）时才发送事件
			if nest <= 1 {
				// 从 LLMManager 中获取 MessageID（Assistant 角色）
				// 如果没有找到 MessageID，说明第一阶段保存未完成，不进行第二阶段更新
				messageID, ok := l.GetLastMessageID(string(schema.Assistant))
				if !ok {
					log.Warnf("TTS 完成时未找到 MessageID，跳过第二阶段音频更新")
					return
				}

				// 发布事件：第二阶段（更新音频）
				assistantMsg := schema.AssistantMessage(strFullText, nil)
				eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
					ClientState: l.clientState,
					Msg:         *assistantMsg,
					MessageID:   messageID,
					AudioData:   audioData, // 第二阶段：有音频
					AudioSize:   audioSize,
					SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
					Channels:    l.clientState.OutputAudioFormat.Channels,
					Timestamp:   time.Now(),
					IsUpdate:    true, // 更新消息
				})
			}
		}
	}

	item := LLMResponseChannelItem{
		ctx:          ctx,
		userMessage:  userMessage,
		responseChan: responseChan,
		onStartFunc:  onStartFunc,
		onEndFunc:    onEndFunc,
	}
	err := l.llmResponseQueue.Push(item)
	if err != nil {
		log.Warnf("llmResponseQueue 已满或已关闭, 丢弃消息")
		return fmt.Errorf("llmResponseQueue 已满或已关闭, 丢弃消息")
	}
	return nil
}

func (l *LLMManager) HandleLLMResponseChannelSync(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct, einoTools []*schema.ToolInfo) (bool, error) {
	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}

	// 在 context 中初始化或复用 fullText（用于聊天历史）
	// 如果 context 中已有 fullText（工具调用后继续LLM请求），则复用；否则创建新的
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("复用已有的 fullText，当前长度: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("创建新的 fullText")
	}

	if needSendTtsCmd {
		// 判断是否为首次LLM调用（通过context的nest值），仅首次调用时清空TTS音频缓存
		if nest <= 1 {
			// 首次调用或没有nest值，清空TTS音频缓存
			l.ttsManager.ClearAudioHistory()
			log.Debugf("HandleLLMResponseChannelSync 首次调用，已清空TTS音频缓存")
		}
		l.serverTransport.SendTtsStart()
	}

	ok, err := l.handleLLMResponse(ctx, userMessage, llmResponseChannel)

	if needSendTtsCmd {
		l.serverTransport.SendTtsStop()

		// 收集TTS音频并发送聊天历史事件
		// 注意：工具调用后的LLM响应（nest > 1）也会累积音频到缓存中，但不会清空
		// 只有在首次调用（nest<=1）时才清空缓存并发送事件
		audioData := l.ttsManager.GetAndClearAudioHistory()

		// 计算总音频大小（所有帧的字节数之和）
		audioSize := 0
		for _, frame := range audioData {
			audioSize += len(frame)
		}

		// 只有在首次调用（nest<=1）时才发送事件
		if nest <= 1 {
			// 从 LLMManager 中获取 MessageID（Assistant 角色）
			// 如果没有找到 MessageID，说明第一阶段保存未完成，不进行第二阶段更新
			messageID, ok := l.GetLastMessageID(string(schema.Assistant))
			if !ok {
				log.Warnf("TTS 完成时未找到 MessageID，跳过第二阶段音频更新")
				return ok, err
			}

			// 发布事件：第二阶段（更新音频）
			assistantMsg := schema.AssistantMessage(fullText.String(), nil)
			eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
				ClientState: l.clientState,
				Msg:         *assistantMsg,
				MessageID:   messageID,
				AudioData:   audioData, // 第二阶段：有音频
				AudioSize:   audioSize,
				SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
				Channels:    l.clientState.OutputAudioFormat.Channels,
				Timestamp:   time.Now(),
			})
		}
	} else {
		// nest > 1 的情况：虽然不发送TTS命令，但音频数据仍然会累积到缓存中
		// 这些音频会在首次响应结束时（nest <= 1）一起收集
		log.Debugf("工具调用后的LLM响应（nest=%d），音频数据将累积到缓存中", nest)
	}

	return ok, err
}

// handleLLMResponse 处理LLM响应
func (l *LLMManager) handleLLMResponse(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
	log.Debugf("handleLLMResponse start")
	defer log.Debugf("handleLLMResponse end")
	select {
	case <-ctx.Done():
		log.Debugf("handleLLMResponse ctx done, return")
		return false, nil
	default:
	}

	state := l.clientState

	// 从 context 中获取 fullText（用于聊天历史）
	fullText := ctx.Value(fullTextKey).(*strings.Builder)
	// toolCalls 使用局部变量（内部工具调用逻辑，不涉及聊天历史）
	var toolCalls []schema.ToolCall

	for {
		select {
		case <-ctx.Done():
			// 上下文已取消，优先处理取消逻辑
			log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
			return false, nil
		default:
			// 非阻塞检查，如果ctx没有Done，继续处理LLM响应
			select {
			case llmResponse, ok := <-llmResponseChannel:
				if !ok {
					// 通道已关闭，退出协程
					log.Infof("LLM 响应通道已关闭，退出协程")
					return true, nil
				}

				log.Debugf("LLM 响应: %+v", llmResponse)

				if len(llmResponse.ToolCalls) > 0 {
					log.Debugf("获取到工具: %+v", llmResponse.ToolCalls)
					toolCalls = append(toolCalls, llmResponse.ToolCalls...)
				}

				if llmResponse.Text != "" {
					// 处理文本内容响应
					if err := l.ttsManager.handleTextResponse(ctx, llmResponse, true); err != nil {
						return true, err
					}
					fullText.WriteString(llmResponse.Text)
				}

				if llmResponse.IsEnd {
					if len(toolCalls) == 0 {
						//写到redis中
						if userMessage != nil {
							if userMessage.Role == schema.User {
								// 检查用户消息是否已经保存过（ASR 处理时已经保存）
								// 通过检查最后一条消息是否是用户消息且内容匹配来判断
								messages := l.clientState.GetMessages(1)
								shouldSave := true
								if len(messages) > 0 {
									lastMsg := messages[len(messages)-1]
									if lastMsg.Role == schema.User && lastMsg.Content == userMessage.Content {
										// 用户消息已经保存过了（ASR 处理时保存的），跳过
										shouldSave = false
										log.Debugf("用户消息已在 ASR 处理时保存，跳过重复保存: %s", userMessage.Content)
									}
								}
								if shouldSave {
									if err := l.AddLlmMessage(ctx, userMessage); err != nil {
										log.Errorf("保存用户消息失败: %v", err)
									}
								}
							}
						}
						strFullText := fullText.String()
						if strFullText != "" || len(toolCalls) > 0 {
							if err := l.AddLlmMessage(ctx, schema.AssistantMessage(strFullText, toolCalls)); err != nil {
								log.Errorf("保存助手消息失败: %v", err)
							}
						}
					}
					if len(toolCalls) > 0 {
						lctx := context.WithValue(ctx, "nest", 2)
						// 将 fullText 传递到新的 context（toolCalls 直接作为参数传递）
						lctx = context.WithValue(lctx, fullTextKey, fullText)
						invokeToolSuccess, err := l.handleToolCallResponse(lctx, userMessage, schema.AssistantMessage(fullText.String(), toolCalls), toolCalls)
						if err != nil {
							log.Errorf("处理工具调用响应失败: %v", err)
							return true, fmt.Errorf("处理工具调用响应失败: %v", err)
						}
						if !invokeToolSuccess {
							//工具调用失败
							if err := l.ttsManager.handleTextResponse(ctx, llmResponse, false); err != nil {
								return true, err
							}
							fullText.WriteString(llmResponse.Text)
						}
					}

					return true, nil
				}
			case <-ctx.Done():
				// 上下文已取消，退出协程
				log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
				return false, nil
			}
		}
	}
}

// handleToolCallResponse 处理工具调用响应
func (l *LLMManager) handleToolCallResponse(ctx context.Context, userMessage *schema.Message, respMsg *schema.Message, tools []schema.ToolCall) (bool, error) {
	if len(tools) == 0 {
		return false, nil
	}

	state := l.clientState

	log.Infof("处理 %d 个工具调用", len(tools))

	var invokeToolSuccess bool

	// 从 context 中获取 chat_session_operator（如果存在）
	// 如果不存在，说明没有需要 ChatSession 操作的工具，可以正常执行
	var toolCtx context.Context = ctx
	if chatSessionOperator, ok := ctx.Value("chat_session_operator").(ChatSessionOperator); ok {
		// 在 context 中传递 chat_session_operator，供 local mcp tool 使用
		toolCtx = context.WithValue(ctx, "chat_session_operator", chatSessionOperator)
	}

	var shouldStopLLMProcessing bool

	var wg sync.WaitGroup

	var messageList []*schema.Message

	messageList = append(messageList, userMessage, respMsg)

	addMessageFunc := func(toolCall schema.ToolCall, result string) {
		toolResultMsg := &schema.Message{
			Role:       schema.Tool,
			ToolCallID: toolCall.ID,
			Content:    result,
		}
		messageList = append(messageList, toolResultMsg)
	}

	for _, toolCall := range tools {
		toolName := toolCall.Function.Name
		tool, ok := mcp.GetToolByName(state.DeviceID, toolName)
		if !ok || tool == nil {
			log.Errorf("未找到工具: %s", toolName)
			addMessageFunc(toolCall, fmt.Sprintf("未找到工具: %s", toolName))
			continue
		}
		log.Infof("进行工具调用请求: %s, 参数: %+v", toolName, toolCall.Function.Arguments)
		startTs := time.Now().UnixMilli()
		fcResult, err := tool.InvokableRun(toolCtx, toolCall.Function.Arguments)
		if err != nil {
			log.Errorf("工具调用失败: %v", err)
			addMessageFunc(toolCall, fmt.Sprintf("工具 %s 调用失败: %v", toolName, err))
			continue
		}
		costTs := time.Now().UnixMilli() - startTs
		invokeToolSuccess = true
		if len(fcResult) > 2048 {
			log.Infof("工具调用结果 len: %d, 耗时: %dms", len(fcResult), costTs)
		} else {
			log.Infof("工具调用结果 %s, 耗时: %dms", fcResult, costTs)
		}

		var result string = fcResult
		var contentList []mcp_go.Content
		if mcpResp, ok := l.handleLocalToolResult(fcResult); ok {
			/*if mcpResp.IsTerminal() {
				log.Infof("工具调用结果: %s, 终止: %t", fcResult, mcpResp.IsTerminal())
				return invokeToolSuccess, nil
			}*/
			contentList = mcpResp.GetContent()
		} else if toolCallResult, ok := l.handleToolResult(fcResult); ok {
			if toolCallResult.IsError {
				log.Errorf("工具调用失败: %s, 错误: %s", fcResult, toolCallResult.IsError)
			}
			contentList = toolCallResult.Content
		}
		if len(contentList) > 0 {
			var mcpContent string
			//如果有audio数据, 则进行播放
			for _, content := range contentList {
				if audioContent, ok := content.(mcp_go.AudioContent); ok {
					log.Debugf("调用工具 %s 返回音频资源长度: %d", toolName, len(audioContent.Data))

					mcpContent = "执行成功"
					//播放音频资源,此时mcpContent是
					err := l.handleAudioContent(ctx, mcpContent, audioContent, &wg)
					if err != nil {
						log.Errorf("mcp播放音频资源失败: %v", err)
						mcpContent = "执行失败"
					}
					shouldStopLLMProcessing = true
					break
				} else if resourceLink, ok := content.(mcp_go.ResourceLink); ok {
					log.Debugf("调用工具 %s 返回资源链接: %+v", toolName, resourceLink)
					mcpContent = "执行成功"
					err := l.handleResourceLink(ctx, resourceLink, tool, &wg)
					if err != nil {
						log.Errorf("mcp播放资源链接失败: %v", err)
						mcpContent = "执行失败"
					}

					shouldStopLLMProcessing = true
					break
				} else if textContent, ok := content.(mcp_go.TextContent); ok {
					log.Debugf("调用工具 %s 返回文本资源长度: %s", toolName, textContent.Text)
					mcpContent += textContent.Text
				}
			}
			if mcpContent != "" {
				result = mcpContent
			}
		}
		addMessageFunc(toolCall, result)
	}

	if len(messageList) > 0 {
		for _, msg := range messageList {
			l.AddLlmMessage(ctx, msg)
		}
	}

	wg.Wait()

	// 如果工具调用成功且没有被标记为停止处理，则继续LLM调用
	if invokeToolSuccess && !shouldStopLLMProcessing {
		l.DoLLmRequest(ctx, nil, l.einoTools, true, nil)
	}

	return invokeToolSuccess, nil
}

func (l *LLMManager) handleResourceLink(ctx context.Context, resourceLink mcp_go.ResourceLink, toolCall tool.InvokableTool, wg *sync.WaitGroup) error {
	wg.Add(1)
	//从resourceLink中获取资源
	client := toolCall.(*mcp.McpTool).GetClient()

	var pipeReader *io.PipeReader
	var pipeWriter *io.PipeWriter
	pipeReader, pipeWriter = io.Pipe()

	audioFormat := util.GetAudioFormatByMimeType(resourceLink.MIMEType)

	streamChan := make(chan []byte, 0) // 增加缓冲区大小
	go func() error {
		defer func() {
			close(streamChan)
		}()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case audioData, ok := <-streamChan:
					if !ok {
						pipeWriter.Close()
						return
					}
					if _, err := pipeWriter.Write(audioData); err != nil {
						log.Errorf("写入pipe失败: %v", err)
						return
					}
				}
			}
		}()

		start := 0
		page := McpReadResourcePageSize
		totalRead := 0
		pageCount := 0

		log.Infof("开始读取资源: %s, 分页大小: %d", resourceLink.URI, page)

		for {
			select {
			case <-ctx.Done():
				log.Debugf("资源读取被取消")
				return nil
			default:
				pageCount++
				log.Debugf("读取第 %d 页资源，起始位置: %d, 结束位置: %d", pageCount, start, start+page)

				// 创建带超时的上下文
				readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

				// 读取资源
				resourceResult, err := client.ReadResource(readCtx, mcp_go.ReadResourceRequest{
					Params: mcp_go.ReadResourceParams{
						URI:       resourceLink.URI,
						Arguments: map[string]any{"url": resourceLink.Description, "start": start, "end": start + page},
					},
				})
				cancel()

				if err != nil {
					log.Errorf("读取资源失败 (第 %d 页), resourceUri: %s, resourceResult: %+v, err: %v", pageCount, resourceLink.Description, resourceResult, err)

					// 如果是超时错误，尝试重试
					if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
						log.Warnf("资源读取超时，尝试重试...")
						time.Sleep(1 * time.Second)
						continue
					}

					return fmt.Errorf("读取资源失败: %v", err)
				}

				if len(resourceResult.Contents) == 0 {
					log.Infof("资源读取完成，总共读取 %d 字节，共 %d 页", totalRead, pageCount-1)
					return nil
				}

				hasData := false
				for _, content := range resourceResult.Contents {
					if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
						if len(audioContent.Blob) == 0 {
							log.Debugf("音频数据为空，跳过")
							continue
						}
						log.Debugf("第 %d 页 resourceResult len: %d", pageCount, len(audioContent.Blob))
						rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
						if err != nil {
							log.Errorf("解码音频数据失败: %v", err)
							return fmt.Errorf("解码音频数据失败: %v", err)
						}

						if string(rawAudioData) == McpReadResourceStreamDoneFlag {
							log.Debugf("资源读取完成")
							return nil
						}

						select {
						case <-ctx.Done():
							log.Debugf("资源读取被取消")
							return nil
						case streamChan <- rawAudioData:
							totalRead += len(rawAudioData)
							hasData = true
							log.Debugf("成功发送第 %d 页数据，长度: %d, 累计: %d", pageCount, len(rawAudioData), totalRead)
						}

						if len(rawAudioData) < page {
							log.Debugf("资源读取完成")
							return nil
						}
					}
				}

				// 如果这一页没有数据，说明已经读取完毕
				if !hasData {
					log.Infof("资源读取完成，总共读取 %d 字节，共 %d 页", totalRead, pageCount)
					return nil
				}

				start += page
			}
		}
	}()

	// 使用music_player播放音乐
	audioChan, err := play_music.PlayMusicFromPipe(ctx, pipeReader, l.clientState.OutputAudioFormat.SampleRate, l.clientState.OutputAudioFormat.FrameDuration, audioFormat)
	if err != nil {
		log.Errorf("播放音乐失败: %v", err)
		return fmt.Errorf("播放音乐失败: %v", err)
	}

	playText := fmt.Sprintf("正在播放音乐: %s", resourceLink.Name)
	l.serverTransport.SendSentenceStart(playText)

	go func() {
		defer func() {
			l.serverTransport.SendSentenceEnd(playText)
			log.Infof("音乐播放完成: %s", resourceLink.Name)
		}()

		l.ttsManager.SendTTSAudio(ctx, audioChan, true)
		wg.Done()
	}()

	return nil
}

func (l *LLMManager) handleAudioContent(ctx context.Context, realMusicName string, audioContent mcp_go.AudioContent, wg *sync.WaitGroup) error {
	wg.Add(1)
	rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Data)
	if err != nil {
		log.Errorf("解码音频数据失败: %v", err)
		return fmt.Errorf("解码音频数据失败: %v", err)
	}
	audioFormat := util.GetAudioFormatByMimeType(audioContent.MIMEType)
	// 使用music_player播放音乐
	audioChan, err := play_music.PlayMusicFromAudioData(ctx, rawAudioData, l.clientState.OutputAudioFormat.SampleRate, l.clientState.OutputAudioFormat.FrameDuration, audioFormat)
	if err != nil {
		log.Errorf("播放音乐失败: %v", err)
		return fmt.Errorf("播放音乐失败: %v", err)
	}

	playText := fmt.Sprintf("正在播放音乐: %s", realMusicName)
	l.serverTransport.SendSentenceStart(playText)

	go func() {
		defer func() {
			l.serverTransport.SendSentenceEnd(playText)
			log.Infof("音乐播放完成: %s", realMusicName)
		}()
		l.ttsManager.SendTTSAudio(ctx, audioChan, true)
		wg.Done()
	}()

	return nil
}

func (l *LLMManager) handleLocalToolResult(toolResult string) (MCPResponse, bool) {
	// 首先尝试解析新的结构化响应
	var response MCPResponse
	var err error
	if response, err = ParseMCPResponse(toolResult); err != nil {
		return nil, false
	}
	return response, true
}

func (l *LLMManager) handleToolResult(toolResultStr string) (mcp_go.CallToolResult, bool) {
	var toolResult mcp_go.CallToolResult
	if err := json.Unmarshal([]byte(toolResultStr), &toolResult); err != nil {
		log.Errorf("解析工具结果失败: %v", err)
		return toolResult, false
	}

	return toolResult, true
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, userMessage *schema.Message, einoTools []*schema.ToolInfo, isSync bool, speakerResult *speaker.IdentifyResult) error {
	log.Debugf("发送带工具的 LLM 请求, seesionID: %s, requestEinoMessages: %+v", l.clientState.SessionID, userMessage)
	clientState := l.clientState

	l.einoTools = einoTools

	//组装历史消息和当前用户的消息
	requestMessages := l.GetMessages(ctx, userMessage, MaxMessageCount, speakerResult)
	clientState.SetStatus(ClientStatusLLMStart)
	responseSentences, err := llm.HandleLLMWithContextAndTools(
		ctx,
		clientState.LLMProvider,
		requestMessages,
		einoTools,
		l.clientState.SessionID,
	)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}

	log.Debugf("DoLLmRequest goroutine开始 - SessionID: %s, context状态: %v", l.clientState.SessionID, ctx.Err())

	if isSync {
		_, err := l.HandleLLMResponseChannelSync(ctx, userMessage, responseSentences, einoTools)
		if err != nil {
			log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
			return err
		}
	} else {
		err = l.HandleLLMResponseChannelAsync(ctx, userMessage, responseSentences)
		if err != nil {
			log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
		}
	}

	log.Debugf("DoLLmRequest 结束 - SessionID: %s", l.clientState.SessionID)

	return nil
}

// AddMessage 添加消息到聊天历史（统一入口，适用于所有消息类型）
func (l *LLMManager) AddMessage(ctx context.Context, msg *schema.Message) error {
	if msg == nil {
		log.Warnf("尝试添加 nil 消息到聊天历史")
		return fmt.Errorf("消息不能为 nil")
	}

	// 生成 MessageID（使用 MD5 哈希缩短长度，避免超过数据库 varchar(64) 限制）
	// 原始格式：{SessionID}-{Role}-{Timestamp}
	rawMessageID := fmt.Sprintf("%s-%s-%d",
		l.clientState.SessionID,
		msg.Role,
		time.Now().UnixMilli())
	// 使用 MD5 哈希生成固定32字符的十六进制字符串
	hash := md5.Sum([]byte(rawMessageID))
	messageID := hex.EncodeToString(hash[:])

	// 同步添加到内存中
	l.clientState.AddMessage(msg)

	// Tool 角色消息：直接保存，不涉及两阶段保存（无音频）
	if msg.Role == schema.Tool {
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: l.clientState,
			Msg:         *msg,
			MessageID:   messageID,
			AudioData:   nil, // Tool 角色无音频
			AudioSize:   0,
			SampleRate:  0,
			Channels:    0,
			Timestamp:   time.Now(),
			IsUpdate:    false, // 一次性保存
		})
		return nil
	}

	// User/Assistant 角色：两阶段保存
	// 将 MessageID 存储到 LLMManager 中，供后续音频更新使用
	if msg.Role == schema.User || msg.Role == schema.Assistant {
		l.lastMessageIDMu.Lock()
		l.lastMessageID[string(msg.Role)] = messageID
		l.lastMessageIDMu.Unlock()
	}

	// 发布事件：第一阶段（仅文本，无音频）
	eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
		ClientState: l.clientState,
		Msg:         *msg,
		MessageID:   messageID,
		AudioData:   nil, // 第一阶段：无音频
		AudioSize:   0,
		SampleRate:  0,
		Channels:    0,
		Timestamp:   time.Now(),
		IsUpdate:    false, // 新增消息
	})

	return nil
}

// AddLlmMessage 保持向后兼容，委托给 AddMessage
func (l *LLMManager) AddLlmMessage(ctx context.Context, msg *schema.Message) error {
	return l.AddMessage(ctx, msg)
}

func (l *LLMManager) GetMessages(ctx context.Context, userMessage *schema.Message, count int, speakerResult *speaker.IdentifyResult) []*schema.Message {
	//从dialogue中获取
	messageList := l.clientState.GetMessages(count)

	// 构建 system prompt
	systemPrompt := l.clientState.SystemPrompt
	if l.clientState.MemoryContext != "" {
		systemPrompt += fmt.Sprintf("\n用户个性化信息: \n%s", l.clientState.MemoryContext)
	}

	log.Debugf("speakerResult: %+v, voiceIdentify: %+v", speakerResult, l.clientState.DeviceConfig.VoiceIdentify)

	// 整合说话人识别结果到 systemPrompt
	if speakerResult != nil && speakerResult.Identified {
		// 根据 speakerResult 匹配 userConfig 中的 speakerGroup 信息
		if l.clientState.DeviceConfig.VoiceIdentify != nil {
			// 优先使用 SpeakerName 匹配（VoiceIdentify 的 key 是 speakerGroup.Name）
			if speakerGroupInfo, found := l.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]; found {
				// 如果找到匹配的 speakerGroup，将描述整合到 systemPrompt
				if speakerGroupInfo.Prompt != "" {
					systemPrompt += fmt.Sprintf("\n基于声纹识别到对话人信息: \n%s", speakerGroupInfo.Prompt)
				}
			}
		}
	}

	//search memory
	if l.clientState.MemoryProvider != nil && userMessage != nil {
		memoryContext, err := l.clientState.MemoryProvider.Search(ctx, l.clientState.GetDeviceIDOrAgentID(), userMessage.Content, 10, 180)
		if err != nil {
			log.Errorf("搜索记忆失败: %v", err)
		}
		log.Debugf("搜索记忆成功, 输入内容: %s, 记忆内容: %s", userMessage.Content, memoryContext)
		if memoryContext != "" {
			systemPrompt += fmt.Sprintf("\n历史关联信息: \n%s", memoryContext)
		}
	}

	retMessage := make([]*schema.Message, 0)
	retMessage = append(retMessage, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})
	retMessage = append(retMessage, messageList...)
	if userMessage != nil {
		retMessage = append(retMessage, userMessage)
	}
	return retMessage
}
