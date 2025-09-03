package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// ActiveHealthChecker 主动健康检查器
type ActiveHealthChecker struct {
	mu          sync.RWMutex
	upstreams   map[string]*upstreamHealthState
	config      *config.Config
	stopCh      chan struct{}
	wg          sync.WaitGroup
	running     bool
	client      *http.Client
	callbacks   []HealthChangeCallback
}

// upstreamHealthState 上游服务健康状态
type upstreamHealthState struct {
	upstream *types.Upstream
	targets  map[string]*targetHealthState
	stopCh   chan struct{}
	config   *types.HealthCheck
}

// targetHealthState 目标实例健康状态
type targetHealthState struct {
	target              *types.Target
	healthy             bool
	consecutiveSuccess  int
	consecutiveFailures int
	lastCheckTime       time.Time
	lastError           error
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Target      *types.Target
	Healthy     bool
	Error       error
	Duration    time.Duration
	StatusCode  int
	CheckTime   time.Time
	UpstreamID  string
}

// HealthChangeCallback 健康状态变化回调函数
type HealthChangeCallback func(upstreamID string, target *types.Target, healthy bool)

// NewActiveHealthChecker 创建新的主动健康检查器
func NewActiveHealthChecker(cfg *config.Config) *ActiveHealthChecker {
	return &ActiveHealthChecker{
		upstreams: make(map[string]*upstreamHealthState),
		config:    cfg,
		stopCh:    make(chan struct{}),
		client: &http.Client{
			Timeout: 10 * time.Second, // 默认超时10秒
		},
		callbacks: make([]HealthChangeCallback, 0),
	}
}

// AddHealthChangeCallback 添加健康状态变化回调
func (hc *ActiveHealthChecker) AddHealthChangeCallback(callback HealthChangeCallback) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.callbacks = append(hc.callbacks, callback)
}

// Start 启动健康检查器
func (hc *ActiveHealthChecker) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker is already running")
	}

	hc.running = true
	hc.stopCh = make(chan struct{})

	// 启动所有已注册的上游服务检查
	for upstreamID, state := range hc.upstreams {
		hc.startUpstreamCheck(upstreamID, state)
	}

	return nil
}

// Stop 停止健康检查器
func (hc *ActiveHealthChecker) Stop() error {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return nil
	}

	hc.running = false
	close(hc.stopCh)

	// 停止所有上游服务检查
	for _, state := range hc.upstreams {
		if state.stopCh != nil {
			close(state.stopCh)
		}
	}
	hc.mu.Unlock()

	// 等待所有goroutine结束（在锁外等待）
	hc.wg.Wait()

	return nil
}

// AddUpstream 添加上游服务进行健康检查
func (hc *ActiveHealthChecker) AddUpstream(upstream *types.Upstream) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// 使用默认健康检查配置或上游服务指定的配置
	healthConfig := hc.getHealthCheckConfig(upstream)
	if healthConfig == nil {
		// 如果没有配置健康检查，跳过
		return nil
	}

	// 创建目标健康状态
	targets := make(map[string]*targetHealthState)
	for _, target := range upstream.Targets {
		key := fmt.Sprintf("%s:%d", target.Host, target.Port)
		targets[key] = &targetHealthState{
			target:  target,
			healthy: target.Healthy, // 初始状态使用配置的健康状态
		}
	}

	// 创建上游健康状态
	state := &upstreamHealthState{
		upstream: upstream,
		targets:  targets,
		stopCh:   make(chan struct{}),
		config:   healthConfig,
	}

	hc.upstreams[upstream.ID] = state

	// 如果健康检查器正在运行，立即启动这个上游服务的检查
	if hc.running {
		hc.startUpstreamCheck(upstream.ID, state)
	}

	return nil
}

// RemoveUpstream 移除上游服务的健康检查
func (hc *ActiveHealthChecker) RemoveUpstream(upstreamID string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	state, exists := hc.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	// 停止检查
	if state.stopCh != nil {
		close(state.stopCh)
	}

	delete(hc.upstreams, upstreamID)
	return nil
}

// getHealthCheckConfig 获取健康检查配置
func (hc *ActiveHealthChecker) getHealthCheckConfig(upstream *types.Upstream) *types.HealthCheck {
	// 优先使用上游服务的健康检查配置
	if upstream.HealthCheck != nil {
		return upstream.HealthCheck
	}

	// 使用默认配置
	return &types.HealthCheck{
		Type:               "http",
		Path:               "/health",
		Interval:           30, // 30秒检查一次
		Timeout:            5,  // 5秒超时
		HealthyThreshold:   2,  // 连续2次成功标记为健康
		UnhealthyThreshold: 3,  // 连续3次失败标记为不健康
	}
}

// startUpstreamCheck 启动上游服务的健康检查
func (hc *ActiveHealthChecker) startUpstreamCheck(upstreamID string, state *upstreamHealthState) {
	hc.wg.Add(1)
	go func() {
		defer hc.wg.Done()
		hc.checkUpstream(upstreamID, state)
	}()
}

