package wallet

import (
	"context"
	"log/slog"
	"time"
)

const defaultSweepInterval = 30 * time.Second

// ExpirePoller 定期清零到期批次并关闭超时未支付订单。
type ExpirePoller struct {
	service  *Service
	interval time.Duration
}

func NewExpirePoller(service *Service) *ExpirePoller {
	return &ExpirePoller{service: service, interval: defaultSweepInterval}
}

func (p *ExpirePoller) Run(ctx context.Context) {
	if p.service == nil {
		return
	}
	p.service.Sweep(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.service.Sweep(ctx)
			slog.DebugContext(ctx, "wallet expire sweep done")
		}
	}
}
