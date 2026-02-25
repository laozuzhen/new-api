package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type ExternalSubscriptionEpayPayRequest struct {
	PlanId        int    `json:"plan_id"`
	PaymentMethod string `json:"payment_method"`
}

// ExternalSubscriptionRequestEpay Editor 用户发起 Epay 支付
func ExternalSubscriptionRequestEpay(c *gin.Context) {
	var req ExternalSubscriptionEpayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	externalUserId := c.GetString("external_user_id")
	if externalUserId == "" {
		common.ApiErrorMsg(c, "未获取到用户信息")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}

	callBackAddress := service.GetCallbackAddress()
	returnUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	notifyUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}

	tradeNo := fmt.Sprintf("SUBEXT%sNO%s%d", externalUserId, common.GetRandomString(6), time.Now().Unix())

	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}

	order := &model.SubscriptionOrder{
		UserId:         0,
		ExternalUserId: externalUserId,
		PlanId:         plan.Id,
		Money:          plan.PriceAmount,
		TradeNo:        tradeNo,
		PaymentMethod:  req.PaymentMethod,
		CreateTime:     time.Now().Unix(),
		Status:         common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("SUB:%s", plan.Title),
		Money:          strconv.FormatFloat(plan.PriceAmount, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

// ExternalGetSubscriptionSelf Editor 用户查询自己的订阅
func ExternalGetSubscriptionSelf(c *gin.Context) {
	externalUserId := c.GetString("external_user_id")
	if externalUserId == "" {
		common.ApiErrorMsg(c, "未获取到用户信息")
		return
	}

	subscriptions, err := model.GetExternalUserSubscriptions(externalUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	active, err := model.GetExternalUserActiveSubscription(externalUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"subscriptions": subscriptions,
		"active":        active,
	})
}
