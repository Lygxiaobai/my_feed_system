package http

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/analytics"
	"my_feed_system/internal/audit"
	"my_feed_system/internal/comment"
	"my_feed_system/internal/config"
	"my_feed_system/internal/feed"
	"my_feed_system/internal/like"
	"my_feed_system/internal/media"
	"my_feed_system/internal/middleware/accesslog"
	jwtmiddleware "my_feed_system/internal/middleware/jwt"
	"my_feed_system/internal/middleware/ratelimit"
	"my_feed_system/internal/middleware/requestid"
	"my_feed_system/internal/mq"
	"my_feed_system/internal/observability"
	"my_feed_system/internal/ops"
	"my_feed_system/internal/popularity"
	"my_feed_system/internal/report"
	"my_feed_system/internal/response"
	"my_feed_system/internal/social"
	"my_feed_system/internal/video"
	"my_feed_system/internal/wallet"
)

func NewRouter(
	db *gorm.DB,
	redisClient redis.Cmdable,
	popularityService *popularity.Service,
	publisher *mq.Publisher,
	jwtSecret string,
	uploadDir string,
) *gin.Engine {
	return NewRouterWithLocalCaches(db, redisClient, popularityService, publisher, nil, nil, nil, jwtSecret, uploadDir, 0, config.AuditConfig{}, config.AuthConfig{}, config.OpsConfig{}, config.AlipayConfig{}, config.FeedConfig{})
}

