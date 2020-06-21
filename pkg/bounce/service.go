package bounce

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"moul.io/banner"
)

type Service struct {
	logger    *zap.Logger
	opts      Opts
	ctx       context.Context
	cancel    func()
	startedAt time.Time

	/// drivers

	discord discordDriver
	server  serverDriver
}

func New(opts Opts) Service {
	opts.applyDefaults()
	fmt.Fprintln(os.Stderr, banner.Inline("moul-bot"))
	ctx, cancel := context.WithCancel(opts.Context)
	svc := Service{
		logger:    opts.Logger,
		opts:      opts,
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
	}
	svc.logger.Info("service initialized", zap.Bool("dev-mode", opts.DevMode))
	return svc
}

func (svc *Service) Close() {
	svc.logger.Debug("closing service")
	svc.cancel()
	fmt.Fprintln(os.Stderr, banner.Inline("kthxbie"))
}
