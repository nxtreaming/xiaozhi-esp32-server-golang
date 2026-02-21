package controllers

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"
	"xiaozhi/manager/backend/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type CloneProviderCapability struct {
	Enabled            bool
	RequiresTranscript bool
	MinTextLen         int
	MaxTextLen         int
	SupportedLangs     map[string]bool
}

var cloneProviderCapabilities = map[string]CloneProviderCapability{
	"minimax": {
		Enabled:            true,
		RequiresTranscript: false,
		MinTextLen:         0,
		MaxTextLen:         0,
		SupportedLangs:     map[string]bool{},
	},
	"cosyvoice": {
		Enabled:            true,
		RequiresTranscript: true,
		MinTextLen:         1,
		MaxTextLen:         0,
		SupportedLangs:     map[string]bool{},
	},
	"aliyun_qwen": {
		Enabled:            true,
		RequiresTranscript: false,
		MinTextLen:         0,
		MaxTextLen:         0,
		SupportedLangs:     map[string]bool{},
	},
}

type VoiceCloneController struct {
	DB           *gorm.DB
	AudioStorage *storage.AudioStorage
	HTTPClient   *http.Client
	taskQueue    chan uint
}

type minimaxVoiceCloneResult struct {
	VoiceID      string
	TargetModel  string
	RawResponse  map[string]any
	RequestID    string
	ResponseCode int
}

const (
	defaultMinimaxCloneEndpoint  = "https://api.minimaxi.com/v1/voice_clone"
	defaultMinimaxUploadEndpoint = "https://api.minimaxi.com/v1/files/upload"
	defaultMinimaxCloneModel     = "speech-2.5-hd-preview"
	minMinimaxCloneAudioSeconds  = 10.0

	defaultAliyunQwenCloneEndpoint     = "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/customization"
	defaultAliyunQwenCloneEndpointIntl = "https://dashscope-intl.aliyuncs.com/api/v1/services/audio/tts/customization"
	defaultAliyunQwenCloneModel        = "qwen-voice-enrollment"
	defaultAliyunQwenCloneTargetModel  = "qwen3-tts-vc-2026-01-22"
	defaultAliyunQwenTTSEndpoint       = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultAliyunQwenTTSEndpointIntl   = "https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	maxAliyunQwenCloneAudioBytes       = 10 * 1024 * 1024
	maxAliyunQwenCloneAudioSeconds     = 60.0

	cosyvoiceCloneEndpoint = "https://tts.linkerai.cn/clone"
	cosyvoiceTTSEndpoint   = "https://tts.linkerai.cn/tts"
	cosyvoiceFixedKey      = "https://linkerai.top/"
	minimaxTTSWSEndpoint   = "wss://api.minimaxi.com/ws/v1/t2a_v2"
	voiceClonePreviewText  = "我是一个有趣的人，一个脱离低级趣味的人"

	voiceCloneStatusQueued     = "queued"
	voiceCloneStatusProcessing = "processing"
	voiceCloneStatusActive     = "active"
	voiceCloneStatusFailed     = "failed"

	voiceCloneTaskStatusQueued     = "queued"
	voiceCloneTaskStatusProcessing = "processing"
	voiceCloneTaskStatusSucceeded  = "succeeded"
	voiceCloneTaskStatusFailed     = "failed"

	voiceCloneTaskQueueSize    = 128
	voiceCloneTaskWorkerCount  = 2
	voiceCloneTaskProcessLimit = 5 * time.Minute
)

var errVoiceCloneQuotaExceeded = errors.New("voice clone quota exceeded")

func NewVoiceCloneController(db *gorm.DB, cfg *config.Config) *VoiceCloneController {
	controller := &VoiceCloneController{
		DB:           db,
		AudioStorage: storage.NewAudioStorage(cfg.Storage.SpeakerAudioPath, cfg.Storage.MaxFileSize),
		HTTPClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		taskQueue: make(chan uint, voiceCloneTaskQueueSize),
	}
	controller.startVoiceCloneWorkers()
	return controller
}

func (vcc *VoiceCloneController) CreateVoiceClone(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	ttsConfigID := strings.TrimSpace(c.PostForm("tts_config_id"))
	name := strings.TrimSpace(c.PostForm("name"))
	transcript := strings.TrimSpace(c.PostForm("transcript"))
	transcriptLang := strings.TrimSpace(c.DefaultPostForm("transcript_lang", "zh-CN"))
	sourceType := strings.TrimSpace(c.DefaultPostForm("source_type", "upload"))
	if ttsConfigID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tts_config_id不能为空"})
		return
	}
	if sourceType != "upload" && sourceType != "record" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_type仅支持upload或record"})
		return
	}

	var ttsCfg models.Config
	if err := vcc.DB.Where("type = ? AND config_id = ?", "tts", ttsConfigID).First(&ttsCfg).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TTS配置不存在"})
		return
	}
	rawProvider := strings.TrimSpace(ttsCfg.Provider)
	provider := normalizeCloneProvider(rawProvider)
	if provider != "minimax" && provider != "cosyvoice" && provider != "aliyun_qwen" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前仅支持 Minimax/CosyVoice/千问 提供商的声音复刻"})
		return
	}

	capability := GetCloneProviderCapability(provider)
	if !capability.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该提供商暂未开启声音复刻"})
		return
	}
	if capability.RequiresTranscript {
		if transcript == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该提供商复刻要求必须填写音频对应文字"})
			return
		}
		if len([]rune(transcript)) < capability.MinTextLen {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("音频对应文字长度不能少于%d个字符", capability.MinTextLen)})
			return
		}
	}
	if capability.MaxTextLen > 0 && len([]rune(transcript)) > capability.MaxTextLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("音频对应文字长度不能超过%d个字符", capability.MaxTextLen)})
		return
	}
	if len(capability.SupportedLangs) > 0 && !capability.SupportedLangs[transcriptLang] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transcript_lang不受该提供商支持"})
		return
	}

	file, header, err := vcc.pickAudioFile(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	log.Printf("[voice_clone][%s] incoming audio: source_type=%s filename=%q ext=%q content_type=%q header_size=%d",
		rawProvider,
		sourceType,
		header.Filename,
		strings.ToLower(filepath.Ext(header.Filename)),
		header.Header.Get("Content-Type"),
		header.Size,
	)

	if name == "" {
		base := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
		if base == "" {
			base = "voice-clone"
		}
		name = base
	}

	audioUUID := uuid.New().String()
	filePath, size, err := vcc.AudioStorage.SaveVoiceCloneAudioFile(userID, audioUUID, header.Filename, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存复刻音频失败: " + err.Error()})
		return
	}
	if err = validateCloneAudioForProvider(provider, filePath); err != nil {
		_ = vcc.AudioStorage.DeleteAudioFile(filePath)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskID := uuid.New().String()
	providerVoiceID := buildMinimaxCustomVoiceID(ttsConfigID)
	pendingMetaJSON, _ := json.Marshal(gin.H{
		"source_type": sourceType,
		"task_id":     taskID,
		"task_status": voiceCloneTaskStatusQueued,
		"queued_at":   time.Now(),
	})

	clone := models.VoiceClone{
		UserID:             userID,
		Name:               name,
		Provider:           rawProvider,
		ProviderVoiceID:    providerVoiceID,
		TTSConfigID:        ttsConfigID,
		Status:             voiceCloneStatusProcessing,
		TranscriptRequired: capability.RequiresTranscript,
		MetaJSON:           string(pendingMetaJSON),
	}
	audio := models.VoiceCloneAudio{
		UserID:         userID,
		SourceType:     sourceType,
		FilePath:       filePath,
		FileName:       header.Filename,
		FileSize:       size,
		ContentType:    header.Header.Get("Content-Type"),
		Transcript:     transcript,
		TranscriptLang: transcriptLang,
	}

	task := models.VoiceCloneTask{
		TaskID:    taskID,
		UserID:    userID,
		Provider:  rawProvider,
		Status:    voiceCloneTaskStatusQueued,
		Attempts:  0,
		LastError: "",
	}

	err = vcc.DB.Transaction(func(tx *gorm.DB) error {
		if err = vcc.consumeVoiceCloneQuota(tx, userID, ttsConfigID); err != nil {
			return err
		}
		if err = tx.Create(&clone).Error; err != nil {
			return err
		}
		audio.VoiceCloneID = &clone.ID
		if err = tx.Create(&audio).Error; err != nil {
			return err
		}
		task.VoiceCloneID = clone.ID
		if err = tx.Create(&task).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = vcc.AudioStorage.DeleteAudioFile(filePath)
		if errors.Is(err, errVoiceCloneQuotaExceeded) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该 TTS 配置的声音复刻次数已用完，请联系管理员分配额度"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建复刻任务失败: " + err.Error()})
		return
	}

	vcc.enqueueVoiceCloneTask(task.ID)
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{
		"id": clone.ID, "name": clone.Name, "provider": clone.Provider,
		"provider_voice_id": clone.ProviderVoiceID, "tts_config_id": clone.TTSConfigID,
		"audio_id": audio.ID, "created_at": clone.CreatedAt, "status": clone.Status,
		"task_id": task.TaskID, "task_status": task.Status,
	}})
}

