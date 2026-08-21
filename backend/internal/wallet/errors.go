package wallet

import "errors"

var (
	ErrInvalidAmount    = errors.New("invalid wallet amount")
	ErrInsufficient     = errors.New("insufficient coins")
	ErrTipSelf          = errors.New("cannot tip own video")
	ErrTipTooFrequent   = errors.New("tip too frequent")
	ErrAlreadyClaimed   = errors.New("daily action already claimed")
	ErrPayNotConfigured = errors.New("alipay is not configured")
	ErrOrderNotFound    = errors.New("recharge order not found")
	ErrOrderNotOwned    = errors.New("recharge order not owned")
	ErrVideoNotTippable = errors.New("video is not tippable")
	ErrNotifyInvalid    = errors.New("alipay notify invalid")
)
