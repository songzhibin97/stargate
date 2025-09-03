package health

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/types"
)

// PassiveHealthChecker 被动健康检查器
// 根据实际请求的成功/失败情况来判断后端实例的健康状况
type PassiveHealthChecker struct {
	mu       sync.RWMutex
	config   *PassiveHealthConfig
	targets  map[string]*passiveTargetState // key: upstreamID:host:port
	callback HealthStatusCallback
	running  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// PassiveHealthConfig 被动健康检查配置
type PassiveHealthConfig struct {
	// 连续失败次数阈值，超过此值将隔离实例
	ConsecutiveFailures int `yaml:"consecutive_failures" json:"consecutive_failures"`
	
	// 隔离时间，实例被隔离后多长时间尝试恢复
	IsolationDuration time.Duration `yaml:"isolation_duration" json:"isolation_duration"`
	
	// 恢复检查间隔，隔离期间多久检查一次是否可以恢复
	RecoveryInterval time.Duration `yaml:"recovery_interval" json:"recovery_interval"`
	
	// 连续成功次数阈值，隔离的实例连续成功此次数后恢复
	ConsecutiveSuccesses int `yaml:"consecutive_successes" json:"consecutive_successes"`
	
	// 是否启用被动健康检查
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// 监控的HTTP状态码范围，这些状态码被认为是失败
	// 默认监控 5xx 错误
	FailureStatusCodes []int `yaml:"failure_status_codes" json:"failure_status_codes"`
	
	// 是否将超时也视为失败
	TimeoutAsFailure bool `yaml:"timeout_as_failure" json:"timeout_as_failure"`
}

// passiveTargetState 被动健康检查的目标状态
type passiveTargetState struct {
	upstreamID           string
	target               *types.Target
	healthy              bool
	isolated             bool
	consecutiveFailures  int
	consecutiveSuccesses int
	lastFailureTime      time.Time
	lastSuccessTime      time.Time
	isolationStartTime   time.Time
	totalRequests        int64
	totalFailures        int64
	totalSuccesses       int64
}

// HealthStatusCallback 健康状态变化回调函数
type HealthStatusCallback func(upstreamID, targetKey string, healthy bool)

// RequestResult 请求结果
type RequestResult struct {
	UpstreamID   string
	Target       *types.Target
	StatusCode   int
	Error        error
	Duration     time.Duration
	IsTimeout    bool
	Timestamp    time.Time
}

// NewPassiveHealthChecker 创建被动健康检查器
func NewPassiveHealthChecker(config *PassiveHealthConfig, callback HealthStatusCallback) *PassiveHealthChecker {
	if config == nil {
		config = &PassiveHealthConfig{
			Enabled:              true,
			ConsecutiveFailures:  3,
			IsolationDuration:    30 * time.Second,
			RecoveryInterval:     10 * time.Second,
			ConsecutiveSuccesses: 2,
			FailureStatusCodes:   []int{500, 501, 502, 503, 504, 505},
			TimeoutAsFailure:     true,
		}
	}

	return &PassiveHealthChecker{
		config:   config,
		targets:  make(map[string]*passiveTargetState),
		callback: callback,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动被动健康检查器
func (phc *PassiveHealthChecker) Start() error {
	phc.mu.Lock()
	defer phc.mu.Unlock()

	if phc.running {
		return fmt.Errorf("passive health checker is already running")
	}

	if !phc.config.Enabled {
		log.Println("Passive health checker is disabled")
		return nil
	}

	phc.running = true
	phc.stopCh = make(chan struct{})

	// 启动恢复检查goroutine
	phc.wg.Add(1)
	go phc.recoveryLoop()

	log.Println("Passive health checker started")
	return nil
}

// Stop 停止被动健康检查器
func (phc *PassiveHealthChecker) Stop() error {
	phc.mu.Lock()
	if !phc.running {
		phc.mu.Unlock()
		return nil
	}

	phc.running = false
	close(phc.stopCh)
	phc.mu.Unlock()

	// 等待所有goroutine结束
	phc.wg.Wait()

	log.Println("Passive health checker stopped")
	return nil
}

// AddTarget 添加目标实例进行被动健康检查
func (phc *PassiveHealthChecker) AddTarget(upstreamID string, target *types.Target) error {
	phc.mu.Lock()
	defer phc.mu.Unlock()

	targetKey := fmt.Sprintf("%s:%s:%d", upstreamID, target.Host, target.Port)
	
	phc.targets[targetKey] = &passiveTargetState{
		upstreamID:           upstreamID,
		target:               target,
		healthy:              true, // 初始假设为健康
		isolated:             false,
		consecutiveFailures:  0,
		consecutiveSuccesses: 0,
		lastSuccessTime:      time.Now(),
	}

	log.Printf("Added target %s for passive health checking", targetKey)
	return nil
}

// RemoveTarget 移除目标实例
func (phc *PassiveHealthChecker) RemoveTarget(upstreamID string, target *types.Target) error {
	phc.mu.Lock()
	defer phc.mu.Unlock()

	targetKey := fmt.Sprintf("%s:%s:%d", upstreamID, target.Host, target.Port)
	delete(phc.targets, targetKey)

	log.Printf("Removed target %s from passive health checking", targetKey)
	return nil
}

// RecordRequest 记录请求结果
func (phc *PassiveHealthChecker) RecordRequest(result *RequestResult) {
	if !phc.config.Enabled {
		return
	}

	phc.mu.Lock()
	defer phc.mu.Unlock()

	targetKey := fmt.Sprintf("%s:%s:%d", result.UpstreamID, result.Target.Host, result.Target.Port)
	state, exists := phc.targets[targetKey]
	if !exists {
		// 如果目标不存在，自动添加
		phc.targets[targetKey] = &passiveTargetState{
			upstreamID:           result.UpstreamID,
			target:               result.Target,
			healthy:              true,
			isolated:             false,
			consecutiveFailures:  0,
			consecutiveSuccesses: 0,
			lastSuccessTime:      time.Now(),
		}
		state = phc.targets[targetKey]
	}

	state.totalRequests++

	// 判断请求是否失败
	isFailure := phc.isRequestFailure(result)

	if isFailure {
		phc.handleFailure(state, result)
	} else {
		phc.handleSuccess(state, result)
	}
}

// isRequestFailure 判断请求是否为失败
func (phc *PassiveHealthChecker) isRequestFailure(result *RequestResult) bool {
	// 检查是否有错误
	if result.Error != nil {
		return true
	}

	// 检查是否超时
	if result.IsTimeout && phc.config.TimeoutAsFailure {
		return true
	}

	// 检查状态码是否在失败范围内
	for _, code := range phc.config.FailureStatusCodes {
		if result.StatusCode == code {
			return true
		}
	}

	return false
}

// handleFailure 处理失败请求
func (phc *PassiveHealthChecker) handleFailure(state *passiveTargetState, result *RequestResult) {
	state.totalFailures++
	state.consecutiveFailures++
	state.consecutiveSuccesses = 0
	state.lastFailureTime = result.Timestamp

	log.Printf("Target %s:%d failure recorded, consecutive failures: %d", 
		state.target.Host, state.target.Port, state.consecutiveFailures)

	// 检查是否需要隔离
	if !state.isolated && state.consecutiveFailures >= phc.config.ConsecutiveFailures {
		phc.isolateTarget(state)
	}
}

// handleSuccess 处理成功请求
func (phc *PassiveHealthChecker) handleSuccess(state *passiveTargetState, result *RequestResult) {
	state.totalSuccesses++
	state.consecutiveSuccesses++
	state.consecutiveFailures = 0
	state.lastSuccessTime = result.Timestamp

	// 如果目标被隔离，检查是否可以恢复
	if state.isolated && state.consecutiveSuccesses >= phc.config.ConsecutiveSuccesses {
		phc.recoverTarget(state)
	}
}

// isolateTarget 隔离目标实例
func (phc *PassiveHealthChecker) isolateTarget(state *passiveTargetState) {
	state.isolated = true
	state.healthy = false
	state.isolationStartTime = time.Now()

	targetKey := fmt.Sprintf("%s:%s:%d", state.upstreamID, state.target.Host, state.target.Port)
	
	log.Printf("Target %s isolated due to %d consecutive failures", 
		targetKey, state.consecutiveFailures)

	// 通知负载均衡器
	if phc.callback != nil {
		phc.callback(state.upstreamID, targetKey, false)
	}
}

// recoverTarget 恢复目标实例
func (phc *PassiveHealthChecker) recoverTarget(state *passiveTargetState) {
	state.isolated = false
	state.healthy = true
	state.consecutiveFailures = 0

	targetKey := fmt.Sprintf("%s:%s:%d", state.upstreamID, state.target.Host, state.target.Port)
	
	log.Printf("Target %s recovered after %d consecutive successes", 
		targetKey, state.consecutiveSuccesses)

	// 通知负载均衡器
	if phc.callback != nil {
		phc.callback(state.upstreamID, targetKey, true)
	}
}

// recoveryLoop 恢复检查循环
func (phc *PassiveHealthChecker) recoveryLoop() {
	defer phc.wg.Done()

	ticker := time.NewTicker(phc.config.RecoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-phc.stopCh:
			return
		case <-ticker.C:
			phc.checkIsolatedTargets()
		}
	}
}

// checkIsolatedTargets 检查被隔离的目标是否可以尝试恢复
func (phc *PassiveHealthChecker) checkIsolatedTargets() {
	phc.mu.RLock()
	defer phc.mu.RUnlock()

	now := time.Now()
	for targetKey, state := range phc.targets {
		if state.isolated && now.Sub(state.isolationStartTime) >= phc.config.IsolationDuration {
			// 隔离时间已到，重置连续成功计数器，等待新的请求来验证
			state.consecutiveSuccesses = 0
			log.Printf("Target %s isolation period expired, ready for recovery attempts", targetKey)
		}
	}
}

// IsTargetHealthy 检查目标是否健康
func (phc *PassiveHealthChecker) IsTargetHealthy(upstreamID string, target *types.Target) bool {
	phc.mu.RLock()
	defer phc.mu.RUnlock()

	targetKey := fmt.Sprintf("%s:%s:%d", upstreamID, target.Host, target.Port)
	state, exists := phc.targets[targetKey]
	if !exists {
		return true // 未知目标默认为健康
	}

	return state.healthy && !state.isolated
}

// GetTargetStats 获取目标统计信息
func (phc *PassiveHealthChecker) GetTargetStats(upstreamID string, target *types.Target) map[string]interface{} {
	phc.mu.RLock()
	defer phc.mu.RUnlock()

	targetKey := fmt.Sprintf("%s:%s:%d", upstreamID, target.Host, target.Port)
	state, exists := phc.targets[targetKey]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"healthy":                state.healthy,
		"isolated":               state.isolated,
		"consecutive_failures":   state.consecutiveFailures,
		"consecutive_successes":  state.consecutiveSuccesses,
		"total_requests":         state.totalRequests,
		"total_failures":         state.totalFailures,
		"total_successes":        state.totalSuccesses,
		"last_failure_time":      state.lastFailureTime,
		"last_success_time":      state.lastSuccessTime,
		"isolation_start_time":   state.isolationStartTime,
	}
}

// Health 返回被动健康检查器的健康状态
func (phc *PassiveHealthChecker) Health() map[string]interface{} {
	phc.mu.RLock()
	defer phc.mu.RUnlock()

	totalTargets := len(phc.targets)
	healthyTargets := 0
	isolatedTargets := 0

	for _, state := range phc.targets {
		if state.healthy {
			healthyTargets++
		}
		if state.isolated {
			isolatedTargets++
		}
	}

	return map[string]interface{}{
		"enabled":          phc.config.Enabled,
		"running":          phc.running,
		"total_targets":    totalTargets,
		"healthy_targets":  healthyTargets,
		"isolated_targets": isolatedTargets,
		"config":           phc.config,
	}
}
