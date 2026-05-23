package agent

import (
	"encoding/json"
	"strings"

	"github.com/ouqiang/gocron/internal/models"
	"github.com/ouqiang/gocron/internal/modules/logger"
	"github.com/ouqiang/gocron/internal/modules/utils"
	"github.com/ouqiang/gocron/internal/routers/user"
	"gopkg.in/macaron.v1"
)

const ctxRequestId = "agent_request_id"

func Context(ctx *macaron.Context) {
	requestId := strings.TrimSpace(ctx.Req.Header.Get("X-Request-Id"))
	if requestId == "" {
		requestId = utils.RandAuthToken()
	}
	ctx.Data[ctxRequestId] = requestId
	ctx.Resp.Header().Set("X-Request-Id", requestId)
}

func Audit(ctx *macaron.Context, action, targetType, targetId, summary string, success bool, errMsg string) {
	audit := models.AgentAuditLog{
		RequestId:      requestId(ctx),
		UserId:         user.Uid(ctx),
		DeviceId:       agentDeviceId(ctx),
		ClientType:     ctxString(ctx, ctxAgentClientType),
		ClientVersion:  ctxString(ctx, ctxAgentClientVersion),
		SourceIp:       ctx.RemoteAddr(),
		Source:         SourceAgentAPI,
		Action:         action,
		TargetType:     targetType,
		TargetId:       targetId,
		RequestSummary: truncate(summary, 512),
		Success:        success,
		ErrorMessage:   truncate(errMsg, 512),
	}
	if _, err := audit.Create(); err != nil {
		logger.Error("写入agent审计日志失败", err)
	}
}

func AuditResponse(ctx *macaron.Context, action, targetType, targetId, summary, response string) string {
	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal([]byte(response), &parsed)
	Audit(ctx, action, targetType, targetId, summary, parsed.Code == utils.ResponseSuccess, parsed.Message)
	return response
}

func requestId(ctx *macaron.Context) string {
	return ctxString(ctx, ctxRequestId)
}

func ctxString(ctx *macaron.Context, key string) string {
	value, ok := ctx.Data[key]
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