func (vcc *VoiceCloneController) GetVoiceClones(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	ttsConfigID := strings.TrimSpace(c.Query("tts_config_id"))
	query := vcc.DB.Model(&models.VoiceClone{}).Where("user_id = ? AND status != ?", userID, "deleted")
	if ttsConfigID != "" {
		query = query.Where("tts_config_id = ?", ttsConfigID)
	}

	var clones []models.VoiceClone
	if err := query.Order("created_at DESC").Find(&clones).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复刻音色失败"})
		return
	}
	if len(clones) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
		return
	}

	cloneIDs := make([]uint, 0, len(clones))
	ttsConfigIDSet := make(map[string]bool, len(clones))
	for _, clone := range clones {
		cloneIDs = append(cloneIDs, clone.ID)
		if strings.TrimSpace(clone.TTSConfigID) != "" {
			ttsConfigIDSet[clone.TTSConfigID] = true
		}
	}

	var tasks []models.VoiceCloneTask
	if err := vcc.DB.Where("user_id = ? AND voice_clone_id IN ?", userID, cloneIDs).Order("created_at DESC").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复刻任务失败"})
		return
	}
	latestTaskByCloneID := make(map[uint]models.VoiceCloneTask, len(clones))
	for _, task := range tasks {
		if _, exists := latestTaskByCloneID[task.VoiceCloneID]; exists {
			continue
		}
		latestTaskByCloneID[task.VoiceCloneID] = task
	}

	ttsConfigNames := make(map[string]string, len(ttsConfigIDSet))
	ttsConfigProviders := make(map[string]string, len(ttsConfigIDSet))
	if len(ttsConfigIDSet) > 0 {
		ttsConfigIDs := make([]string, 0, len(ttsConfigIDSet))
		for configID := range ttsConfigIDSet {
			ttsConfigIDs = append(ttsConfigIDs, configID)
		}
		var ttsConfigs []models.Config
		if err := vcc.DB.Where("type = ? AND config_id IN ?", "tts", ttsConfigIDs).Find(&ttsConfigs).Error; err == nil {
			for _, ttsConfig := range ttsConfigs {
				ttsConfigNames[ttsConfig.ConfigID] = strings.TrimSpace(ttsConfig.Name)
				ttsConfigProviders[ttsConfig.ConfigID] = strings.TrimSpace(ttsConfig.Provider)
			}
		}
	}

	result := make([]gin.H, 0, len(clones))
	for _, clone := range clones {
		item := gin.H{
			"id":                  clone.ID,
			"user_id":             clone.UserID,
			"name":                clone.Name,
			"provider":            clone.Provider,
			"provider_voice_id":   clone.ProviderVoiceID,
			"tts_config_id":       clone.TTSConfigID,
			"tts_config_name":     clone.TTSConfigID,
			"status":              clone.Status,
			"transcript_required": clone.TranscriptRequired,
			"meta_json":           clone.MetaJSON,
			"created_at":          clone.CreatedAt,
			"updated_at":          clone.UpdatedAt,
		}
		if name, ok := ttsConfigNames[clone.TTSConfigID]; ok && name != "" {
			item["tts_config_name"] = name
		}
		if provider, ok := ttsConfigProviders[clone.TTSConfigID]; ok && provider != "" {
			item["tts_provider"] = provider
		}
		if task, ok := latestTaskByCloneID[clone.ID]; ok {
			item["task_id"] = task.TaskID
			item["task_status"] = task.Status
			item["task_attempts"] = task.Attempts
			item["task_last_error"] = task.LastError
			item["task_started_at"] = task.StartedAt
			item["task_finished_at"] = task.FinishedAt
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (vcc *VoiceCloneController) UpdateVoiceClone(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	cloneID := strings.TrimSpace(c.Param("id"))
	if cloneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "复刻音色ID不能为空"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称不能为空"})
		return
	}
	if len([]rune(name)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称长度不能超过100个字符"})
		return
	}

	var clone models.VoiceClone
	if err := vcc.DB.Where("id = ? AND user_id = ? AND status != ?", cloneID, userID, "deleted").First(&clone).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "复刻音色不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复刻音色失败"})
		return
	}

	if err := vcc.DB.Model(&clone).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新复刻名称失败"})
		return
	}
	clone.Name = name
	c.JSON(http.StatusOK, gin.H{"success": true, "data": clone})
}

func (vcc *VoiceCloneController) RetryVoiceClone(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	cloneID := strings.TrimSpace(c.Param("id"))
	if cloneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "复刻音色ID不能为空"})
		return
	}

	var clone models.VoiceClone
	var task models.VoiceCloneTask
	now := time.Now()
	err := vcc.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ? AND status != ?", cloneID, userID, "deleted").First(&clone).Error; err != nil {
			return err
		}
		if err := tx.Where("voice_clone_id = ? AND user_id = ?", clone.ID, userID).
			Order("created_at DESC, id DESC").
			First(&task).Error; err != nil {
			return err
		}
		if strings.TrimSpace(strings.ToLower(task.Status)) != voiceCloneTaskStatusFailed {
			return fmt.Errorf("当前任务状态为 %s，仅失败任务允许重新复刻", task.Status)
		}

		cloneMetaJSON := mergeJSONMeta(clone.MetaJSON, map[string]any{
			"task_id":     task.TaskID,
			"task_status": voiceCloneTaskStatusQueued,
			"queued_at":   now,
			"retry_at":    now,
			"last_error":  "",
		})
		updateClone := tx.Model(&models.VoiceClone{}).
			Where("id = ? AND user_id = ?", clone.ID, userID).
			Updates(map[string]any{
				"status":    voiceCloneStatusProcessing,
				"meta_json": cloneMetaJSON,
			})
		if updateClone.Error != nil {
			return updateClone.Error
		}
		updateTask := tx.Model(&models.VoiceCloneTask{}).
			Where("id = ? AND status = ?", task.ID, voiceCloneTaskStatusFailed).
			Updates(map[string]any{
				"status":      voiceCloneTaskStatusQueued,
				"last_error":  "",
				"started_at":  nil,
				"finished_at": nil,
			})
		if updateTask.Error != nil {
			return updateTask.Error
		}
		if updateTask.RowsAffected == 0 {
			return errors.New("任务状态已变更，请刷新后重试")
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "复刻任务不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vcc.enqueueVoiceCloneTask(task.ID)
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{
		"id":          clone.ID,
		"task_id":     task.TaskID,
		"task_status": voiceCloneTaskStatusQueued,
		"status":      voiceCloneStatusProcessing,
		"retry_at":    now,
	}})
}

