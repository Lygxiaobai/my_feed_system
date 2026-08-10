package like

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
	"my_feed_system/internal/video"
)

// Handler 负责点赞模块的 HTTP 接口。
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterProtectedRoutes 注册需要登录后才能访问的点赞接口。
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/like", h.Like)
	rg.POST("/unlike", h.Unlike)
	rg.POST("/isLiked", h.IsLiked)
	rg.POST("/listLikedVideoIDs", h.ListLikedVideoIDs)
}

func (h *Handler) Like(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	if err := h.service.Like(c.GetUint64("account_id"), req); err != nil {
		if errors.Is(err, video.ErrVideoNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		if errors.Is(err, ErrAlreadyLiked) {
			// 重复点赞对用户来说结果一致，按重复提交归类而非报错误提示。
			response.FailTip(c, http.StatusBadRequest, response.DuplicatedRequest, "已经点过赞了", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, nil)
}

func (h *Handler) Unlike(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	if err := h.service.Unlike(c.GetUint64("account_id"), req); err != nil {
		if errors.Is(err, video.ErrVideoNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		if errors.Is(err, ErrLikeNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "尚未点赞，无需取消", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, nil)
}

func (h *Handler) IsLiked(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	isLiked, err := h.service.IsLiked(c.GetUint64("account_id"), req)
	if err != nil {
		if errors.Is(err, video.ErrVideoNotFound) {
			response.FailTip(c, http.StatusNotFound, response.ResourceNotFound, "视频不存在或已被删除", err)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"is_liked": isLiked})
}

func (h *Handler) ListLikedVideoIDs(c *gin.Context) {
	var req ListLikedVideoIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParamError, err)
		return
	}

	ids, err := h.service.ListLikedVideoIDs(c.GetUint64("account_id"), req.VideoIDs)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.SystemError, err)
		return
	}

	response.OK(c, gin.H{"video_ids": ids})
}
