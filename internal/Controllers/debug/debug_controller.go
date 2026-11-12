package debug

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	svc "patreonoj/internal/services/debug"
	"strconv"
)

type Controller struct{ Service *svc.Service }

func (c *Controller) fail(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, svc.ErrInvalid):
		shared.APIError(ctx, 400, "自测参数无效：代码最多 256 KB，输入和预期输出各最多 64 KB")
	case errors.Is(err, svc.ErrBusy):
		ctx.Header("Retry-After", "2")
		shared.APIError(ctx, 429, "自测资源繁忙，请等待当前运行完成后再试")
	case errors.Is(err, svc.ErrUnavailable):
		shared.APIError(ctx, 503, "自测服务暂不可用，请稍后重试")
	case errors.Is(err, svc.ErrQuestionNotFound):
		shared.APIError(ctx, 404, "题目不存在或不可访问")
	case errors.Is(err, svc.ErrNotFound):
		shared.APIError(ctx, 404, "自测记录不存在或已过期，请重新运行")
	default:
		shared.APIError(ctx, 500, "自测请求失败")
	}
}

func (c *Controller) Create(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20)
	var request svc.Request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		c.fail(ctx, svc.ErrInvalid)
		return
	}
	result, err := c.Service.Create(ctx.Request.Context(), ctx.MustGet("currentUser").(models.User), request)
	if err != nil {
		c.fail(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 202, "自测已加入队列", gin.H{"data": result})
}

func (c *Controller) Get(ctx *gin.Context) {
	result, err := c.Service.Get(ctx.MustGet("currentUser").(models.User).UUID, ctx.Param("id"))
	if err != nil {
		c.fail(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 200, "ok", gin.H{"data": result})
}

func (c *Controller) Cancel(ctx *gin.Context) {
	result, err := c.Service.Cancel(ctx.MustGet("currentUser").(models.User).UUID, ctx.Param("id"))
	if err != nil {
		c.fail(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 200, "已停止自测", gin.H{"data": result})
}

func (c *Controller) Samples(ctx *gin.Context) {
	pk, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || pk < 1 {
		c.fail(ctx, svc.ErrQuestionNotFound)
		return
	}
	user, _ := ctx.Get("currentUser")
	viewer, _ := user.(models.User)
	result, err := c.Service.Samples(ctx.Request.Context(), viewer, pk)
	if err != nil {
		c.fail(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 200, "ok", gin.H{"data": result})
}