func (vcc *VoiceCloneController) PreviewClonedVoice(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	cloneID := strings.TrimSpace(c.Param("id"))
	if cloneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "复刻音色ID不能为空"})
		return
	}

	var clone models.VoiceClone
	if err := vcc.DB.Where("id = ? AND user_id = ? AND status != ?", cloneID, userID, "deleted").First(&clone).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "复刻音色不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复刻音色失败"})
		return
	}
	if normalizeCloneStatusValue(clone.Status) != voiceCloneStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅已成功的复刻音色允许试听"})
		return
	}
	voiceID := strings.TrimSpace(clone.ProviderVoiceID)
	if voiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "复刻音色ID为空，无法试听"})
		return
	}

	var ttsCfg models.Config
	if err := vcc.DB.Where("type = ? AND config_id = ?", "tts", clone.TTSConfigID).First(&ttsCfg).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "关联TTS配置不存在"})
		return
	}
	provider := normalizeCloneProvider(ttsCfg.Provider)

	cfgMap := make(map[string]any)
	if strings.TrimSpace(ttsCfg.JsonData) != "" {
		if err := json.Unmarshal([]byte(ttsCfg.JsonData), &cfgMap); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解析TTS配置失败"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	var (
		audioBytes  []byte
		contentType string
		err         error
	)
	switch provider {
	case "minimax":
		audioBytes, contentType, err = vcc.previewMinimaxClonedVoice(ctx, cfgMap, voiceID, voiceClonePreviewText)
	case "cosyvoice":
		audioBytes, contentType, err = vcc.previewCosyVoiceClonedVoice(ctx, cfgMap, voiceID, voiceClonePreviewText)
	case "aliyun_qwen":
		audioBytes, contentType, err = vcc.previewAliyunQwenClonedVoice(ctx, cfgMap, voiceID, voiceClonePreviewText)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前提供商不支持复刻试听"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "生成试听音频失败: " + err.Error()})
		return
	}
	if len(audioBytes) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "生成试听音频失败: 返回音频为空"})
		return
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/mpeg"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"voice_clone_preview_%d\"", clone.ID))
	c.Data(http.StatusOK, contentType, audioBytes)
}

func (vcc *VoiceCloneController) GetVoiceCloneAudios(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	cloneID := strings.TrimSpace(c.Param("id"))
	var clone models.VoiceClone
	if err := vcc.DB.Where("id = ? AND user_id = ?", cloneID, userID).First(&clone).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "复刻音色不存在"})
		return
	}

	var audios []models.VoiceCloneAudio
	if err := vcc.DB.Where("voice_clone_id = ? AND user_id = ?", clone.ID, userID).Order("created_at DESC").Find(&audios).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询复刻音频失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": audios})
}

func (vcc *VoiceCloneController) GetVoiceCloneAudioFile(c *gin.Context) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证信息缺失"})
		return
	}
	userID := userIDAny.(uint)

	audioID := strings.TrimSpace(c.Param("audio_id"))
	var audio models.VoiceCloneAudio
	if err := vcc.DB.Where("id = ? AND user_id = ?", audioID, userID).First(&audio).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "复刻音频不存在"})
		return
	}
	if !vcc.AudioStorage.FileExists(audio.FilePath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "音频文件不存在"})
		return
	}

	contentType := audio.ContentType
	if contentType == "" {
		contentType = "audio/wav"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", audio.FileName))
	c.File(audio.FilePath)
}

func (vcc *VoiceCloneController) GetCloneProviderCapabilities(c *gin.Context) {
	provider := strings.TrimSpace(c.Query("provider"))
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider参数必填"})
		return
	}
	capability := GetCloneProviderCapability(provider)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"provider":            provider,
		"enabled":             capability.Enabled,
		"requires_transcript": capability.RequiresTranscript,
		"min_text_len":        capability.MinTextLen,
		"max_text_len":        capability.MaxTextLen,
		"supported_langs":     mapsKeys(capability.SupportedLangs),
		"updated_at":          time.Now(),
	}})
}

func (vcc *VoiceCloneController) pickAudioFile(c *gin.Context) (multipart.File, *multipart.FileHeader, error) {
	candidates := []string{"audio_file", "audio_blob", "audio"}
	for _, field := range candidates {
		file, header, err := c.Request.FormFile(field)
		if err == nil {
			return file, header, nil
		}
	}
	return nil, nil, fmt.Errorf("请上传音频文件（audio_file 或 audio_blob）")
}

func GetCloneProviderCapability(provider string) CloneProviderCapability {
	provider = normalizeCloneProvider(provider)
	if capability, ok := cloneProviderCapabilities[provider]; ok {
		return capability
	}
	return CloneProviderCapability{Enabled: false, SupportedLangs: map[string]bool{}}
}

func normalizeCloneProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func BuildVoiceOptionForClone(clone models.VoiceClone) VoiceOption {
	label := fmt.Sprintf("[我的复刻] %s (%s)", clone.Name, clone.ProviderVoiceID)
	return VoiceOption{Value: clone.ProviderVoiceID, Label: label}
}

func mapsKeys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k, enabled := range m {
		if enabled {
			result = append(result, k)
		}
	}
	return result
}

func (vcc *VoiceCloneController) consumeVoiceCloneQuota(tx *gorm.DB, userID uint, ttsConfigID string) error {
	ttsConfigID = strings.TrimSpace(ttsConfigID)
	if ttsConfigID == "" {
		return nil
	}

	var quota models.UserVoiceCloneQuota
	err := tx.Where("user_id = ? AND tts_config_id = ?", userID, ttsConfigID).First(&quota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 未配置额度时默认不限制，兼容历史行为
			return nil
		}
		return err
	}

	if quota.MaxCount < 0 {
		return nil
	}
	result := tx.Model(&models.UserVoiceCloneQuota{}).
		Where("id = ? AND max_count >= 0 AND used_count < max_count", quota.ID).
		Update("used_count", gorm.Expr("used_count + ?", 1))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errVoiceCloneQuotaExceeded
	}
	return nil
}

