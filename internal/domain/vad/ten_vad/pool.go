package ten_vad

import (
	"errors"
	"fmt"
	"sync"
	"time"

	. "xiaozhi-esp32-server-golang/internal/domain/vad/inter"

	log "xiaozhi-esp32-server-golang/logger"
)

// 资源池默认配置
var defaultPoolConfig = struct {
	// 池大小
	MaxSize int
	// 获取超时时间（毫秒）
	AcquireTimeout int64
}{
	MaxSize:        10,
	AcquireTimeout: 3000, // 3秒
}

// 全局变量和初始化
var (
	// 全局VAD资源池实例
	globalVADResourcePool *VADResourcePool

	once sync.Once
)

// VADResourcePool VAD资源池管理，不与会话ID绑定
type VADResourcePool struct {
	// 可用的VAD实例队列
	availableVADs chan VAD
	// 已分配的VAD实例映射，用于跟踪和管理
	allocatedVADs sync.Map
	// 池大小配置
	maxSize int
	// 获取VAD超时时间（毫秒）
	acquireTimeout int64
	// 默认VAD配置
	defaultConfig map[string]interface{}
	// 互斥锁，用于初始化和重置操作
	mu sync.Mutex
	// 是否已初始化标志
	initialized bool
}

// InitVADFromConfig 从配置文件初始化VAD模块
func InitVADFromConfig(config map[string]interface{}) error {
	// 获取配置项
	if rawHopSize, ok := config["hop_size"]; ok {
		if hopSize, ok := rawHopSize.(int); ok && hopSize > 0 {
			globalVADResourcePool.defaultConfig["hop_size"] = hopSize
		} else if hopSizeFloat, ok := rawHopSize.(float64); ok && hopSizeFloat > 0 {
			globalVADResourcePool.defaultConfig["hop_size"] = int(hopSizeFloat)
		}
	}

	if rawThreshold, ok := config["threshold"]; ok {
		threshold, ok := rawThreshold.(float64)
		if ok && threshold > 0 {
			globalVADResourcePool.defaultConfig["threshold"] = threshold
		} else if thresholdFloat32, ok := rawThreshold.(float32); ok && thresholdFloat32 > 0 {
			globalVADResourcePool.defaultConfig["threshold"] = float64(thresholdFloat32)
		}
	}

	// VAD资源池特有配置
	if rawPoolSize, ok := config["pool_size"]; ok {
		poolSize, ok := rawPoolSize.(int)
		if ok && poolSize > 0 {
			globalVADResourcePool.maxSize = poolSize
		} else if poolSizeFloat, ok := rawPoolSize.(float64); ok && poolSizeFloat > 0 {
			globalVADResourcePool.maxSize = int(poolSizeFloat)
		}
	}

	if rawTimeout, ok := config["acquire_timeout_ms"]; ok {
		timeout, ok := rawTimeout.(int64)
		if ok && timeout > 0 {
			globalVADResourcePool.acquireTimeout = timeout
		} else if timeoutFloat, ok := rawTimeout.(float64); ok && timeoutFloat > 0 {
			globalVADResourcePool.acquireTimeout = int64(timeoutFloat)
		}
	}

	// 完成初始化
	return initVADResourcePool()
}

// 内部方法：初始化VAD资源池
func initVADResourcePool() error {
	globalVADResourcePool.mu.Lock()
	defer globalVADResourcePool.mu.Unlock()

	// 已经初始化过，先关闭现有资源
	if globalVADResourcePool.availableVADs != nil {
		close(globalVADResourcePool.availableVADs)
		globalVADResourcePool.availableVADs = nil

		// 释放所有已分配的VAD实例
		globalVADResourcePool.allocatedVADs.Range(func(key, value interface{}) bool {
			if tenVAD, ok := value.(*TenVAD); ok {
				tenVAD.Close()
			}
			globalVADResourcePool.allocatedVADs.Delete(key)
			return true
		})
	}

	// 创建资源队列
	globalVADResourcePool.availableVADs = make(chan VAD, globalVADResourcePool.maxSize)

	// 预创建VAD实例
	for i := 0; i < globalVADResourcePool.maxSize; i++ {
		vadInstance, err := CreateVAD(globalVADResourcePool.defaultConfig)
		if err != nil {
			// 关闭已创建的实例
			for j := 0; j < i; j++ {
				vad := <-globalVADResourcePool.availableVADs
				if tenVAD, ok := vad.(*TenVAD); ok {
					tenVAD.Close()
				}
			}
			close(globalVADResourcePool.availableVADs)
			globalVADResourcePool.availableVADs = nil

			return fmt.Errorf("预创建VAD实例失败: %v", err)
		}

		// 放入可用队列
		globalVADResourcePool.availableVADs <- vadInstance
	}

	log.Infof("TEN-VAD资源池初始化完成，创建了 %d 个VAD实例", globalVADResourcePool.maxSize)
	globalVADResourcePool.initialized = true
	return nil
}

