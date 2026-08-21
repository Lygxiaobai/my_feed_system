---
title: dm
status: active
code:
  - backend/internal/dm/service.go
related:
  - backend/internal/dm/entity.go
  - backend/internal/dm/repo.go
  - backend/internal/dm/handler.go
  - backend/internal/social/repo.go
  - backend/internal/http/router.go
  - backend/internal/db/db.go
---
# dm

## raw source
登录用户可以和另一个账号私聊。互相关注后条数不限；未关注或只单向关注时，每名发送者对同一对象只能发出一条。

## expanded spec
私信是独立的会话域，不是互动通知的投影。点赞、评论、关注、打赏仍只进入 `notification`；私信不写 outbox、不进 Worker、不走 WebSocket。发送在接口事务里落库后立即对发送者可见。

一条会话对应一对账号，用较小的账号 id 和较大的账号 id 做唯一键，避免正反向各建一行。会话在第一条私信写入时创建。列表、未读计数和已读游标都按会话两侧分开保存，打开会话会清掉自己这一侧的未读，并把已读时间推到现在；对方打开后，自己发出的消息可以显示为已读。

不能给自己发私信。对方账号不存在则不能发送。正文去掉首尾空白后不能为空，最长 500 字。互关用 `social_relations` 的双向存在判定：我关注对方且对方关注我才是好友。未互关时，额度按「我已经向对方发出的条数」计算，对方回一条不会解锁我继续连发；双方后来互关，才恢复不限条。取关后额度规则立刻按当前关系重新生效。

收件箱、会话、未读和发送都只对调用者自己可见。本轮只支持文本，不支持图片、分享卡片或撤回。

## change rules
改额度、互关判定或已读语义必须同步 `frontend/message-ui`。把私信并进通知表或改成异步投递，属于新能力，必须先改本 spec。