func normalizeCloneStatusValue(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

type minimaxTTSWSMessage struct {
	Event           string                  `json:"event,omitempty"`
	Model           string                  `json:"model,omitempty"`
	VoiceSetting    *minimaxTTSVoiceSetting `json:"voice_setting,omitempty"`
	AudioSetting    *minimaxTTSAudioSetting `json:"audio_setting,omitempty"`
	ContinuousSound bool                    `json:"continuous_sound,omitempty"`
	Text            string                  `json:"text,omitempty"`
}

type minimaxTTSVoiceSetting struct {
	VoiceID              string  `json:"voice_id"`
	Speed                float64 `json:"speed"`
	Vol                  float64 `json:"vol"`
	Pitch                int     `json:"pitch"`
	EnglishNormalization bool    `json:"english_normalization"`
}

type minimaxTTSAudioSetting struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

type minimaxTTSWSResponse struct {
	Event   string `json:"event"`
	IsFinal bool   `json:"is_final"`
	Data    struct {
		Audio string `json:"audio"`
	} `json:"data"`
	BaseResp *struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func (vcc *VoiceCloneController) previewMinimaxClonedVoice(ctx context.Context, cfgMap map[string]any, voiceID, text string) ([]byte, string, error) {
	apiKey := normalizeMinimaxAPIKey(getStringAny(cfgMap, "api_key"))
	if apiKey == "" {
		return nil, "", errors.New("minimax api_key 未配置")
	}
	model := strings.TrimSpace(getStringAny(cfgMap, "model"))
	if model == "" {
		model = "speech-2.8-hd"
	}
	speed, ok := getFloatAny(cfgMap, "speed")
	if !ok || speed <= 0 {
		speed = 1.0
	}
	vol, ok := getFloatAny(cfgMap, "vol", "volume")
	if !ok || vol <= 0 {
		vol = 1.0
	}
	pitch, ok := getIntAny(cfgMap, "pitch")
	if !ok {
		pitch = 0
	}
	groupID := strings.TrimSpace(getStringAny(cfgMap, "group_id", "GroupId"))

	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	if groupID != "" {
		header.Set("Group-Id", groupID)
		header.Set("GroupId", groupID)
	}
	conn, _, err := dialer.DialContext(ctx, minimaxTTSWSEndpoint, header)
	if err != nil {
		return nil, "", fmt.Errorf("连接Minimax语音接口失败: %w", err)
	}
	defer conn.Close()

	startMessage := minimaxTTSWSMessage{
		Event: "task_start",
		Model: model,
		VoiceSetting: &minimaxTTSVoiceSetting{
			VoiceID:              voiceID,
			Speed:                speed,
			Vol:                  vol,
			Pitch:                pitch,
			EnglishNormalization: false,
		},
		AudioSetting: &minimaxTTSAudioSetting{
			SampleRate: 32000,
			Bitrate:    128000,
			Format:     "mp3",
			Channel:    1,
		},
		ContinuousSound: false,
	}
	if err = conn.WriteJSON(startMessage); err != nil {
		return nil, "", fmt.Errorf("发送Minimax task_start失败: %w", err)
	}
	if err = conn.WriteJSON(minimaxTTSWSMessage{Event: "task_continue", Text: text}); err != nil {
		return nil, "", fmt.Errorf("发送Minimax task_continue失败: %w", err)
	}
	if err = conn.WriteJSON(minimaxTTSWSMessage{Event: "task_finish"}); err != nil {
		return nil, "", fmt.Errorf("发送Minimax task_finish失败: %w", err)
	}

	var mergedAudio []byte
	for {
		_, messageBytes, readErr := conn.ReadMessage()
		if readErr != nil {
			if len(mergedAudio) > 0 {
				break
			}
			return nil, "", fmt.Errorf("读取Minimax响应失败: %w", readErr)
		}
		var resp minimaxTTSWSResponse
		if err = json.Unmarshal(messageBytes, &resp); err != nil {
			continue
		}
		if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
			return nil, "", fmt.Errorf("Minimax返回错误(code=%d, msg=%s)", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
		}
		audioHex := strings.TrimSpace(resp.Data.Audio)
		if audioHex != "" {
			chunk, decodeErr := hex.DecodeString(audioHex)
			if decodeErr != nil {
				return nil, "", fmt.Errorf("解析Minimax音频数据失败: %w", decodeErr)
			}
			mergedAudio = append(mergedAudio, chunk...)
		}
		if resp.IsFinal || strings.EqualFold(strings.TrimSpace(resp.Event), "task_finish") {
			break
		}
	}
	if len(mergedAudio) == 0 {
		return nil, "", errors.New("Minimax返回音频为空")
	}
	return mergedAudio, "audio/mpeg", nil
}

func (vcc *VoiceCloneController) previewCosyVoiceClonedVoice(ctx context.Context, cfgMap map[string]any, voiceID, text string) ([]byte, string, error) {
	endpoint := strings.TrimSpace(getStringAny(cfgMap, "api_url", "tts_endpoint"))
	if endpoint == "" {
		endpoint = cosyvoiceTTSEndpoint
	}
	query := url.Values{}
	query.Set("tts_text", text)
	query.Set("spk_id", voiceID)
	query.Set("frame_durition", "60")
	query.Set("stream", "true")
	query.Set("target_sr", "24000")
	query.Set("audio_format", "mp3")
	if instructText := strings.TrimSpace(getStringAny(cfgMap, "instruct_text")); instructText != "" {
		query.Set("instruct_text", instructText)
	}
	requestURL := endpoint + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建CosyVoice试听请求失败: %w", err)
	}
	req.Header.Set("Accept", "audio/mpeg,application/octet-stream,*/*")

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("调用CosyVoice试听失败: %w", err)
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("读取CosyVoice试听响应失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("CosyVoice试听HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(audioBytes)))
	}
	if len(audioBytes) == 0 {
		return nil, "", errors.New("CosyVoice返回音频为空")
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" || strings.Contains(strings.ToLower(contentType), "application/json") {
		contentType = "audio/mpeg"
	}
	return audioBytes, contentType, nil
}

func (vcc *VoiceCloneController) previewAliyunQwenClonedVoice(ctx context.Context, cfgMap map[string]any, voiceID, text string) ([]byte, string, error) {
	apiKey := strings.TrimSpace(getStringAny(cfgMap, "api_key"))
	if apiKey == "" {
		return nil, "", errors.New("aliyun_qwen api_key 未配置")
	}
	endpoint := resolveAliyunQwenTTSEndpoint(cfgMap)
	languageType := strings.TrimSpace(getStringAny(cfgMap, "language_type"))
	if languageType == "" {
		languageType = "Chinese"
	}
	reqBody := map[string]any{
		"model": defaultAliyunQwenCloneTargetModel,
		"input": map[string]any{
			"text":          text,
			"voice":         voiceID,
			"language_type": languageType,
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("构建千问试听请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("创建千问试听请求失败: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("调用千问试听失败: %w", err)
	}
	defer resp.Body.Close()

	ttsRespBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("读取千问试听响应失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("千问试听HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(ttsRespBody)))
	}

	parsed, err := unmarshalJSONMap(ttsRespBody)
	if err != nil {
		return nil, "", fmt.Errorf("解析千问试听响应失败: %w", err)
	}
	if code := strings.TrimSpace(getStringAny(parsed, "code")); code != "" {
		return nil, "", fmt.Errorf("千问试听失败(code=%s, msg=%s)", code, strings.TrimSpace(getStringAny(parsed, "message")))
	}
	if statusCode, ok := getIntAny(parsed, "status_code"); ok && statusCode != 200 {
		return nil, "", fmt.Errorf("千问试听失败(status_code=%d)", statusCode)
	}

	output, ok := parsed["output"].(map[string]any)
	if !ok {
		return nil, "", errors.New("千问试听响应缺少 output")
	}
	audioOutput, ok := output["audio"].(map[string]any)
	if !ok {
		return nil, "", errors.New("千问试听响应缺少 output.audio")
	}
	audioURL := strings.TrimSpace(getStringAny(audioOutput, "url"))
	if audioURL == "" {
		return nil, "", errors.New("千问试听响应缺少 output.audio.url")
	}

	audioReq, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建千问音频下载请求失败: %w", err)
	}
	audioResp, err := vcc.HTTPClient.Do(audioReq)
	if err != nil {
		return nil, "", fmt.Errorf("下载千问试听音频失败: %w", err)
	}
	defer audioResp.Body.Close()
	audioBytes, err := io.ReadAll(io.LimitReader(audioResp.Body, 20*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("读取千问试听音频失败: %w", err)
	}
	if audioResp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("下载千问试听音频HTTP %d: %s", audioResp.StatusCode, strings.TrimSpace(string(audioBytes)))
	}
	if len(audioBytes) == 0 {
		return nil, "", errors.New("千问试听返回音频为空")
	}
	contentType := strings.TrimSpace(audioResp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "audio/wav"
	}
	return audioBytes, contentType, nil
}