// InitVadPool 初始化VAD资源池
func InitVadPool(config map[string]interface{}) {
	once.Do(func() {
		globalVADResourcePool = &VADResourcePool{
			maxSize:        defaultPoolConfig.MaxSize,
			acquireTimeout: defaultPoolConfig.AcquireTimeout,
			defaultConfig:  defaultVADConfig,
			initialized:    false, // 标记为未完全初始化，需要后续读取配置
		}
		InitVADFromConfig(config)
	})
}

// AcquireVAD 获取一个VAD实例
func AcquireVAD(config map[string]interface{}) (VAD, error) {
	if globalVADResourcePool == nil || !globalVADResourcePool.initialized {
		return nil, errors.New("VAD资源池尚未初始化")
	}

	return globalVADResourcePool.AcquireVAD()
}

// ReleaseVAD 释放一个VAD实例
func ReleaseVAD(vad VAD) error {
	if globalVADResourcePool != nil && globalVADResourcePool.initialized {
		globalVADResourcePool.ReleaseVAD(vad)
	}
	return nil
}

// AcquireVAD 从资源池获取一个VAD实例
func (p *VADResourcePool) AcquireVAD() (VAD, error) {
	if !p.initialized {
		return nil, errors.New("VAD资源池未初始化")
	}

	// 设置超时
	timeout := time.After(time.Duration(p.acquireTimeout) * time.Millisecond)

	log.Debugf("获取TEN-VAD实例, 当前可用: %d/%d", len(p.availableVADs), p.maxSize)

	// 尝试从池中获取一个VAD实例
	select {
	case vad := <-p.availableVADs:
		if vad == nil {
			return nil, errors.New("VAD资源池已关闭")
		}

		// 标记为已分配
		p.allocatedVADs.Store(vad, time.Now())

		log.Debugf("从TEN-VAD资源池获取了一个VAD实例，当前可用: %d/%d", len(p.availableVADs), p.maxSize)
		return vad, nil

	case <-timeout:
		return nil, fmt.Errorf("获取TEN-VAD实例超时，当前资源池已满载运行（%d/%d）", p.maxSize, p.maxSize)
	}
}

// ReleaseVAD 释放VAD实例回资源池
func (p *VADResourcePool) ReleaseVAD(vad VAD) {
	if vad == nil || !p.initialized {
		return
	}

	log.Debugf("释放TEN-VAD实例: %v, 当前可用: %d/%d", vad, len(p.availableVADs), p.maxSize)

	// 检查是否是从此池分配的实例
	if _, exists := p.allocatedVADs.Load(vad); exists {
		// 从已分配映射中删除
		p.allocatedVADs.Delete(vad)

		// 如果资源池已关闭，直接销毁实例
		if p.availableVADs == nil {
			if tenVAD, ok := vad.(*TenVAD); ok {
				tenVAD.Close()
			}
			return
		}

		// 尝试放回资源池，如果满了就丢弃
		select {
		case p.availableVADs <- vad:
			log.Debugf("TEN-VAD实例已归还资源池，当前可用: %d/%d", len(p.availableVADs), p.maxSize)
		default:
			// 资源池满了，直接关闭实例
			if tenVAD, ok := vad.(*TenVAD); ok {
				tenVAD.Close()
			}
			log.Warn("TEN-VAD资源池已满，多余实例已销毁")
		}
	} else {
		log.Warn("尝试释放非此资源池管理的TEN-VAD实例")
	}
}

// GetActiveCount 获取当前活跃（被分配）的VAD实例数量
func (p *VADResourcePool) GetActiveCount() int {
	count := 0
	p.allocatedVADs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetAvailableCount 获取当前可用的VAD实例数量
func (p *VADResourcePool) GetAvailableCount() int {
	if p.availableVADs == nil {
		return 0
	}
	return len(p.availableVADs)
}

// Close 关闭资源池，释放所有资源
func (p *VADResourcePool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.availableVADs != nil {
		// 关闭可用队列
		close(p.availableVADs)

		// 释放所有可用的VAD实例
		for vad := range p.availableVADs {
			if tenVAD, ok := vad.(*TenVAD); ok {
				tenVAD.Close()
			}
		}

		p.availableVADs = nil
	}

	// 释放所有已分配的VAD实例
	p.allocatedVADs.Range(func(key, _ interface{}) bool {
		vad := key.(VAD)
		if tenVAD, ok := vad.(*TenVAD); ok {
			tenVAD.Close()
		}
		p.allocatedVADs.Delete(key)
		return true
	})

	p.initialized = false
	log.Info("TEN-VAD资源池已关闭，所有资源已释放")
}
