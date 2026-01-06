package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ChannelRateLimitResponse 渠道速率限制响应
type ChannelRateLimitResponse struct {
	ChannelID    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	KeyIndex     int    `json:"key_index"`
	RPMLimit     int    `json:"rpm_limit"`
	RPDLimit     int    `json:"rpd_limit"`
	RPMCount     int    `json:"rpm_count"`
	RPDCount     int    `json:"rpd_count"`
	RPMRemaining int    `json:"rpm_remaining"`
	RPDRemaining int    `json:"rpd_remaining"`
	Enabled      bool   `json:"enabled"`
}

// GetChannelRateLimitInfo 获取渠道速率限制信息
func GetChannelRateLimitInfo(c *gin.Context) {
	channelIdStr := c.Param("id")
	channelId, err := strconv.Atoi(channelIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的渠道 ID",
		})
		return
	}

	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "渠道不存在",
		})
		return
	}

	setting := channel.GetSetting()
	
	var responses []ChannelRateLimitResponse

	if channel.ChannelInfo.IsMultiKey {
		// 多 key 模式，获取每个 key 的信息
		for i := 0; i < channel.ChannelInfo.MultiKeySize; i++ {
			info := service.GetChannelRateLimitInfo(channelId, i, setting.RateLimitRPM, setting.RateLimitRPD)
			responses = append(responses, ChannelRateLimitResponse{
				ChannelID:    channelId,
				ChannelName:  channel.Name,
				KeyIndex:     i,
				RPMLimit:     setting.RateLimitRPM,
				RPDLimit:     setting.RateLimitRPD,
				RPMCount:     info.RPMCount,
				RPDCount:     info.RPDCount,
				RPMRemaining: info.RPMRemaining,
				RPDRemaining: info.RPDRemaining,
				Enabled:      setting.RateLimitEnabled,
			})
		}
	} else {
		// 单 key 模式
		info := service.GetChannelRateLimitInfo(channelId, 0, setting.RateLimitRPM, setting.RateLimitRPD)
		responses = append(responses, ChannelRateLimitResponse{
			ChannelID:    channelId,
			ChannelName:  channel.Name,
			KeyIndex:     0,
			RPMLimit:     setting.RateLimitRPM,
			RPDLimit:     setting.RateLimitRPD,
			RPMCount:     info.RPMCount,
			RPDCount:     info.RPDCount,
			RPMRemaining: info.RPMRemaining,
			RPDRemaining: info.RPDRemaining,
			Enabled:      setting.RateLimitEnabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
	})
}

// GetAllChannelRateLimitInfo 获取所有渠道的速率限制信息
func GetAllChannelRateLimitInfo(c *gin.Context) {
	// 获取所有启用了速率限制的渠道
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取渠道列表失败",
		})
		return
	}

	var responses []ChannelRateLimitResponse

	for _, channel := range channels {
		setting := channel.GetSetting()
		if !setting.RateLimitEnabled {
			continue
		}

		if channel.ChannelInfo.IsMultiKey {
			for i := 0; i < channel.ChannelInfo.MultiKeySize; i++ {
				info := service.GetChannelRateLimitInfo(channel.Id, i, setting.RateLimitRPM, setting.RateLimitRPD)
				responses = append(responses, ChannelRateLimitResponse{
					ChannelID:    channel.Id,
					ChannelName:  channel.Name,
					KeyIndex:     i,
					RPMLimit:     setting.RateLimitRPM,
					RPDLimit:     setting.RateLimitRPD,
					RPMCount:     info.RPMCount,
					RPDCount:     info.RPDCount,
					RPMRemaining: info.RPMRemaining,
					RPDRemaining: info.RPDRemaining,
					Enabled:      setting.RateLimitEnabled,
				})
			}
		} else {
			info := service.GetChannelRateLimitInfo(channel.Id, 0, setting.RateLimitRPM, setting.RateLimitRPD)
			responses = append(responses, ChannelRateLimitResponse{
				ChannelID:    channel.Id,
				ChannelName:  channel.Name,
				KeyIndex:     0,
				RPMLimit:     setting.RateLimitRPM,
				RPDLimit:     setting.RateLimitRPD,
				RPMCount:     info.RPMCount,
				RPDCount:     info.RPDCount,
				RPMRemaining: info.RPMRemaining,
				RPDRemaining: info.RPDRemaining,
				Enabled:      setting.RateLimitEnabled,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
		"total":   len(responses),
	})
}

// GetAllChannelsForBatchRateLimit 获取所有渠道列表（用于批量设置速率限制）
func GetAllChannelsForBatchRateLimit(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取渠道列表失败",
		})
		return
	}

	type ChannelSimple struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		RateLimitEnabled bool   `json:"rate_limit_enabled"`
		RateLimitRPM     int    `json:"rate_limit_rpm"`
		RateLimitRPD     int    `json:"rate_limit_rpd"`
	}

	var result []ChannelSimple
	for _, channel := range channels {
		setting := channel.GetSetting()
		result = append(result, ChannelSimple{
			ID:               channel.Id,
			Name:             channel.Name,
			RateLimitEnabled: setting.RateLimitEnabled,
			RateLimitRPM:     setting.RateLimitRPM,
			RateLimitRPD:     setting.RateLimitRPD,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// BatchSetChannelRateLimit 批量设置渠道速率限制
func BatchSetChannelRateLimit(c *gin.Context) {
	var req struct {
		Ids              []int `json:"ids" binding:"required"`
		RateLimitRPM     int   `json:"rate_limit_rpm"`
		RateLimitRPD     int   `json:"rate_limit_rpd"`
		RateLimitEnabled *bool `json:"rate_limit_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	if len(req.Ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请选择要设置的渠道",
		})
		return
	}

	// 获取所有渠道
	channels, err := model.GetChannelsByIds(req.Ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取渠道失败: " + err.Error(),
		})
		return
	}

	successCount := 0
	for _, channel := range channels {
		setting := channel.GetSetting()

		// 更新速率限制设置
		if req.RateLimitRPM >= 0 {
			setting.RateLimitRPM = req.RateLimitRPM
		}
		if req.RateLimitRPD >= 0 {
			setting.RateLimitRPD = req.RateLimitRPD
		}
		if req.RateLimitEnabled != nil {
			setting.RateLimitEnabled = *req.RateLimitEnabled
		}

		// 保存设置
		channel.SetSetting(setting)
		if err := channel.Save(); err != nil {
			continue
		}
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "成功更新 " + strconv.Itoa(successCount) + " 个渠道",
		"data":    successCount,
	})
}

// ResetChannelRateLimit 重置渠道速率限制计数
func ResetChannelRateLimit(c *gin.Context) {
	channelIdStr := c.Param("id")
	channelId, err := strconv.Atoi(channelIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的渠道 ID",
		})
		return
	}

	var req struct {
		KeyIndex int `json:"key_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有指定 key_index，重置所有
		req.KeyIndex = -1
	}

	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "渠道不存在",
		})
		return
	}

	if req.KeyIndex >= 0 {
		service.ResetChannelRateLimit(channelId, req.KeyIndex)
	} else {
		// 重置所有 key
		if channel.ChannelInfo.IsMultiKey {
			for i := 0; i < channel.ChannelInfo.MultiKeySize; i++ {
				service.ResetChannelRateLimit(channelId, i)
			}
		} else {
			service.ResetChannelRateLimit(channelId, 0)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "速率限制计数已重置",
	})
}
