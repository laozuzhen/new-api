package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ChannelRateLimitInfo 渠道速率限制信息
type ChannelRateLimitInfo struct {
	ChannelID     int   `json:"channel_id"`
	KeyIndex      int   `json:"key_index"`      // 多 key 模式下的 key 索引
	RPMCount      int   `json:"rpm_count"`      // 当前分钟请求数
	RPDCount      int   `json:"rpd_count"`      // 当天请求数
	RPMLimit      int   `json:"rpm_limit"`      // 每分钟限制
	RPDLimit      int   `json:"rpd_limit"`      // 每天限制
	RPMRemaining  int   `json:"rpm_remaining"`  // 每分钟剩余
	RPDRemaining  int   `json:"rpd_remaining"`  // 每天剩余
	LastMinuteKey string `json:"last_minute_key"` // 上次分钟 key
	LastDayKey    string `json:"last_day_key"`    // 上次日期 key
}

// 内存存储（简单实现，生产环境建议用 Redis）
var (
	channelRateLimitStore = make(map[string]*ChannelRateLimitInfo)
	channelRateLimitMutex sync.RWMutex
)

// getChannelRateLimitKey 生成渠道速率限制的 key
func getChannelRateLimitKey(channelID int, keyIndex int) string {
	return fmt.Sprintf("channel_rate_limit:%d:%d", channelID, keyIndex)
}

// GetChannelRateLimitInfo 获取渠道速率限制信息
func GetChannelRateLimitInfo(channelID int, keyIndex int, rpmLimit int, rpdLimit int) *ChannelRateLimitInfo {
	key := getChannelRateLimitKey(channelID, keyIndex)
	currentMinute := time.Now().Format("2006-01-02-15-04")
	currentDay := time.Now().Format("2006-01-02")

	channelRateLimitMutex.Lock()
	defer channelRateLimitMutex.Unlock()

	info, exists := channelRateLimitStore[key]
	if !exists {
		info = &ChannelRateLimitInfo{
			ChannelID:     channelID,
			KeyIndex:      keyIndex,
			RPMCount:      0,
			RPDCount:      0,
			RPMLimit:      rpmLimit,
			RPDLimit:      rpdLimit,
			LastMinuteKey: currentMinute,
			LastDayKey:    currentDay,
		}
		channelRateLimitStore[key] = info
	}

	// 检查是否需要重置分钟计数
	if info.LastMinuteKey != currentMinute {
		info.RPMCount = 0
		info.LastMinuteKey = currentMinute
	}

	// 检查是否需要重置日计数
	if info.LastDayKey != currentDay {
		info.RPDCount = 0
		info.LastDayKey = currentDay
	}

	// 更新限制值
	info.RPMLimit = rpmLimit
	info.RPDLimit = rpdLimit

	// 计算剩余
	if rpmLimit > 0 {
		info.RPMRemaining = rpmLimit - info.RPMCount
		if info.RPMRemaining < 0 {
			info.RPMRemaining = 0
		}
	} else {
		info.RPMRemaining = -1 // -1 表示无限制
	}

	if rpdLimit > 0 {
		info.RPDRemaining = rpdLimit - info.RPDCount
		if info.RPDRemaining < 0 {
			info.RPDRemaining = 0
		}
	} else {
		info.RPDRemaining = -1 // -1 表示无限制
	}

	return info
}

// CheckChannelRateLimit 检查渠道是否超过速率限制
// 返回: (是否允许, 错误信息)
func CheckChannelRateLimit(channelID int, keyIndex int, rpmLimit int, rpdLimit int) (bool, string) {
	if rpmLimit <= 0 && rpdLimit <= 0 {
		return true, "" // 没有限制
	}

	info := GetChannelRateLimitInfo(channelID, keyIndex, rpmLimit, rpdLimit)

	// 检查 RPM 限制
	if rpmLimit > 0 && info.RPMCount >= rpmLimit {
		return false, fmt.Sprintf("渠道 %d (key %d) 已达到每分钟请求限制 (%d/%d)", channelID, keyIndex, info.RPMCount, rpmLimit)
	}

	// 检查 RPD 限制
	if rpdLimit > 0 && info.RPDCount >= rpdLimit {
		return false, fmt.Sprintf("渠道 %d (key %d) 已达到每天请求限制 (%d/%d)", channelID, keyIndex, info.RPDCount, rpdLimit)
	}

	return true, ""
}

// IncrementChannelRateLimit 增加渠道请求计数
func IncrementChannelRateLimit(channelID int, keyIndex int, rpmLimit int, rpdLimit int) {
	key := getChannelRateLimitKey(channelID, keyIndex)
	currentMinute := time.Now().Format("2006-01-02-15-04")
	currentDay := time.Now().Format("2006-01-02")

	channelRateLimitMutex.Lock()
	defer channelRateLimitMutex.Unlock()

	info, exists := channelRateLimitStore[key]
	if !exists {
		info = &ChannelRateLimitInfo{
			ChannelID:     channelID,
			KeyIndex:      keyIndex,
			RPMCount:      0,
			RPDCount:      0,
			RPMLimit:      rpmLimit,
			RPDLimit:      rpdLimit,
			LastMinuteKey: currentMinute,
			LastDayKey:    currentDay,
		}
		channelRateLimitStore[key] = info
	}

	// 检查是否需要重置分钟计数
	if info.LastMinuteKey != currentMinute {
		info.RPMCount = 0
		info.LastMinuteKey = currentMinute
	}

	// 检查是否需要重置日计数
	if info.LastDayKey != currentDay {
		info.RPDCount = 0
		info.LastDayKey = currentDay
	}

	// 增加计数
	info.RPMCount++
	info.RPDCount++

	if common.DebugEnabled {
		fmt.Printf("[ChannelRateLimit] Channel %d Key %d: RPM=%d/%d, RPD=%d/%d\n",
			channelID, keyIndex, info.RPMCount, rpmLimit, info.RPDCount, rpdLimit)
	}
}

// GetAllChannelRateLimitInfo 获取所有渠道的速率限制信息
func GetAllChannelRateLimitInfo() map[string]*ChannelRateLimitInfo {
	channelRateLimitMutex.RLock()
	defer channelRateLimitMutex.RUnlock()

	result := make(map[string]*ChannelRateLimitInfo)
	for k, v := range channelRateLimitStore {
		result[k] = v
	}
	return result
}

// GetChannelRateLimitInfoByChannelID 根据渠道 ID 获取所有 key 的速率限制信息
func GetChannelRateLimitInfoByChannelID(channelID int) []*ChannelRateLimitInfo {
	channelRateLimitMutex.RLock()
	defer channelRateLimitMutex.RUnlock()

	var result []*ChannelRateLimitInfo
	prefix := fmt.Sprintf("channel_rate_limit:%d:", channelID)
	for k, v := range channelRateLimitStore {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, v)
		}
	}
	return result
}

// ResetChannelRateLimit 重置渠道速率限制计数
func ResetChannelRateLimit(channelID int, keyIndex int) {
	key := getChannelRateLimitKey(channelID, keyIndex)

	channelRateLimitMutex.Lock()
	defer channelRateLimitMutex.Unlock()

	delete(channelRateLimitStore, key)
}
