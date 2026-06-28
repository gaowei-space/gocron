package agent

import (
	"strconv"
	"strings"
	"time"

	"github.com/ouqiang/gocron/internal/models"
	"github.com/ouqiang/gocron/internal/modules/logger"
	"github.com/ouqiang/gocron/internal/modules/utils"
	"github.com/ouqiang/gocron/internal/routers/user"
	"github.com/ouqiang/gocron/internal/service"
	"gopkg.in/macaron.v1"
)

const SourceAgentAPI = "agent_api"

type deviceStartResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func StartDeviceAuthorization(ctx *macaron.Context) string {
	deviceCode := service.NewAgentDeviceCode()
	userCode := service.NewAgentUserCode()
	expiresAt := time.Now().Add(service.AgentDeviceCodeDuration)
	req := models.AgentDeviceAuthorizationRequest{
		DeviceCodeHash: service.HashAgentToken(deviceCode),
		UserCodeHash:   service.HashAgentToken(userCode),
		DeviceName:     strings.TrimSpace(ctx.Query("device_name")),
		ClientType:     defaultString(strings.TrimSpace(ctx.Query("client_type")), "gocron-cli"),
		ClientVersion:  strings.TrimSpace(ctx.Query("client_version")),
		Status:         models.AgentAuthStatusPending,
		ExpiresAt:      expiresAt,
	}
	json := utils.JsonResponse{}
	if _, err := req.Create(); err != nil {
		logger.Error(err)
		Audit(ctx, "auth.device.start", "device_authorization_request", "", "create device authorization request", false, err.Error())
		return json.CommonFailure("创建授权请求失败")
	}

	verificationURI := "/#/agent/authorize"
	resp := deviceStartResponse{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + userCode,
		ExpiresIn:               int64(service.AgentDeviceCodeDuration / time.Second),
		Interval:                service.AgentDeviceCodePollInterval,
	}

	result := json.Success(utils.SuccessContent, resp)
	Audit(ctx, "auth.device.start", "device_authorization_request", "", req.ClientType, true, "")
	return result
}

func ApproveDeviceAuthorization(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	if err := user.RestoreToken(ctx); err != nil || !user.IsSuperAdmin(ctx) {
		Audit(ctx, "auth.device.approve", "device_authorization_request", "", "approve device authorization", false, "not super admin")
		return json.Failure(utils.UnauthorizedError, "仅超级管理员可以授权CLI")
	}
	userCode := strings.TrimSpace(ctx.Query("user_code"))
	if userCode == "" {
		return json.CommonFailure("授权码不能为空")
	}
	req := new(models.AgentDeviceAuthorizationRequest)
	exists, err := req.FindByUserCodeHash(service.HashAgentToken(userCode))
	if err != nil || !exists {
		return json.CommonFailure("授权请求不存在")
	}
	if req.Status != models.AgentAuthStatusPending {
		return json.CommonFailure("授权请求已处理")
	}
	if time.Now().After(req.ExpiresAt) {
		req.UpdateById(req.Id, models.CommonMap{"status": models.AgentAuthStatusExpired})
		return json.CommonFailure("授权请求已过期")
	}
	_, err = req.UpdateById(req.Id, models.CommonMap{
		"status":      models.AgentAuthStatusApproved,
		"approved_by": user.Uid(ctx),
	})
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("授权失败")
	}

	result := json.Success("授权成功", nil)
	Audit(ctx, "auth.device.approve", "device_authorization_request", strconv.Itoa(req.Id), "approve device authorization", true, "")
	return result
}

func DeviceToken(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	deviceCode := strings.TrimSpace(ctx.Query("device_code"))
	if deviceCode == "" {
		return json.CommonFailure("device_code不能为空")
	}
	req := new(models.AgentDeviceAuthorizationRequest)
	exists, err := req.FindByDeviceCodeHash(service.HashAgentToken(deviceCode))
	if err != nil || !exists {
		return json.CommonFailure("授权请求不存在")
	}
	if time.Now().After(req.ExpiresAt) {
		req.UpdateById(req.Id, models.CommonMap{"status": models.AgentAuthStatusExpired})
		return json.CommonFailure("授权请求已过期")
	}
	if !req.LastPolledAt.IsZero() && time.Since(req.LastPolledAt) < time.Duration(service.AgentDeviceCodePollInterval)*time.Second {
		return json.Failure(utils.ResponseFailure, "轮询过于频繁")
	}
	req.UpdateById(req.Id, models.CommonMap{
		"last_polled_at": time.Now(),
		"poll_count":     req.PollCount + 1,
	})
	if req.Status == models.AgentAuthStatusPending {
		return json.CommonFailure("授权待确认")
	}
	if req.Status != models.AgentAuthStatusApproved {
		return json.CommonFailure("授权请求已处理")
	}
	admin := new(models.User)
	if err := admin.Find(req.ApprovedBy); err != nil || admin.Id == 0 || admin.Status != models.Enabled || admin.IsAdmin != 2 {
		return json.Failure(utils.UnauthorizedError, "授权用户无效")
	}
	claimed, err := req.ClaimApproved(req.Id)
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("授权请求处理失败")
	}
	if !claimed {
		return json.CommonFailure("授权请求已处理")
	}

	deviceId := service.NewAgentDeviceId()
	refreshToken := service.NewAgentRefreshToken()
	device := models.AgentDeviceAuthorization{
		UserId:           admin.Id,
		DeviceId:         deviceId,
		DeviceName:       req.DeviceName,
		ClientType:       req.ClientType,
		ClientVersion:    req.ClientVersion,
		RefreshTokenHash: service.HashAgentToken(refreshToken),
		ExpiresAt:        time.Now().Add(service.AgentDeviceAuthorizationDuration),
		LastUsedAt:       time.Now(),
		LastUsedIp:       ctx.RemoteAddr(),
	}
	if _, err := device.Create(); err != nil {
		logger.Error(err)
		return json.CommonFailure("创建设备授权失败")
	}
	accessToken, err := service.GenerateAgentAccessToken(admin.Id, deviceId, req.ClientType, req.ClientVersion)
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("生成access token失败")
	}

	result := json.Success(utils.SuccessContent, tokenResponse(accessToken, refreshToken, deviceId))
	Audit(ctx, "auth.device.token", "device_authorization", deviceId, "exchange device code", true, "")
	return result
}

