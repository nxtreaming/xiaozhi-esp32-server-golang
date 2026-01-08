package controllers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeviceActivationController struct {
	DB *gorm.DB
}

// 生成6位随机数字代码
func generateCode() string {
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	code := 0
	for i, b := range randomBytes {
		code += int(b) << (8 * i)
	}
	return fmt.Sprintf("%06d", code%1000000)
}

// 生成UUID格式的挑战码
func generateChallenge() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)

	// 设置版本 (4) 和变体位
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		randomBytes[0:4],
		randomBytes[4:6],
		randomBytes[6:8],
		randomBytes[8:10],
		randomBytes[10:16])
}

// 1. 判断设备是否已激活
// GET /api/public/device/check-activation?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) CheckDeviceActivation(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "device_id参数必填",
		})
		return
	}

	var device models.Device
	// 使用device_id (对应device_name字段) 查找设备
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"activated": false,
				"message":   "设备不存在",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "查询设备失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": device.Activated,
		"message": func() string {
			if device.Activated {
				return "设备已激活"
			}
			return "设备未激活"
		}(),
	})
}

// 2. 获取激活信息
// GET /api/public/device/activation-info?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) GetActivationInfo(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id和client_id参数必填"})
		return
	}

	var device models.Device
	var isNewDevice bool

	// 使用device_id (对应device_name字段) 查找设备
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 设备不存在，创建新设备记录
			device = models.Device{
				DeviceName: deviceId,
				UserID:     0, // user_id置为0
				DeviceCode: generateCode(),
				Challenge:  generateChallenge(),
				Activated:  false,
			}

			if err := dac.DB.Create(&device).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "创建设备记录失败"})
				return
			}
			isNewDevice = true
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询设备失败"})
			return
		}
	}

	// 如果设备已激活，直接返回状态
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"activated": true,
			"message":   "设备已激活",
		})
		return
	}

	// 如果设备未激活，生成或返回激活信息
	needUpdate := false

	// 如果没有激活码，生成新的激活码
	if device.DeviceCode == "" {
		device.DeviceCode = generateCode()
		needUpdate = true
	}

	// 如果没有挑战码，生成新的挑战码
	if device.Challenge == "" {
		device.Challenge = generateChallenge()
		needUpdate = true
	}

	// 确保user_id为0（如果不是新设备且未激活）
	if !isNewDevice && device.UserID != 0 {
		device.UserID = 0
		needUpdate = true
	}

	// 更新数据库
	if needUpdate {
		if err := dac.DB.Save(&device).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设备信息失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": false,
		"code":      device.DeviceCode,
		"challenge": device.Challenge,
	})
}

// 验证HMAC-SHA256
func verifyHMAC(challenge, secretKey, providedHmac string) bool {
	if secretKey == "" {
		return true // 如果pre_secret_key为空，直接通过验证
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// 3. 设备激活接口
// POST /api/public/device/activate
func (dac *DeviceActivationController) ActivateDevice(c *gin.Context) {
	var req struct {
		DeviceId     string `json:"device_id" binding:"required"`
		ClientId     string `json:"client_id" binding:"required"`
		Challenge    string `json:"challenge" binding:"required"`
		Algorithm    string `json:"algorithm" binding:"required"`
		SerialNumber string `json:"serial_number" binding:"required"`
		Hmac         string `json:"hmac" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var device models.Device
	// 使用device_id (对应device_name字段) 查找设备
	if err := dac.DB.Where("device_name = ?", req.DeviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "设备不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "查询设备失败",
		})
		return
	}

	// 检查设备是否已经激活
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "设备已激活",
		})
		return
	}

	if device.UserID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "设备未绑定用户",
		})
		return
	}

	if device.Challenge != req.Challenge {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "挑战码错误",
		})
		return
	}

	// 验证HMAC（如果pre_secret_key为空则直接通过）
	if !verifyHMAC(req.Challenge, device.PreSecretKey, req.Hmac) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "HMAC验证失败",
		})
		return
	}

	// 激活设备
	device.Activated = true
	if err := dac.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "激活设备失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备激活成功",
		"data": gin.H{
			"device_id": device.DeviceName,
			"activated": device.Activated,
		},
	})
}
