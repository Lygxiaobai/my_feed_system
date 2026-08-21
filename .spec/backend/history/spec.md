---
title: history
status: active
code:
  - backend/internal/history/service.go
related:
  - backend/internal/history/handler.go
  - backend/internal/history/repo.go
  - backend/internal/history/entity.go
  - backend/internal/history/classify.go
  - backend/internal/video/service.go
---
# history

## raw source
登录用户可以记录自己看过的视频进度，按未看完和已看完列出，并在再次打开详情时恢复未看完的位置。

## expanded spec
浏览历史是账号私有状态，不是产品埋点。写入走独立的 upsert，不进入 outbox、不写 Redis、不复用 `/event/report`。每个账号对每条视频只保留一行，后一次有效上报覆盖前一次。

只有登录用户会写入服务端。上报携带播放进度和片长，不携带「是否看完」；完成态由服务端按阈值计算。进度过浅（不足 3 秒且不足片长 20%）不落库，也不得覆盖已有记录。剩余不超过 2 秒或进度达到 95% 视为已看完，并把存库进度清零，下次从头播放。循环回到片头时，客户端应先用回跳前的进度上报；服务端把进度 0 当作无效上报，避免已看完被改回未看完。

列表按未看完 / 已看完分页，新看的在前。批量读取进度只返回调用者仍能看见的视频。看不见的视频对外仍是「不存在」，历史行在列表里直接丢掉。浏览历史没有用户删除入口，记录只随观看进度更新。

详情页是否 seek 也由服务端给出 `resume_ms`：未看完、进度至少 2 秒、至少片长 10%、且尚未达到完成阈值时才恢复。信息流首次滑到一条视频不得按历史进度起播。
