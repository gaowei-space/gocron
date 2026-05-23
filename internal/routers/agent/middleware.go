package agent

import (
	"strings"
	"time"

	"github.com/ouqiang/gocron/internal/models"
	"github.com/ouqiang/gocron/internal/modules/utils"
	"github.com/ouqiang/gocron/internal/routers/user"
	"github.com/ouqiang/gocron/internal/service"
	"gopkg.in/macaron.v1"
)

const (
	ctxAgentDeviceId      = "agent_device_id"
	ctxAgentClientType    = "agent_client_type"
	ctxAgentClientVersion = "agent_client_version"
)

func Auth(ctx *macaron.Context) {
	json := utils.JsonResponse{}
	header := strings.TrimSpace(ctx.Req.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		ctx.Write([]byte(json.Failure(utils.AuthError, "认证失败")))
		return
	}
	claims, err := service.ParseAgentAccessToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		ctx.Write([]byte(json.Failure(utils.AuthError, "认证失败")))
		return
	}
	deviceId, _ := claims["device_id"].(string)
	clientType, _ := claims["client_type"].(string)
	clientVersion, _ := claims["client_version"].(string)
	if deviceId == "" {
		ctx.Write([]byte(json.Failure(utils.AuthError, "认证失败")))
		return
	}
	device := new(models.AgentDeviceAuthorization)
	exists, err := device.FindByDeviceId(deviceId)
	if err != nil || !exists || !deviceUsable(device) {
		ctx.Write([]byte(json.Failure(utils.AuthError, "设备授权无效")))
		return
	}
	userModel := new(models.User)
	if err := userModel.Find(device.UserId); err != nil || !superAdminUsable(userModel) {
		ctx.Write([]byte(json.Failure(utils.AuthError, "授权用户无效")))
		return
	}
	device.UpdateById(device.Id, models.CommonMap{
		"last_used_at": time.Now(),
		"last_used_ip": ctx.RemoteAddr(),
	})
	ctx.Data["uid"] = userModel.Id
	ctx.Data["username"] = userModel.Name
	ctx.Data["is_admin"] = int(userModel.IsAdmin)
	ctx.Data[ctxAgentDeviceId] = deviceId
	ctx.Data[ctxAgentClientType] = clientType
	ctx.Data[ctxAgentClientVersion] = clientVersion
}

func WebSuperAdminAuth(ctx *macaron.Context) {
	json := utils.JsonResponse{}
	if err := user.RestoreToken(ctx); err != nil || !user.IsSuperAdmin(ctx) {
		ctx.Write([]byte(json.Failure(utils.UnauthorizedError, "仅超级管理员可以操作")))
		return
	}
}

func agentDeviceId(ctx *macaron.Context) string {
	value, ok := ctx.Data[ctxAgentDeviceId]
	if !ok {
		return ""
	}
	if deviceId, ok := value.(string); ok {
		return deviceId
	}
	return ""
}