func (vcc *VoiceCloneController) cloneWithMinimax(ctx context.Context, ttsCfg models.Config, ttsConfigID, filePath, fileName, transcript string) (*minimaxVoiceCloneResult, error) {
	cfgMap := make(map[string]any)
	if ttsCfg.JsonData != "" {
		if err := json.Unmarshal([]byte(ttsCfg.JsonData), &cfgMap); err != nil {
			return nil, fmt.Errorf("解析TTS配置失败: %w", err)
		}
	}
	apiKey := normalizeMinimaxAPIKey(getStringAny(cfgMap, "api_key"))
	if apiKey == "" {
		return nil, errors.New("minimax api_key 未配置")
	}
	endpoint := strings.TrimSpace(getStringAny(cfgMap, "voice_clone_endpoint", "clone_endpoint"))
	if endpoint == "" {
		endpoint = defaultMinimaxCloneEndpoint
	}
	uploadEndpoint := strings.TrimSpace(getStringAny(cfgMap, "voice_clone_upload_endpoint", "files_upload_endpoint", "file_upload_endpoint"))
	if uploadEndpoint == "" {
		uploadEndpoint = defaultMinimaxUploadEndpoint
	}
	model := strings.TrimSpace(getStringAny(cfgMap, "voice_clone_model", "voice_clone_model_id", "model"))
	if model == "" {
		model = defaultMinimaxCloneModel
	}
	voiceID := buildMinimaxCustomVoiceID(ttsConfigID)
	groupID := strings.TrimSpace(getStringAny(cfgMap, "group_id", "GroupId"))
	log.Printf("[voice_clone][minimax] prepare request: upload_endpoint=%s clone_endpoint=%s model=%q voice_id=%q transcript_len=%d group_id=%q file_name=%q file_path=%q api_key=%s",
		uploadEndpoint,
		endpoint,
		model,
		voiceID,
		len([]rune(strings.TrimSpace(transcript))),
		groupID,
		fileName,
		filePath,
		maskSecret(apiKey),
	)
	return vcc.cloneWithMinimaxEndpoints(ctx, apiKey, endpoint, uploadEndpoint, groupID, filePath, fileName, transcript, model, voiceID)
}

