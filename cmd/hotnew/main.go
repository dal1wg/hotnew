package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hotnew/internal/app"
	"hotnew/internal/config"
	"hotnew/internal/distribute"
	apihttp "hotnew/internal/distribute/http"
	"hotnew/internal/domain"
	"hotnew/internal/normalize"
	"hotnew/internal/source/rss"
	"hotnew/internal/store"
	"hotnew/internal/summarize"
)

func main() {
	cfg := config.Load()
	articleStore, deliveryStore, retryQueue, cleanupStores := mustBuildStores(cfg)
	defer cleanupStores()

	registry := store.NewMemorySourceRegistry()
	if err := registry.RegisterDefaults(cfg.Sources); err != nil {
		log.Fatalf("register default sources: %v", err)
	}

	trackedDistributor, retryChannels, cleanupDistributor := mustBuildDistributor(cfg, deliveryStore, retryQueue)
	defer cleanupDistributor()

	pipeline := app.NewPipeline(normalize.NewService(), summarize.NewRuleSummarizer(cfg.Summary.MaxChars), articleStore, trackedDistributor)
	for _, sc := range cfg.Sources {
		if sc.Enabled {
			pipeline.AddSource(rss.NewSource(sc))
		}
	}

	runner := app.NewRunner(pipeline)
	retryProcessor := app.NewRetryProcessor(articleStore, deliveryStore, retryQueue, retryChannels, cfg.Retry.Backoff, cfg.Retry.MaxBackoff)

	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var scheduler *app.Scheduler
	if cfg.Scheduler.Enabled {
		scheduler = app.NewScheduler(runner, cfg.Scheduler.Interval, cfg.Scheduler.RunLimit, cfg.Scheduler.RunTimeout, cfg.Scheduler.StartImmediately)
		scheduler.Start(baseCtx)
	}

	var retryWorker *app.RetryWorker
	if cfg.Retry.Enabled {
		retryWorker = app.NewRetryWorker(retryProcessor, cfg.Retry.Interval, cfg.Retry.BatchSize, cfg.Retry.Timeout)
		retryWorker.Start(baseCtx)
	}

	server := apihttp.NewServer(cfg, runner, retryProcessor, articleStore, deliveryStore, retryQueue, registry)
	httpServer := &http.Server{Addr: cfg.HTTP.Addr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Printf("hotnew listening on %s", cfg.HTTP.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-baseCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if retryWorker != nil {
		if err := retryWorker.Stop(shutdownCtx); err != nil {
			log.Printf("retry worker stop error: %v", err)
		}
	}
	if scheduler != nil {
		if err := scheduler.Stop(shutdownCtx); err != nil {
			log.Printf("scheduler stop error: %v", err)
		}
	}
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func mustBuildStores(cfg config.Config) (domain.ArticleStore, domain.DeliveryStore, domain.RetryQueue, func()) {
	switch cfg.Store.Backend {
	case "memory":
		return store.NewMemoryArticleStore(), store.NewMemoryDeliveryStore(), store.NewMemoryRetryQueue(), func() {}
	case "file":
		articleStore, err := store.NewFileArticleStoreAt(cfg.Store.FileArticlesPath)
		if err != nil {
			log.Fatalf("create file article store: %v", err)
		}
		deliveryStore, err := store.NewFileDeliveryStoreAt(cfg.Store.FileDeliveriesPath)
		if err != nil {
			log.Fatalf("create file delivery store: %v", err)
		}
		retryQueue, err := store.NewFileRetryQueueAt(cfg.Store.FileRetriesPath, cfg.Store.FileRetriesArchivePath)
		if err != nil {
			log.Fatalf("create file retry queue: %v", err)
		}
		return articleStore, deliveryStore, retryQueue, func() {
			_ = articleStore.Close(context.Background())
			_ = deliveryStore.Close(context.Background())
			_ = retryQueue.Close(context.Background())
		}
	case "sqlite":
		sqliteDB, err := store.NewSQLiteDB(cfg.Store.SQLiteDSN)
		if err != nil {
			log.Fatalf("create sqlite store: %v", err)
		}
		return sqliteDB.ArticleStore(), sqliteDB.DeliveryStore(), sqliteDB.RetryQueue(), func() { _ = sqliteDB.Close(context.Background()) }
	default:
		log.Fatalf("unsupported store backend: %s", cfg.Store.Backend)
		return nil, nil, nil, func() {}
	}
}

func mustBuildDistributor(cfg config.Config, deliveryStore domain.DeliveryStore, retryQueue domain.RetryQueue) (domain.Distributor, map[string]domain.Distributor, func()) {
	retryChannels := map[string]domain.Distributor{"stdout": distribute.NewStdoutDistributor()}
	downstream := []domain.Distributor{distribute.NewTrackedDistributor("stdout", "local-log", retryChannels["stdout"], deliveryStore, retryQueue, cfg.Retry.MaxAttempts, cfg.Retry.Backoff)}
	
	// 调试日志
	log.Printf("DEBUG: DingTalk config - Enabled: %v, Webhook: %s, SecurityType: %s", cfg.Distribute.DingTalk.Enabled, cfg.Distribute.DingTalk.Webhook, cfg.Distribute.DingTalk.SecurityType)
	
	if cfg.Distribute.Blog.Enabled {
		blog, err := distribute.NewBlogDistributor(cfg.Distribute.Blog)
		if err != nil {
			log.Fatalf("create blog distributor: %v", err)
		}
		retryChannels["blog"] = blog
		downstream = append(downstream, distribute.NewTrackedDistributor("blog", cfg.Distribute.Blog.Endpoint, blog, deliveryStore, retryQueue, cfg.Retry.MaxAttempts, cfg.Retry.Backoff))
	}
	if cfg.Distribute.WeCom.Enabled {
		wecom, err := distribute.NewWeComDistributor(cfg.Distribute.WeCom)
		if err != nil {
			log.Printf("WARNING: create wecom distributor failed: %v", err)
			log.Printf("To fix this, either:")
			log.Printf("1. Set HOTNEW_WECOM_ENABLED=false in hotnew.env to disable WeCom推送")
			log.Printf("2. Set HOTNEW_WECOM_WEBHOOK to a valid WeCom robot webhook URL")
			log.Printf("   Format: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY")
			log.Printf("Continuing without WeCom推送...")
		} else {
			retryChannels["wecom"] = wecom
			downstream = append(downstream, distribute.NewTrackedDistributor("wecom", cfg.Distribute.WeCom.Webhook, wecom, deliveryStore, retryQueue, cfg.Retry.MaxAttempts, cfg.Retry.Backoff))
		}
	}
	if cfg.Distribute.DingTalk.Enabled {
		log.Printf("DEBUG: Initializing DingTalk distributor...")
		dingtalk, err := distribute.NewDingTalkDistributor(cfg.Distribute.DingTalk)
		if err != nil {
			log.Printf("WARNING: create dingtalk distributor failed: %v", err)
			log.Printf("To fix this, either:")
			log.Printf("1. Set HOTNEW_DINGTALK_ENABLED=false in hotnew.env to disable 钉钉推送")
			log.Printf("2. Set HOTNEW_DINGTALK_WEBHOOK to a valid 钉钉 robot webhook URL")
			log.Printf("   Format: https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN")
			log.Printf("Continuing without 钉钉推送...")
		} else {
			log.Printf("DEBUG: DingTalk distributor initialized successfully")
			retryChannels["dingtalk"] = dingtalk
			downstream = append(downstream, distribute.NewTrackedDistributor("dingtalk", cfg.Distribute.DingTalk.Webhook, dingtalk, deliveryStore, retryQueue, cfg.Retry.MaxAttempts, cfg.Retry.Backoff))
		}
	} else {
		log.Printf("DEBUG: DingTalk is disabled")
	}
	
	log.Printf("DEBUG: Downstream distributors: %v", len(downstream))
	for i, dist := range downstream {
		log.Printf("DEBUG: Distributor %d: %T", i, dist)
	}
	
	async := distribute.NewAsyncDistributor(cfg.Distribute.AsyncBuffer, distribute.NewMultiDistributor(downstream...))
	return async, retryChannels, func() { _ = async.Close(context.Background()) }
}
