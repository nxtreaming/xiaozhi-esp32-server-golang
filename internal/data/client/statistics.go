package client

import "time"

type Statistic struct {
	AsrStartTs int64 //asr开始时间
	LlmStartTs int64 //llm开始时间
	TtsStartTs int64 //tts开始时间
}

func (s *Statistic) Reset() {
	s.AsrStartTs = 0
	s.LlmStartTs = 0
	s.TtsStartTs = 0
}

func (state *ClientState) SetStartAsrTs() {
	state.Statistic.AsrStartTs = time.Now().UnixMilli()
}

func (state *ClientState) GetAsrDuration() int64 {
	if state.Statistic.AsrStartTs == 0 {
		return 0
	}
	return time.Now().UnixMilli() - state.Statistic.AsrStartTs
}

func (state *ClientState) GetAsrLlmTtsDuration() int64 {
	return time.Now().UnixMilli() - state.Statistic.AsrStartTs
}

func (state *ClientState) SetStartLlmTs() {
	state.Statistic.LlmStartTs = time.Now().UnixMilli()
}

func (state *ClientState) GetLlmDuration() int64 {
	return time.Now().UnixMilli() - state.Statistic.LlmStartTs
}

func (state *ClientState) SetStartTtsTs() {
	state.Statistic.TtsStartTs = time.Now().UnixMilli()
}

func (state *ClientState) GetTtsDuration() int64 {
	return time.Now().UnixMilli() - state.Statistic.TtsStartTs
}