func RefreshToken(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	refreshToken := strings.TrimSpace(ctx.Query("refresh_token"))
	if refreshToken == "" {
		return json.CommonFailure("refresh_token不能为空")
	}
	device := new(models.AgentDeviceAuthorization)
	exists, err := device.FindByRefreshTokenHash(service.HashAgentToken(refreshToken))
	if err != nil || !exists {
		replayed := new(models.AgentDeviceAuthorization)
		if replayExists, replayErr := replayed.FindByPreviousTokenHash(service.HashAgentToken(refreshToken)); replayErr == nil && replayExists {
			if service.ShouldRevokeAgentRefreshReplay(replayed.UpdatedAt, time.Now()) {
				replayed.UpdateById(replayed.Id, models.CommonMap{"revoked_at": time.Now()})
				Audit(ctx, "auth.token.refresh", "device_authorization", replayed.DeviceId, "refresh token replay", false, "refresh token replay")
			} else {
				Audit(ctx, "auth.token.refresh", "device_authorization", replayed.DeviceId, "refresh token replay grace", false, "refresh token replay grace")
			}
		} else {
			Audit(ctx, "auth.token.refresh", "device_authorization", "", "refresh token invalid", false, "refresh token invalid")
		}
		return json.Failure(utils.AuthError, "refresh token无效")
	}
	if !deviceUsable(device) {
		return json.Failure(utils.AuthError, "设备授权无效")
	}
	userModel := new(models.User)
	if err := userModel.Find(device.UserId); err != nil || !superAdminUsable(userModel) {
		return json.Failure(utils.AuthError, "授权用户无效")
	}
	session := service.AgentDeviceSession{
		RefreshTokenHash:  device.RefreshTokenHash,
		PreviousTokenHash: device.PreviousTokenHash,
	}
	nextRefreshToken, err := service.RotateAgentRefreshToken(&session, refreshToken)
	if err != nil {
		return json.Failure(utils.AuthError, "refresh token无效")
	}
	_, err = device.UpdateById(device.Id, models.CommonMap{
		"refresh_token_hash":  session.RefreshTokenHash,
		"previous_token_hash": session.PreviousTokenHash,
		"last_used_at":        time.Now(),
		"last_used_ip":        ctx.RemoteAddr(),
	})
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("刷新token失败")
	}
	accessToken, err := service.GenerateAgentAccessToken(device.UserId, device.DeviceId, device.ClientType, device.ClientVersion)
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("生成access token失败")
	}

	result := json.Success(utils.SuccessContent, tokenResponse(accessToken, nextRefreshToken, device.DeviceId))
	Audit(ctx, "auth.token.refresh", "device_authorization", device.DeviceId, "refresh access token", true, "")
	return result
}

func Logout(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	deviceId := agentDeviceId(ctx)
	if deviceId == "" {
		return json.Failure(utils.AuthError, "认证失败")
	}
	device := new(models.AgentDeviceAuthorization)
	exists, err := device.FindByDeviceId(deviceId)
	if err != nil || !exists {
		return json.Failure(utils.AuthError, "设备授权不存在")
	}
	_, err = device.UpdateById(device.Id, models.CommonMap{"revoked_at": time.Now()})
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("退出失败")
	}
	result := json.Success(utils.SuccessContent, nil)
	Audit(ctx, "auth.logout", "device_authorization", deviceId, "logout device", true, "")
	return result
}

func Devices(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	deviceModel := new(models.AgentDeviceAuthorization)
	devices, err := deviceModel.List(models.CommonMap{})
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("获取设备授权失败")
	}
	return json.Success(utils.SuccessContent, devices)
}

func RevokeDevice(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	deviceId := strings.TrimSpace(ctx.Params(":device_id"))
	device := new(models.AgentDeviceAuthorization)
	exists, err := device.FindByDeviceId(deviceId)
	if err != nil || !exists {
		return json.CommonFailure("设备授权不存在")
	}
	_, err = device.UpdateById(device.Id, models.CommonMap{"revoked_at": time.Now()})
	if err != nil {
		logger.Error(err)
		return json.CommonFailure("撤销失败")
	}
	result := json.Success(utils.SuccessContent, nil)
	Audit(ctx, "auth.device.revoke", "device_authorization", deviceId, "revoke device", true, "")
	return result
}

func tokenResponse(accessToken, refreshToken, deviceId string) map[string]interface{} {
	return map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int64(service.AgentAccessTokenDuration / time.Second),
		"device_id":     deviceId,
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func superAdminUsable(userModel *models.User) bool {
	return userModel != nil && userModel.Id > 0 && userModel.Status == models.Enabled && userModel.IsAdmin == 2
}

func deviceUsable(device *models.AgentDeviceAuthorization) bool {
	if device == nil || device.Id == 0 {
		return false
	}
	if !device.RevokedAt.IsZero() {
		return false
	}
	if time.Now().After(device.ExpiresAt) {
		return false
	}
	return true
}
