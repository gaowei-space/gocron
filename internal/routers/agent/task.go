package agent

import (
	"strconv"

	"github.com/go-macaron/binding"
	"github.com/ouqiang/gocron/internal/models"
	"github.com/ouqiang/gocron/internal/modules/logger"
	"github.com/ouqiang/gocron/internal/modules/utils"
	"github.com/ouqiang/gocron/internal/routers/base"
	"github.com/ouqiang/gocron/internal/routers/host"
	"github.com/ouqiang/gocron/internal/routers/task"
	"github.com/ouqiang/gocron/internal/service"
	"gopkg.in/macaron.v1"
)

func TaskList(ctx *macaron.Context) string {
	return task.Index(ctx)
}

func TaskDetail(ctx *macaron.Context) string {
	return task.Detail(ctx)
}

func TaskCreate(ctx *macaron.Context, form task.TaskForm) string {
	form.Id = 0
	return AuditResponse(ctx, "task.create", "task", "", "create task", task.Store(ctx, form))
}

func TaskUpdate(ctx *macaron.Context, form task.TaskForm) string {
	form.Id = ctx.ParamsInt(":id")
	return AuditResponse(ctx, "task.update", "task", ctx.Params(":id"), "update task", task.Store(ctx, form))
}

func TaskEnable(ctx *macaron.Context) string {
	return AuditResponse(ctx, "task.enable", "task", ctx.Params(":id"), "enable task", task.Enable(ctx))
}

func TaskDisable(ctx *macaron.Context) string {
	return AuditResponse(ctx, "task.disable", "task", ctx.Params(":id"), "disable task", task.Disable(ctx))
}

func TaskRun(ctx *macaron.Context) string {
	return AuditResponse(ctx, "task.run", "task", ctx.Params(":id"), "run task", task.Run(ctx))
}

func TaskLogs(ctx *macaron.Context) string {
	jsonResp := utils.JsonResponse{}
	logModel := new(models.TaskLog)
	params := models.CommonMap{}
	params["TaskId"] = ctx.ParamsInt(":id")
	params["Protocol"] = ctx.QueryInt("protocol")
	status := ctx.QueryInt("status")
	if status >= 0 {
		status -= 1
	}
	params["Status"] = status
	base.ParsePageAndPageSize(ctx, params)
	total, err := logModel.Total(params)
	if err != nil {
		logger.Error(err)
	}
	logs, err := logModel.List(params)
	if err != nil {
		logger.Error(err)
	}

	return jsonResp.Success(utils.SuccessContent, map[string]interface{}{
		"total": total,
		"data":  logs,
	})
}

func TaskStop(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	taskId := ctx.ParamsInt(":task_id")
	logId, err := strconv.ParseInt(ctx.Params(":log_id"), 10, 64)
	if err != nil || logId <= 0 {
		return json.CommonFailure("log_id参数错误")
	}
	logModel := new(models.TaskLog)
	exists, err := models.Db.Id(logId).Get(logModel)
	if err != nil {
		return json.CommonFailure("获取任务日志失败", err)
	}
	if !exists || logModel.Id == 0 {
		return json.CommonFailure("任务日志不存在")
	}
	if logModel.TaskId != taskId {
		return json.CommonFailure("任务日志不属于该任务")
	}
	if logModel.Status != models.Running {
		return json.CommonFailure("任务日志不是运行中状态")
	}
	taskModel := new(models.Task)
	taskDetail, err := taskModel.Detail(taskId)
	if err != nil || taskDetail.Id <= 0 {
		return json.CommonFailure("获取任务信息失败", err)
	}
	if taskDetail.Protocol != models.TaskRPC {
		return json.CommonFailure("仅支持SHELL任务手动停止")
	}
	if len(taskDetail.Hosts) == 0 {
		return json.CommonFailure("任务节点列表为空")
	}
	for _, h := range taskDetail.Hosts {
		service.ServiceTask.Stop(h.Name, h.Port, logId)
	}

	result := json.Success("已执行停止操作, 请等待任务退出", nil)
	return AuditResponse(ctx, "task.stop", "task_run", ctx.Params(":log_id"), "stop task run", result)
}

func HostList(ctx *macaron.Context) string {
	return host.All(ctx)
}

func BindTaskForm() macaron.Handler {
	return binding.Bind(task.TaskForm{})
}
