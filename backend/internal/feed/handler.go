package feed

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

// Handler 负责信息流模块的 HTTP 接口。
type Handler struct {
	service *Service
}

// NewHandler 创建信息流接口处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册匿名可访问的信息流接口。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/listLatest", h.ListLatest)
	rg.POST("/listLikesCount", h.ListLikesCount)
	rg.POST("/listByPopularity", h.ListByPopularity)
}

// RegisterProtectedRoutes 注册需要登录后才能访问的信息流接口。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/listByFollowing", h.ListByFollowing)
}

// ListLatest 按发布时间倒序返回推荐流。
func (h *Handler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.ListLatest(req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, result)
}

// ListLikesCount 按点赞数倒序返回排行榜。
func (h *Handler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.ListLikesCount(req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, result)
}

// ListByFollowing 返回当前用户关注作者的视频流。
func (h *Handler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.ListByFollowing(c.GetUint64("account_id"), req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, result)
}

// ListByPopularity 返回基于热度聚合结果的热榜。
func (h *Handler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	result, err := h.service.ListByPopularity(req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, result)
}