func (vcc *VoiceCloneController) cloneWithMinimaxEndpoints(ctx context.Context, apiKey, cloneEndpoint, uploadEndpoint, groupID, filePath, fileName, transcript, model, voiceID string) (*minimaxVoiceCloneResult, error) {
	fileID, err := vcc.uploadMinimaxVoiceCloneFile(ctx, apiKey, uploadEndpoint, groupID, filePath, fileName)
	if err != nil {
		return nil, err
	}
	fileIDPayload := makeMinimaxFileIDPayload(fileID)

	bodyMap := map[string]any{
		"file_id":  fileIDPayload,
		"voice_id": voiceID,
	}
	transcript = strings.TrimSpace(transcript)
	if transcript != "" {
		bodyMap["text"] = transcript
		if model != "" {
			bodyMap["model"] = model
		}
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("构建Minimax复刻请求失败: %w", err)
	}
	log.Printf("[voice_clone][minimax] clone request: endpoint=%s file_id_type=%T body=%s group_id=%q api_key=%s",
		cloneEndpoint,
		fileIDPayload,
		truncateForLog(string(bodyBytes), 1024),
		groupID,
		maskSecret(apiKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloneEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if groupID != "" {
		req.Header.Set("Group-Id", groupID)
		req.Header.Set("GroupId", groupID)
	}

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	log.Printf("[voice_clone][minimax] clone response: status=%d body=%s",
		resp.StatusCode,
		truncateForLog(strings.TrimSpace(string(respBody)), 4096),
	)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	parsed, err := unmarshalJSONMap(respBody)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if statusCode, statusMsg, ok := parseMinimaxStatus(parsed); ok && statusCode != 0 {
		return nil, fmt.Errorf("Minimax返回错误(code=%d, msg=%s): %s", statusCode, statusMsg, strings.TrimSpace(string(respBody)))
	}

	resolvedVoiceID := strings.TrimSpace(voiceID)
	if resolvedVoiceID == "" {
		return nil, errors.New("请求 voice_id 为空")
	}
	if payloadVoiceID := pickVoiceID(parsed); payloadVoiceID != "" && payloadVoiceID != resolvedVoiceID {
		log.Printf("[voice_clone][minimax] clone response voice_id=%q ignored, using requested voice_id=%q", payloadVoiceID, resolvedVoiceID)
	}
	if pickVoiceID(parsed) == "" {
		log.Printf("[voice_clone][minimax] clone response missing voice_id, using requested voice_id=%q", resolvedVoiceID)
	}
	return &minimaxVoiceCloneResult{
		VoiceID:      resolvedVoiceID,
		RawResponse:  parsed,
		RequestID:    getStringAny(parsed, "request_id", "trace_id"),
		ResponseCode: resp.StatusCode,
	}, nil
}

func (vcc *VoiceCloneController) cloneWithCosyVoice(ctx context.Context, filePath, fileName, transcript string) (*minimaxVoiceCloneResult, error) {
	cloneURL, err := url.Parse(cosyvoiceCloneEndpoint)
	if err != nil {
		return nil, fmt.Errorf("解析CosyVoice克隆地址失败: %w", err)
	}
	query := cloneURL.Query()
	query.Set("key", cosyvoiceFixedKey)
	cloneURL.RawQuery = query.Encode()

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取音频文件失败: %w", err)
	}
	defer f.Close()

	fileSize := int64(-1)
	if stat, statErr := f.Stat(); statErr == nil {
		fileSize = stat.Size()
	}
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return nil, errors.New("CosyVoice 复刻要求必须填写音频对应文字(train_text)")
	}
	log.Printf("[voice_clone][cosyvoice] prepare request: endpoint=%s file_name=%q file_ext=%q file_size=%d transcript_len=%d fixed_key=%q",
		cloneURL.String(),
		fileName,
		strings.ToLower(filepath.Ext(fileName)),
		fileSize,
		len([]rune(transcript)),
		cosyvoiceFixedKey,
	)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("train_text", transcript); err != nil {
		return nil, fmt.Errorf("构建CosyVoice请求参数失败: %w", err)
	}
	formFile, err := writer.CreateFormFile("train_wav_file", fileName)
	if err != nil {
		return nil, fmt.Errorf("创建CosyVoice音频上传表单失败: %w", err)
	}
	if _, err = io.Copy(formFile, f); err != nil {
		return nil, fmt.Errorf("写入CosyVoice上传文件失败: %w", err)
	}
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("构建CosyVoice上传请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloneURL.String(), &body)
	if err != nil {
		return nil, fmt.Errorf("创建CosyVoice请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用CosyVoice克隆接口失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取CosyVoice响应失败: %w", err)
	}
	log.Printf("[voice_clone][cosyvoice] clone response: status=%d body=%s",
		resp.StatusCode,
		truncateForLog(strings.TrimSpace(string(respBody)), 4096),
	)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("CosyVoice克隆HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	parsed, err := unmarshalJSONMap(respBody)
	if err != nil {
		return nil, fmt.Errorf("解析CosyVoice响应失败: %w", err)
	}
	status := strings.TrimSpace(getStringAny(parsed, "status"))
	if status != "成功" {
		return nil, fmt.Errorf("CosyVoice克隆失败(status=%s): %s", status, strings.TrimSpace(string(respBody)))
	}
	sid := strings.TrimSpace(getStringAny(parsed, "sid"))
	if sid == "" {
		return nil, fmt.Errorf("CosyVoice响应缺少 sid: %s", strings.TrimSpace(string(respBody)))
	}
	return &minimaxVoiceCloneResult{
		VoiceID:      sid,
		RawResponse:  parsed,
		RequestID:    getStringAny(parsed, "request_id", "trace_id"),
		ResponseCode: resp.StatusCode,
	}, nil
}

func (vcc *VoiceCloneController) cloneWithAliyunQwen(ctx context.Context, ttsCfg models.Config, ttsConfigID, filePath, fileName, transcript, transcriptLang string) (*minimaxVoiceCloneResult, error) {
	cfgMap := make(map[string]any)
	if ttsCfg.JsonData != "" {
		if err := json.Unmarshal([]byte(ttsCfg.JsonData), &cfgMap); err != nil {
			return nil, fmt.Errorf("解析TTS配置失败: %w", err)
		}
	}

	apiKey := strings.TrimSpace(getStringAny(cfgMap, "api_key"))
	if apiKey == "" {
		return nil, errors.New("aliyun_qwen api_key 未配置")
	}
	endpoint := resolveAliyunQwenCloneEndpoint(cfgMap)
	targetModel := resolveAliyunQwenTargetModel()
	preferredName := buildAliyunQwenPreferredName(ttsConfigID)
	audioData, mimeType, fileSize, err := buildAliyunQwenAudioDataURI(filePath)
	if err != nil {
		return nil, err
	}

	transcript = strings.TrimSpace(transcript)
	language := mapAliyunQwenCloneLanguage(transcriptLang)
	log.Printf("[voice_clone][aliyun_qwen] prepare request: endpoint=%s model=%q target_model=%q preferred_name=%q file_name=%q file_ext=%q file_size=%d mime_type=%q transcript_len=%d language=%q api_key=%s",
		endpoint,
		defaultAliyunQwenCloneModel,
		targetModel,
		preferredName,
		fileName,
		strings.ToLower(filepath.Ext(fileName)),
		fileSize,
		mimeType,
		len([]rune(transcript)),
		language,
		maskSecret(apiKey),
	)

	input := map[string]any{
		"action":         "create",
		"target_model":   targetModel,
		"preferred_name": preferredName,
		"audio": map[string]any{
			"data": audioData,
		},
	}
	if transcript != "" {
		input["text"] = transcript
		if language != "" {
			input["language"] = language
		}
	}
	bodyMap := map[string]any{
		"model": defaultAliyunQwenCloneModel,
		"input": input,
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("构建千问复刻请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建千问复刻请求失败: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用千问复刻接口失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取千问复刻响应失败: %w", err)
	}
	log.Printf("[voice_clone][aliyun_qwen] clone response: status=%d body=%s",
		resp.StatusCode,
		truncateForLog(strings.TrimSpace(string(respBody)), 4096),
	)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("千问复刻HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	parsed, err := unmarshalJSONMap(respBody)
	if err != nil {
		return nil, fmt.Errorf("解析千问复刻响应失败: %w", err)
	}
	if code := strings.TrimSpace(getStringAny(parsed, "code")); code != "" {
		msg := strings.TrimSpace(getStringAny(parsed, "message"))
		return nil, fmt.Errorf("千问复刻失败(code=%s, msg=%s): %s", code, msg, strings.TrimSpace(string(respBody)))
	}

	output, ok := parsed["output"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("千问复刻响应缺少 output: %s", strings.TrimSpace(string(respBody)))
	}
	voiceID := strings.TrimSpace(getStringAny(output, "voice"))
	if voiceID == "" {
		return nil, fmt.Errorf("千问复刻响应缺少 output.voice: %s", strings.TrimSpace(string(respBody)))
	}

	return &minimaxVoiceCloneResult{
		VoiceID:      voiceID,
		TargetModel:  targetModel,
		RawResponse:  parsed,
		RequestID:    getStringAny(parsed, "request_id"),
		ResponseCode: resp.StatusCode,
	}, nil
}

func (vcc *VoiceCloneController) uploadMinimaxVoiceCloneFile(ctx context.Context, apiKey, uploadEndpoint, groupID, filePath, fileName string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("读取音频文件失败: %w", err)
	}
	defer f.Close()
	fileSize := int64(-1)
	if stat, statErr := f.Stat(); statErr == nil {
		fileSize = stat.Size()
	}
	detectedContentType := ""
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
		sniffBuf := make([]byte, 512)
		n, readErr := f.Read(sniffBuf)
		if readErr == nil || readErr == io.EOF {
			if n > 0 {
				detectedContentType = http.DetectContentType(sniffBuf[:n])
			}
		}
		_, _ = f.Seek(0, io.SeekStart)
	}
	log.Printf("[voice_clone][minimax] upload request: endpoint=%s purpose=voice_clone file_name=%q file_ext=%q stored_ext=%q file_size=%d detected_content_type=%q group_id=%q api_key=%s",
		uploadEndpoint,
		fileName,
		strings.ToLower(filepath.Ext(fileName)),
		strings.ToLower(filepath.Ext(filePath)),
		fileSize,
		detectedContentType,
		groupID,
		maskSecret(apiKey),
	)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("purpose", "voice_clone"); err != nil {
		return "", fmt.Errorf("构建上传参数失败: %w", err)
	}
	formFile, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("创建上传文件表单失败: %w", err)
	}
	if _, err = io.Copy(formFile, f); err != nil {
		return "", fmt.Errorf("写入上传文件失败: %w", err)
	}
	if err = writer.Close(); err != nil {
		return "", fmt.Errorf("构建上传请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadEndpoint, &body)
	if err != nil {
		return "", fmt.Errorf("创建上传请求失败: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	if groupID != "" {
		req.Header.Set("Group-Id", groupID)
		req.Header.Set("GroupId", groupID)
	}

	resp, err := vcc.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("上传复刻音频失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("读取上传响应失败: %w", err)
	}
	log.Printf("[voice_clone][minimax] upload response: status=%d body=%s",
		resp.StatusCode,
		truncateForLog(strings.TrimSpace(string(respBody)), 4096),
	)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("上传复刻音频HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	parsed, err := unmarshalJSONMap(respBody)
	if err != nil {
		return "", fmt.Errorf("解析上传响应失败: %w", err)
	}
	if statusCode, statusMsg, ok := parseMinimaxStatus(parsed); ok && statusCode != 0 {
		return "", fmt.Errorf("上传复刻音频被Minimax拒绝(code=%d, msg=%s): %s", statusCode, statusMsg, strings.TrimSpace(string(respBody)))
	}

	fileMap, ok := parsed["file"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("上传响应中未返回 file 对象: %s", strings.TrimSpace(string(respBody)))
	}
	fileID := getStringOrNumberAny(fileMap, "file_id", "fileId", "id")
	if fileID == "" {
		return "", fmt.Errorf("上传响应中未返回 file_id: %s", strings.TrimSpace(string(respBody)))
	}
	return fileID, nil
}

func pickVoiceID(payload map[string]any) string {
	candidates := []string{"voice_id", "voiceId", "voice", "speaker_id", "speakerId"}
	for _, key := range candidates {
		if value := getStringAny(payload, key); value != "" {
			return value
		}
	}
	if data, ok := payload["data"].(map[string]any); ok {
		for _, key := range candidates {
			if value := getStringAny(data, key); value != "" {
				return value
			}
		}
	}
	return ""
}

func parseMinimaxStatus(payload map[string]any) (int, string, bool) {
	baseResp, ok := payload["base_resp"].(map[string]any)
	if !ok {
		return 0, "", false
	}
	code, ok := getIntAny(baseResp, "status_code")
	if !ok {
		return 0, "", false
	}
	return code, strings.TrimSpace(getStringAny(baseResp, "status_msg")), true
}

func normalizeMinimaxAPIKey(raw string) string {
	key := strings.TrimSpace(strings.Trim(raw, "\"'"))
	if key == "" {
		return ""
	}
	lowerKey := strings.ToLower(key)
	if strings.HasPrefix(lowerKey, "bearer ") {
		key = strings.TrimSpace(key[len("bearer "):])
	}
	return strings.TrimSpace(strings.Trim(key, "\"'"))
}

func maskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "<empty>"
	}
	if len(secret) <= 8 {
		return fmt.Sprintf("%s(len=%d)", strings.Repeat("*", len(secret)), len(secret))
	}
	return fmt.Sprintf("%s...%s(len=%d)", secret[:4], secret[len(secret)-4:], len(secret))
}

func getMinimaxCloneAudioDurationSeconds(filePath string) (float64, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))
	if ext != ".wav" {
		return 0, fmt.Errorf("当前仅支持 WAV 音频，检测到扩展名: %s", ext)
	}
	return getWAVDurationSeconds(filePath)
}