// checkUpstream 检查上游服务的所有目标
func (hc *ActiveHealthChecker) checkUpstream(upstreamID string, state *upstreamHealthState) {
	ticker := time.NewTicker(time.Duration(state.config.Interval) * time.Second)
	defer ticker.Stop()

	// 立即执行一次检查
	hc.checkAllTargets(upstreamID, state)

	for {
		select {
		case <-ticker.C:
			hc.checkAllTargets(upstreamID, state)
		case <-state.stopCh:
			return
		case <-hc.stopCh:
			return
		}
	}
}

// checkAllTargets 检查所有目标实例
func (hc *ActiveHealthChecker) checkAllTargets(upstreamID string, state *upstreamHealthState) {
	var wg sync.WaitGroup

	for _, targetState := range state.targets {
		wg.Add(1)
		go func(ts *targetHealthState) {
			defer wg.Done()
			result := hc.checkTarget(ts.target, state.config, upstreamID)
			hc.updateTargetHealth(upstreamID, ts, result)
		}(targetState)
	}

	wg.Wait()
}

// checkTarget 检查单个目标实例
func (hc *ActiveHealthChecker) checkTarget(target *types.Target, config *types.HealthCheck, upstreamID string) *HealthCheckResult {
	startTime := time.Now()
	
	// 构建健康检查URL
	url := fmt.Sprintf("http://%s:%d%s", target.Host, target.Port, config.Path)
	
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &HealthCheckResult{
			Target:     target,
			Healthy:    false,
			Error:      err,
			Duration:   time.Since(startTime),
			CheckTime:  startTime,
			UpstreamID: upstreamID,
		}
	}

	// 发送请求
	resp, err := hc.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return &HealthCheckResult{
			Target:     target,
			Healthy:    false,
			Error:      err,
			Duration:   duration,
			CheckTime:  startTime,
			UpstreamID: upstreamID,
		}
	}
	defer resp.Body.Close()

	// 检查响应状态码（200-299为健康）
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300

	return &HealthCheckResult{
		Target:     target,
		Healthy:    healthy,
		Error:      nil,
		Duration:   duration,
		StatusCode: resp.StatusCode,
		CheckTime:  startTime,
		UpstreamID: upstreamID,
	}
}

// updateTargetHealth 更新目标实例的健康状态
func (hc *ActiveHealthChecker) updateTargetHealth(upstreamID string, targetState *targetHealthState, result *HealthCheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// 更新检查时间和错误信息
	targetState.lastCheckTime = result.CheckTime
	targetState.lastError = result.Error

	// 获取健康检查配置
	state := hc.upstreams[upstreamID]
	if state == nil {
		return
	}

	oldHealthy := targetState.healthy

	// 更新连续成功/失败计数
	if result.Healthy {
		targetState.consecutiveSuccess++
		targetState.consecutiveFailures = 0

		// 检查是否达到健康阈值
		if !targetState.healthy && targetState.consecutiveSuccess >= state.config.HealthyThreshold {
			targetState.healthy = true
		}
	} else {
		targetState.consecutiveFailures++
		targetState.consecutiveSuccess = 0

		// 检查是否达到不健康阈值
		if targetState.healthy && targetState.consecutiveFailures >= state.config.UnhealthyThreshold {
			targetState.healthy = false
		}
	}

	// 如果健康状态发生变化，通知回调函数
	if oldHealthy != targetState.healthy {
		// 更新目标实例的健康状态
		targetState.target.Healthy = targetState.healthy

		// 调用所有回调函数
		for _, callback := range hc.callbacks {
			go callback(upstreamID, targetState.target, targetState.healthy)
		}
	}
}

// GetUpstreamHealth 获取上游服务的健康状态
func (hc *ActiveHealthChecker) GetUpstreamHealth(upstreamID string) map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	state, exists := hc.upstreams[upstreamID]
	if !exists {
		return nil
	}

	targets := make([]map[string]interface{}, 0, len(state.targets))
	healthyCount := 0

	for _, targetState := range state.targets {
		if targetState.healthy {
			healthyCount++
		}

		targets = append(targets, map[string]interface{}{
			"host":                 targetState.target.Host,
			"port":                 targetState.target.Port,
			"healthy":              targetState.healthy,
			"consecutive_success":  targetState.consecutiveSuccess,
			"consecutive_failures": targetState.consecutiveFailures,
			"last_check_time":      targetState.lastCheckTime.Unix(),
			"last_error":           errorToString(targetState.lastError),
		})
	}

	return map[string]interface{}{
		"upstream_id":     upstreamID,
		"total_targets":   len(state.targets),
		"healthy_targets": healthyCount,
		"config":          state.config,
		"targets":         targets,
	}
}

// GetAllHealth 获取所有上游服务的健康状态
func (hc *ActiveHealthChecker) GetAllHealth() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := map[string]interface{}{
		"running":         hc.running,
		"upstreams_count": len(hc.upstreams),
		"upstreams":       make(map[string]interface{}),
	}

	upstreams := result["upstreams"].(map[string]interface{})
	for upstreamID := range hc.upstreams {
		upstreams[upstreamID] = hc.GetUpstreamHealth(upstreamID)
	}

	return result
}

// errorToString 将错误转换为字符串
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