func NewRouterWithLocalCaches(
	db *gorm.DB,
	redisClient redis.Cmdable,
	popularityService *popularity.Service,
	publisher *mq.Publisher,
	localDetailCache *video.LocalDetailCache,
	localLatestCache *feed.LocalLatestPageCache,
	localHotCache *feed.LocalHotPageCache,
	jwtSecret string,
	uploadDir string,
	maxVideoBytes int64,
	auditCfg config.AuditConfig,
	authCfg config.AuthConfig,
	opsCfg config.OpsConfig,
	alipayCfg config.AlipayConfig,
	feedCfg config.FeedConfig,
) *gin.Engine {
	// 不用 gin.Default()：它固定绑定 gin.Logger()，而后者只能输出拼好的文本行，
	// 无法交给 slog 分级和结构化。这里自行组装等价的中间件链。
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestid.New())
	r.Use(accesslog.New(accesslog.Options{
		// 健康检查每十秒一次，指标端点由 Prometheus 定期抓取，都属于记录了也无人查看的噪音。
		SkipPaths:     []string{"/ping", "/metrics", "/event/report", "/ops/gate"},
		SlowThreshold: time.Second,
	}))

	r.GET("/ping", func(c *gin.Context) {
		response.OK(c, gin.H{"message": "pong"})
	})
	r.GET("/metrics", gin.WrapH(observability.NewMetricsHandler()))
	// 仅暴露处理后的媒体目录，sources 目录只供 Worker 读取，避免原始上传文件被直接访问。
	r.Static("/static/videos", filepath.Join(uploadDir, "videos"))
	r.Static("/static/covers", filepath.Join(uploadDir, "covers"))

	tokenCache := account.NewTokenCache(redisClient)
	detailCache := video.NewDetailCache(redisClient)
	latestCache := feed.NewLatestCache(redisClient)
	hotCache := feed.NewHotPageCache(redisClient)
	// 时间线索引与 latest 页缓存配套使用：前者加速候选集读取，后者缓存最终结果页。
	timelineStore := feed.NewGlobalTimelineStore(redisClient)

	var rateLimiter ratelimit.Checker
	if redisClient != nil {
		rateLimiter = ratelimit.NewFixedWindow(redisClient)
	}

	loginIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "account.login.ip",
		Limit:    10,
		Window:   time.Minute,
		FailOpen: true,
	})
	registerIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "account.register.ip",
		Limit:    5,
		Window:   10 * time.Minute,
		FailOpen: true,
	})
	likeLikeIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "like.like.ip",
		Limit:    60,
		Window:   time.Minute,
		FailOpen: true,
	})
	likeLikeAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "like.like.account",
		Limit:    30,
		Window:   time.Minute,
		FailOpen: true,
	})
	likeUnlikeIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "like.unlike.ip",
		Limit:    60,
		Window:   time.Minute,
		FailOpen: true,
	})
	likeUnlikeAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "like.unlike.account",
		Limit:    30,
		Window:   time.Minute,
		FailOpen: true,
	})
	commentPublishIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "comment.publish.ip",
		Limit:    30,
		Window:   time.Minute,
		FailOpen: true,
	})
	commentPublishAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "comment.publish.account",
		Limit:    15,
		Window:   time.Minute,
		FailOpen: true,
	})
	commentDeleteIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "comment.delete.ip",
		Limit:    40,
		Window:   time.Minute,
		FailOpen: true,
	})
	commentDeleteAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "comment.delete.account",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})
	socialFollowIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "social.follow.ip",
		Limit:    40,
		Window:   time.Minute,
		FailOpen: true,
	})
	socialFollowAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "social.follow.account",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})
	socialUnfollowIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "social.unfollow.ip",
		Limit:    40,
		Window:   time.Minute,
		FailOpen: true,
	})
	socialUnfollowAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "social.unfollow.account",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})

	eventReportIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "event.report.ip",
		Limit:    80,
		Window:   time.Minute,
		FailOpen: true,
	})
	eventGroup := r.Group("/event")
	eventGroup.Use(jwtmiddleware.OptionalJWTAuth(jwtSecret), eventReportIPLimit)
	analytics.NewHandler().RegisterRoutes(eventGroup)

	emailSendIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "account.email.send.ip",
		Limit:    5,
		Window:   time.Minute,
		FailOpen: true,
	})
	emailVerifyIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "account.email.verify.ip",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})

	walletService := wallet.NewService(db, alipayCfg)
	walletService.SetPublisher(publisher, popularityService)
	accountService := account.NewServiceWithTokenCache(db, tokenCache, jwtSecret)
	accountService.SetCreatedHook(walletService.GrantRegisterGiftTx)
	var otpStore *account.OTPStore
	if redisClient != nil {
		ttl := time.Duration(authCfg.Email.CodeTTLSeconds) * time.Second
		otpStore = account.NewOTPStore(redisClient, ttl)
	}
	accountService.SetEmail(otpStore, &account.SMTPMailer{
		Host:     authCfg.SMTP.Host,
		Port:     authCfg.SMTP.Port,
		TLS:      authCfg.SMTP.TLS,
		User:     authCfg.SMTP.User,
		Password: authCfg.SMTP.Password,
		From:     authCfg.SMTP.From,
	}, authCfg.Email)
	accountHandler := account.NewHandler(accountService)
	accountGroup := r.Group("/account")
	accountGroup.POST("/register", registerIPLimit, accountHandler.Register)
	accountGroup.POST("/login", loginIPLimit, accountHandler.Login)
	accountGroup.POST("/email/sendCode", emailSendIPLimit, accountHandler.SendEmailCode)
	accountGroup.POST("/email/verify", emailVerifyIPLimit, accountHandler.VerifyEmail)
	accountGroup.POST("/findByID", accountHandler.FindByID)
	accountGroup.POST("/findByUsername", accountHandler.FindByUsername)

	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	accountHandler.RegisterProtectedRoutes(protectedAccountGroup)

	opsHandler := ops.NewHandler(ops.NewService(accountService, opsCfg), db, tokenCache, jwtSecret)
	opsGroup := r.Group("/ops")
	opsGroup.GET("/gate", opsHandler.Gate)
	opsProtected := opsGroup.Group("")
	opsProtected.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	opsLogsLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "ops.logs.ip",
		Limit:    30,
		Window:   time.Minute,
		FailOpen: true,
	})
	opsProtected.GET("/access", opsHandler.Access)
	opsProtected.GET("/metrics", opsHandler.Metrics)
	opsProtected.POST("/logs", opsLogsLimit, opsHandler.Logs)

	videoService := video.NewServiceWithCachesAndPublisher(
		db,
		popularityService,
		detailCache,
		localDetailCache,
		publisher,
		uploadDir,
		auditCfg.Enabled,
	)
	videoHandler := video.NewHandler(videoService, uploadDir, media.NewService(db, uploadDir, maxVideoBytes))
	videoGroup := r.Group("/video")
	// 公开路由挂可选鉴权：作者本人需要能看到自己尚未过审的内容，
	// 匿名访问则只看得到已过审的。
	videoGroup.Use(jwtmiddleware.OptionalJWTAuth(jwtSecret))
	videoHandler.RegisterRoutes(videoGroup)

	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	videoHandler.RegisterProtectedRoutes(protectedVideoGroup)

	if auditCfg.Enabled {
		auditService := audit.NewService(
			db,
			video.NewAuditStore(db),
			buildModerator(auditCfg),
			video.NewApprovalPublisher(db),
			auditCfg.ReviewerAccountIDs,
		)
		auditHandler := audit.NewHandler(auditService)
		auditGroup := r.Group("/audit")
		auditGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
		auditHandler.RegisterProtectedRoutes(auditGroup)
		slog.Info("content audit enabled")
	} else {
		slog.Info("content audit disabled, publish goes public immediately")
	}

	// 举报独立于 audit.enabled：机审是可选能力，而举报是常设的用户通道，
	// 不能因为机审关闭就连带消失——那会让平台失去接收违规通知的唯一入口。
	reportSubmitIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "report.submit.ip",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})
	reportSubmitAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "report.submit.account",
		Limit:    10,
		Window:   time.Minute,
		FailOpen: true,
	})
	// 审核员白名单复用审核配置：这里只有「是/不是审核员」一个区分，
	// 为它再引入一套配置或 RBAC 属于过度设计。
	reportHandler := report.NewHandler(report.NewService(db, videoService, auditCfg.ReviewerAccountIDs))
	reportGroup := r.Group("/report")
	reportGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	reportHandler.RegisterProtectedRoutes(reportGroup, reportSubmitIPLimit, reportSubmitAccountLimit)

	likeHandler := like.NewHandler(like.NewServiceWithCachesAndPublisher(db, popularityService, detailCache, localDetailCache, publisher))
	likeGroup := r.Group("/like")
	likeGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	likeGroup.POST("/like", likeLikeIPLimit, likeLikeAccountLimit, likeHandler.Like)
	likeGroup.POST("/unlike", likeUnlikeIPLimit, likeUnlikeAccountLimit, likeHandler.Unlike)
	likeGroup.POST("/isLiked", likeHandler.IsLiked)
	likeGroup.POST("/listLikedVideoIDs", likeHandler.ListLikedVideoIDs)

	commentHandler := comment.NewHandler(comment.NewServiceWithDetailCacheAndPublisher(db, popularityService, detailCache, publisher))
	commentGroup := r.Group("/comment")
	commentHandler.RegisterRoutes(commentGroup)

	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	protectedCommentGroup.POST("/publish", commentPublishIPLimit, commentPublishAccountLimit, commentHandler.Publish)
	protectedCommentGroup.POST("/delete", commentDeleteIPLimit, commentDeleteAccountLimit, commentHandler.Delete)

	socialHandler := social.NewHandler(social.NewServiceWithPublisher(db, publisher))
	socialGroup := r.Group("/social")
	socialHandler.RegisterRoutes(socialGroup)

	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	protectedSocialGroup.POST("/follow", socialFollowIPLimit, socialFollowAccountLimit, socialHandler.Follow)
	protectedSocialGroup.POST("/unfollow", socialUnfollowIPLimit, socialUnfollowAccountLimit, socialHandler.Unfollow)
	protectedSocialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
	protectedSocialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)

	feedService := feed.NewServiceWithCachesAndTimeline(
		db,
		popularityService,
		latestCache,
		localLatestCache,
		hotCache,
		localHotCache,
		timelineStore,
		uploadDir,
	)
	if redisClient != nil {
		fanoutCfg := feedCfg.Fanout
		fanoutCfg.ApplyDefaults()
		feedService = feedService.WithFollowingFanout(&feed.FollowingFanout{
			Inbox:          feed.NewInboxStore(redisClient, fanoutCfg.InboxMaxSize, fanoutCfg.ActiveTTL()),
			Outbox:         feed.NewOutboxStore(redisClient, fanoutCfg.OutboxMaxSize),
			Following:      feed.NewFollowingCache(redisClient, 0),
			PullThreshold:  fanoutCfg.PullThreshold,
			MaxPullAuthors: fanoutCfg.MaxPullAuthors,
		})
	}
	feedHandler := feed.NewHandler(feedService)
	feedGroup := r.Group("/feed")
	feedHandler.RegisterRoutes(feedGroup)

	protectedFeedGroup := feedGroup.Group("")
	protectedFeedGroup.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	feedHandler.RegisterProtectedRoutes(protectedFeedGroup)

	walletTipIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "wallet.tip.ip",
		Limit:    40,
		Window:   time.Minute,
		FailOpen: true,
	})
	walletTipAccountLimit := ratelimit.ByAccountID(rateLimiter, ratelimit.Policy{
		Name:     "wallet.tip.account",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})
	walletPayIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "wallet.recharge.ip",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})
	walletDailyIPLimit := ratelimit.ByIP(rateLimiter, ratelimit.Policy{
		Name:     "wallet.daily.ip",
		Limit:    20,
		Window:   time.Minute,
		FailOpen: true,
	})

	walletHandler := wallet.NewHandler(walletService)
	walletGroup := r.Group("/wallet")
	walletHandler.RegisterPublicRoutes(walletGroup)
	protectedWallet := walletGroup.Group("")
	protectedWallet.Use(jwtmiddleware.JWTAuthWithTokenCache(db, tokenCache, jwtSecret))
	protectedWallet.POST("/summary", walletHandler.Summary)
	protectedWallet.POST("/ledger", walletHandler.ListLedger)
	protectedWallet.POST("/checkin", walletDailyIPLimit, walletHandler.Checkin)
	protectedWallet.POST("/checkin/month", walletHandler.CheckinMonth)
	protectedWallet.POST("/lottery", walletDailyIPLimit, walletHandler.Lottery)
	protectedWallet.POST("/recharge/create", walletPayIPLimit, walletHandler.CreateRecharge)
	protectedWallet.POST("/recharge/query", walletHandler.QueryOrder)
	protectedWallet.POST("/tip", walletTipIPLimit, walletTipAccountLimit, walletHandler.Tip)
	protectedWallet.POST("/tips/mine", walletHandler.ListMyTips)
	protectedWallet.POST("/tips/byVideo", walletHandler.ListVideoTips)

	return r
}

// buildModerator 按配置装配审核实现。
//
// 当前只有本地词库；日后接入云厂商时在这里替换或用装饰器串联即可，
// 业务侧的状态机、流水与查询过滤都不需要改动。
func buildModerator(cfg config.AuditConfig) audit.Moderator {
	blockWords, err := audit.LoadWordFile(cfg.BlockWordFile)
	if err != nil {
		slog.Warn("load block word file failed, continuing without it",
			slog.String("path", cfg.BlockWordFile), slog.String("error", err.Error()))
	}
	reviewWords, err := audit.LoadWordFile(cfg.ReviewWordFile)
	if err != nil {
		slog.Warn("load review word file failed, continuing without it",
			slog.String("path", cfg.ReviewWordFile), slog.String("error", err.Error()))
	}

	slog.Info("content moderator ready",
		slog.Int("block_words", len(blockWords)),
		slog.Int("review_words", len(reviewWords)),
		slog.String("media_policy", cfg.MediaPolicy))

	return audit.NewKeywordModerator(blockWords, reviewWords, audit.MediaPolicy(cfg.MediaPolicy))
}