func validateCloneAudioForProvider(provider, filePath string) error {
	provider = strings.TrimSpace(provider)
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))

	switch provider {
	case "minimax":
		if ext != ".wav" {
			return fmt.Errorf("Minimax 仅支持 WAV 音频，检测到扩展名: %s", ext)
		}
		audioSeconds, err := getMinimaxCloneAudioDurationSeconds(filePath)
		if err != nil {
			return fmt.Errorf("音频格式校验失败: %w", err)
		}
		log.Printf("[voice_clone][minimax] local duration check: file=%q duration=%.3fs min=%.1fs", filePath, audioSeconds, minMinimaxCloneAudioSeconds)
		if audioSeconds < minMinimaxCloneAudioSeconds {
			return fmt.Errorf("Minimax 声音复刻要求音频时长至少 %.0f 秒，当前 %.2f 秒", minMinimaxCloneAudioSeconds, audioSeconds)
		}
		return nil
	case "cosyvoice":
		if ext != ".wav" {
			return fmt.Errorf("CosyVoice 仅支持 WAV 音频，检测到扩展名: %s", ext)
		}
		audioSeconds, err := getWAVDurationSeconds(filePath)
		if err != nil {
			return fmt.Errorf("音频格式校验失败: %w", err)
		}
		log.Printf("[voice_clone][cosyvoice] local wav check: file=%q duration=%.3fs", filePath, audioSeconds)
		return nil
	case "aliyun_qwen":
		mimeType, supported := aliyunQwenCloneAudioMimeTypeByExt(ext)
		if !supported {
			return fmt.Errorf("千问声音复刻仅支持 WAV/MP3/M4A，检测到扩展名: %s", ext)
		}
		stat, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("读取音频文件信息失败: %w", err)
		}
		if stat.Size() <= 0 {
			return errors.New("音频文件不能为空")
		}
		if stat.Size() > maxAliyunQwenCloneAudioBytes {
			return fmt.Errorf("千问声音复刻音频大小不能超过%dMB，当前%.2fMB", maxAliyunQwenCloneAudioBytes/1024/1024, float64(stat.Size())/1024.0/1024.0)
		}
		if ext == ".wav" {
			audioSeconds, err := getWAVDurationSeconds(filePath)
			if err != nil {
				return fmt.Errorf("音频格式校验失败: %w", err)
			}
			log.Printf("[voice_clone][aliyun_qwen] local wav check: file=%q duration=%.3fs max=%.1fs", filePath, audioSeconds, maxAliyunQwenCloneAudioSeconds)
			if audioSeconds > maxAliyunQwenCloneAudioSeconds {
				return fmt.Errorf("千问声音复刻音频时长不能超过 %.0f 秒，当前 %.2f 秒", maxAliyunQwenCloneAudioSeconds, audioSeconds)
			}
		} else {
			log.Printf("[voice_clone][aliyun_qwen] local audio check: file=%q ext=%q size=%d mime=%q", filePath, ext, stat.Size(), mimeType)
		}
		return nil
	default:
		return fmt.Errorf("暂不支持提供商 %s 的音频校验", provider)
	}
}

func aliyunQwenCloneAudioMimeTypeByExt(ext string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".wav":
		return "audio/wav", true
	case ".mp3":
		return "audio/mpeg", true
	case ".m4a":
		return "audio/mp4", true
	default:
		return "", false
	}
}

func resolveAliyunQwenCloneEndpoint(cfgMap map[string]any) string {
	if endpoint := strings.TrimSpace(getStringAny(cfgMap, "voice_clone_endpoint", "clone_endpoint", "customization_endpoint")); endpoint != "" {
		return endpoint
	}
	apiURL := strings.ToLower(strings.TrimSpace(getStringAny(cfgMap, "api_url")))
	if strings.Contains(apiURL, "dashscope-intl.aliyuncs.com") {
		return defaultAliyunQwenCloneEndpointIntl
	}
	return defaultAliyunQwenCloneEndpoint
}

func resolveAliyunQwenTTSEndpoint(cfgMap map[string]any) string {
	if endpoint := strings.TrimSpace(getStringAny(cfgMap, "api_url", "tts_endpoint")); endpoint != "" {
		return endpoint
	}
	if strings.EqualFold(strings.TrimSpace(getStringAny(cfgMap, "region")), "singapore") {
		return defaultAliyunQwenTTSEndpointIntl
	}
	return defaultAliyunQwenTTSEndpoint
}

func resolveAliyunQwenTargetModel() string {
	// 按接入规范固定使用 VC-Realtime 目标模型，避免受到普通 TTS 配置 model 干扰。
	return defaultAliyunQwenCloneTargetModel
}

func buildAliyunQwenPreferredName(ttsConfigID string) string {
	name := sanitizeMinimaxVoiceIDPrefix(ttsConfigID)
	if name == "" {
		name = "voiceclone"
	}
	if len(name) > 16 {
		name = name[:16]
	}
	first := name[0]
	if first >= '0' && first <= '9' {
		name = "vc_" + name
		if len(name) > 16 {
			name = name[:16]
		}
	}
	return name
}

