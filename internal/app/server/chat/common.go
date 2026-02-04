package chat

func (s *ChatSession) StopSpeaking(isSendTtsStop bool) {
	s.ClearChatTextQueue()
	s.llmManager.ClearLLMResponseQueue()
	s.ttsManager.ClearTTSQueue()
	s.ttsManager.InterruptAndClearQueue()

	s.clientState.SessionCtx.Cancel()

	if isSendTtsStop {
		s.serverTransport.SendTtsStop()
	}

}

func (s *ChatSession) MqttClose() {
	s.serverTransport.SendMqttGoodbye()
}
