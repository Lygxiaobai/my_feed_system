package danmaku

import "my_feed_system/internal/video"

// VideoAccess 只回答「这个查看者能不能看见这条视频」。
// 弹幕读写必须走和详情页同一套可见性，否则未过审内容会从弹幕接口漏出去。
type VideoAccess interface {
	EnsureVisible(viewerID uint64, videoID uint64) error
}

type videoAccess struct {
	svc *video.Service
}

func NewVideoAccess(svc *video.Service) VideoAccess {
	return videoAccess{svc: svc}
}

func (a videoAccess) EnsureVisible(viewerID uint64, videoID uint64) error {
	_, err := a.svc.GetDetail(viewerID, video.GetDetailRequest{ID: videoID})
	return err
}
