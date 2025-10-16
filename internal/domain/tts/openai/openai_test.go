package openai

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenAITTS(t *testing.T) {
	// 跳过实际的网络请求测试，除非设置了环境变量
	if os.Getenv("RUN_OPENAI_TEST") != "1" {
		t.Skip("跳过OpenAI API测试，设置环境变量RUN_OPENAI_TEST=1以启用")
	}

	// 从环境变量获取API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("跳过OpenAI API测试，需要设置环境变量OPENAI_API_KEY")
	}

	config := map[string]interface{}{
		"api_key":         apiKey,
		"api_url":         "https://api.openai.com/v1/audio/speech",
		"model":           "tts-1",
		"voice":           "alloy",
		"response_format": "mp3",
		"speed":           1.0,
		"frame_duration":  float64(60),
	}

	provider := NewOpenAITTSProvider(config)

	// 测试文本转语音
	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		frames, err := provider.TextToSpeech(ctx, "Hello, this is a test of OpenAI text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeech失败: %v", err)
		}

		if len(frames) == 0 {
			t.Error("未返回任何音频帧")
		}

		t.Logf("成功生成 %d 个音频帧", len(frames))
	})

	// 测试流式文本转语音
	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outputChan, err := provider.TextToSpeechStream(ctx, "Hello, this is a test of OpenAI streaming text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeechStream失败: %v", err)
		}

		// 接收所有帧
		var receivedFrames [][]byte
		timeout := time.After(20 * time.Second)

	receiveLoop:
		for {
			select {
			case frame, ok := <-outputChan:
				if !ok {
					break receiveLoop
				}
				receivedFrames = append(receivedFrames, frame)
			case <-timeout:
				t.Error("接收音频帧超时")
				break receiveLoop
			}
		}

		if len(receivedFrames) == 0 {
			t.Error("未接收到任何音频帧")
		}

		t.Logf("成功接收 %d 个音频帧", len(receivedFrames))
	})

	// 测试不同的语音
	t.Run("TestDifferentVoices", func(t *testing.T) {
		voices := []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}

		for _, voice := range voices {
			t.Run(voice, func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           voice,
					"response_format": "mp3",
					"speed":           1.0,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing voice: "+voice, 16000, 1, 60)
				if err != nil {
					t.Errorf("使用语音 %s 失败: %v", voice, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("语音 %s 未返回任何音频帧", voice)
				}

				t.Logf("语音 %s 成功生成 %d 个音频帧", voice, len(frames))
			})
		}
	})

	// 测试不同的速度
	t.Run("TestDifferentSpeeds", func(t *testing.T) {
		speeds := []float64{0.5, 1.0, 1.5, 2.0}

		for _, speed := range speeds {
			t.Run(string(rune(speed)), func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           "alloy",
					"response_format": "mp3",
					"speed":           speed,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing speed", 16000, 1, 60)
				if err != nil {
					t.Errorf("使用速度 %.1f 失败: %v", speed, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("速度 %.1f 未返回任何音频帧", speed)
				}

				t.Logf("速度 %.1f 成功生成 %d 个音频帧", speed, len(frames))
			})
		}
	})
}

// TestOpenAITTSProviderDefaults 测试默认值
func TestOpenAITTSProviderDefaults(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider := NewOpenAITTSProvider(config)

	if provider.APIURL != "https://api.openai.com/v1/audio/speech" {
		t.Errorf("期望默认API URL为 https://api.openai.com/v1/audio/speech，实际为 %s", provider.APIURL)
	}

	if provider.Model != "tts-1" {
		t.Errorf("期望默认模型为 tts-1，实际为 %s", provider.Model)
	}

	if provider.Voice != "alloy" {
		t.Errorf("期望默认语音为 alloy，实际为 %s", provider.Voice)
	}

	if provider.ResponseFormat != "mp3" {
		t.Errorf("期望默认响应格式为 mp3，实际为 %s", provider.ResponseFormat)
	}

	if provider.Speed != 1.0 {
		t.Errorf("期望默认速度为 1.0，实际为 %.1f", provider.Speed)
	}
}
