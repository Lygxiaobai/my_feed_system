package comment

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
	"my_feed_system/internal/video"
)

// Handler 负责评论模块的 HTTP 接口。
type Handler struct {
	service *Service
}

// NewHandler 创建评论接口处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册匿名可访问的评论接口。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/listAll", h.ListAll)
}

// RegisterProtectedRoutes 注册需要登录后才能访问的评论接口。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/publish", h.Publish)
	rg.POST("/delete", h.Delete)
}

// ListAll 返回某个视频下的根评论及其回复列表。
func (h *Handler) ListAll(c *gin.Context) {
	var req ListAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	comments, err := h.service.ListAll(req)
	if err != nil {
		if errors.Is(err, video.ErrVideoNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"comments": comments})
}

// Publish 发布一条新评论或回复评论。
func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	comment, err := h.service.Publish(c.GetUint64("account_id"), c.GetString("account_username"), req)
	if err != nil {
		if errors.Is(err, video.ErrVideoNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		if errors.Is(err, ErrInvalidParentComment) || errors.Is(err, ErrParentCommentMismatch) {
			response.FailTip(c, http.StatusBadRequest, response.ParamError, "回复的评论不存在或已被删除", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"comment": comment})
}

// Delete 删除评论，并在需要时一并移除其回复树。
func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	if err := h.service.Delete(c.GetUint64("account_id"), req); err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "评论不存在或已被删除", err)
			return
		}
		if errors.Is(err, ErrCommentForbidden) {
			response.FailTip(c, http.StatusForbidden, response.AccessDenied, "只能删除自己的评论或自己视频下的评论", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, nil)
}
