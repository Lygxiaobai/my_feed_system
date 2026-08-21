package history

// ShouldPersist 判断这次上报值不值得落库。
// 进度为 0 或过浅时返回 false，已有记录不得被它覆盖成「没看过」。
func ShouldPersist(positionMs, durationMs int64) bool {
	if positionMs <= 0 {
		return false
	}
	if positionMs >= MinPersistMs {
		return true
	}
	return durationMs > 0 && float64(positionMs) >= float64(durationMs)*MinPersistRatio
}

// IsCompleted 由服务端单独计算：剩余过短或进度过深都算看完。
func IsCompleted(positionMs, durationMs int64) bool {
	if durationMs <= 0 || positionMs <= 0 {
		return false
	}
	if durationMs-positionMs <= CompleteRemainMs {
		return true
	}
	return float64(positionMs) >= float64(durationMs)*CompleteRatio
}

// ShouldResume 决定详情页要不要 seek。
// 未看完、进度足够深、且离片尾仍超过完成阈值时才恢复。
func ShouldResume(positionMs, durationMs int64, completed bool) bool {
	if completed {
		return false
	}
	if positionMs < ResumeMinMs {
		return false
	}
	if durationMs > 0 && float64(positionMs) < float64(durationMs)*ResumeMinRatio {
		return false
	}
	return !IsCompleted(positionMs, durationMs)
}

// ResumeMs 是详情页应跳到的毫秒数；不恢复时为 0。
func ResumeMs(positionMs, durationMs int64, completed bool) int64 {
	if !ShouldResume(positionMs, durationMs, completed) {
		return 0
	}
	return positionMs
}

func clampPosition(positionMs, durationMs int64) int64 {
	if positionMs < 0 {
		return 0
	}
	if durationMs > 0 && positionMs > durationMs {
		return durationMs
	}
	return positionMs
}