func mapAliyunQwenCloneLanguage(transcriptLang string) string {
	lang := strings.ToLower(strings.TrimSpace(transcriptLang))
	switch lang {
	case "zh", "zh-cn", "zh-hans", "zh-hant", "zh-tw", "zh-hk":
		return "zh"
	case "en", "en-us", "en-gb":
		return "en"
	case "de", "de-de":
		return "de"
	case "it", "it-it":
		return "it"
	case "pt", "pt-pt", "pt-br":
		return "pt"
	case "es", "es-es", "es-mx":
		return "es"
	case "ja", "ja-jp":
		return "ja"
	case "ko", "ko-kr":
		return "ko"
	case "fr", "fr-fr":
		return "fr"
	case "ru", "ru-ru":
		return "ru"
	default:
		if len(lang) >= 2 {
			short := lang[:2]
			if short == "zh" || short == "en" || short == "de" || short == "it" || short == "pt" || short == "es" || short == "ja" || short == "ko" || short == "fr" || short == "ru" {
				return short
			}
		}
		return ""
	}
}

func buildAliyunQwenAudioDataURI(filePath string) (string, string, int64, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))
	mimeType, supported := aliyunQwenCloneAudioMimeTypeByExt(ext)
	if !supported {
		return "", "", 0, fmt.Errorf("千问声音复刻仅支持 WAV/MP3/M4A，检测到扩展名: %s", ext)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("读取音频文件失败: %w", err)
	}
	if len(data) == 0 {
		return "", "", 0, errors.New("音频文件不能为空")
	}
	if len(data) > maxAliyunQwenCloneAudioBytes {
		return "", "", 0, fmt.Errorf("千问声音复刻音频大小不能超过%dMB，当前%.2fMB", maxAliyunQwenCloneAudioBytes/1024/1024, float64(len(data))/1024.0/1024.0)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), mimeType, int64(len(data)), nil
}

func getWAVDurationSeconds(filePath string) (float64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer f.Close()

	header := make([]byte, 12)
	if _, err = io.ReadFull(f, header); err != nil {
		return 0, fmt.Errorf("读取WAV头失败: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, errors.New("不是有效的 WAV 文件")
	}

	var sampleRate uint32
	var channels uint16
	var bitsPerSample uint16
	var dataBytes uint64

	for {
		chunkHeader := make([]byte, 8)
		_, err = io.ReadFull(f, chunkHeader)
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return 0, fmt.Errorf("读取WAV分块头失败: %w", err)
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])
		chunkSizeInt := int64(chunkSize)

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return 0, fmt.Errorf("WAV fmt 分块长度无效: %d", chunkSize)
			}
			fmtData := make([]byte, chunkSize)
			if _, err = io.ReadFull(f, fmtData); err != nil {
				return 0, fmt.Errorf("读取WAV fmt分块失败: %w", err)
			}
			audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 && audioFormat != 3 {
				return 0, fmt.Errorf("不支持的 WAV 编码格式: %d", audioFormat)
			}
			channels = binary.LittleEndian.Uint16(fmtData[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtData[14:16])
		case "data":
			dataBytes = uint64(chunkSize)
			if _, err = f.Seek(chunkSizeInt, io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("跳过WAV data分块失败: %w", err)
			}
		default:
			if _, err = f.Seek(chunkSizeInt, io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("跳过WAV分块失败: %w", err)
			}
		}

		// WAV chunk 数据按 2 字节对齐，奇数长度需补 1 字节。
		if chunkSize%2 == 1 {
			if _, err = f.Seek(1, io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("跳过WAV对齐字节失败: %w", err)
			}
		}
	}

	if sampleRate == 0 || channels == 0 || bitsPerSample == 0 || dataBytes == 0 {
		return 0, fmt.Errorf("WAV信息不完整(sample_rate=%d channels=%d bits=%d data_bytes=%d)", sampleRate, channels, bitsPerSample, dataBytes)
	}
	bytesPerSecond := (float64(sampleRate) * float64(channels) * float64(bitsPerSample)) / 8.0
	if bytesPerSecond <= 0 {
		return 0, errors.New("WAV每秒字节数无效")
	}
	return float64(dataBytes) / bytesPerSecond, nil
}

func buildMinimaxCustomVoiceID(ttsConfigID string) string {
	prefix := sanitizeMinimaxVoiceIDPrefix(ttsConfigID)
	return fmt.Sprintf("%s_%s", prefix, randomDigits(8))
}

func sanitizeMinimaxVoiceIDPrefix(ttsConfigID string) string {
	ttsConfigID = strings.TrimSpace(ttsConfigID)
	if ttsConfigID == "" {
		return "voice"
	}
	filtered := make([]byte, 0, len(ttsConfigID))
	for i := 0; i < len(ttsConfigID); i++ {
		ch := ttsConfigID[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			filtered = append(filtered, ch)
			continue
		}
		filtered = append(filtered, '_')
	}
	prefix := strings.Trim(strings.TrimSpace(string(filtered)), "_")
	if prefix == "" {
		return "voice"
	}
	return prefix
}

func randomDigits(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(crand.Reader, buf); err == nil {
		for i := range buf {
			buf[i] = '0' + (buf[i] % 10)
		}
		return string(buf)
	}
	fallback := fmt.Sprintf("%d", time.Now().UnixNano())
	if len(fallback) >= n {
		return fallback[len(fallback)-n:]
	}
	if len(fallback) == 0 {
		return strings.Repeat("0", n)
	}
	return strings.Repeat("0", n-len(fallback)) + fallback
}

func getStringAny(m map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		if value, ok := raw.(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func getStringOrNumberAny(m map[string]any, keys ...string) string {
	if value := getStringAny(m, keys...); value != "" {
		return value
	}
	for _, key := range keys {
		raw, ok := m[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case json.Number:
			value = json.Number(strings.TrimSpace(string(value)))
			if value == "" {
				continue
			}
			return value.String()
		case int:
			return strconv.Itoa(value)
		case int8:
			return strconv.FormatInt(int64(value), 10)
		case int16:
			return strconv.FormatInt(int64(value), 10)
		case int32:
			return strconv.FormatInt(int64(value), 10)
		case int64:
			return strconv.FormatInt(value, 10)
		case uint:
			return strconv.FormatUint(uint64(value), 10)
		case uint8:
			return strconv.FormatUint(uint64(value), 10)
		case uint16:
			return strconv.FormatUint(uint64(value), 10)
		case uint32:
			return strconv.FormatUint(uint64(value), 10)
		case uint64:
			return strconv.FormatUint(value, 10)
		case float32:
			return strconv.FormatFloat(float64(value), 'f', -1, 32)
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func makeMinimaxFileIDPayload(fileID string) any {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return ""
	}
	if _, err := strconv.ParseInt(fileID, 10, 64); err == nil {
		return json.Number(fileID)
	}
	return fileID
}

func unmarshalJSONMap(payload []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var parsed map[string]any
	if err := decoder.Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func getIntAny(m map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case int:
			return value, true
		case int8:
			return int(value), true
		case int16:
			return int(value), true
		case int32:
			return int(value), true
		case int64:
			return int(value), true
		case uint:
			return int(value), true
		case uint8:
			return int(value), true
		case uint16:
			return int(value), true
		case uint32:
			return int(value), true
		case uint64:
			return int(value), true
		case float32:
			return int(value), true
		case float64:
			return int(value), true
		case json.Number:
			n, err := value.Int64()
			if err == nil {
				return int(n), true
			}
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func getFloatAny(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value, true
		case float32:
			return float64(value), true
		case int:
			return float64(value), true
		case int8:
			return float64(value), true
		case int16:
			return float64(value), true
		case int32:
			return float64(value), true
		case int64:
			return float64(value), true
		case uint:
			return float64(value), true
		case uint8:
			return float64(value), true
		case uint16:
			return float64(value), true
		case uint32:
			return float64(value), true
		case uint64:
			return float64(value), true
		case json.Number:
			n, err := value.Float64()
			if err == nil {
				return n, true
			}
		case string:
			n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
