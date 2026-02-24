package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"cgoforum/config"
	"cgoforum/internal/handler"
	"cgoforum/internal/handler/middleware"
	"cgoforum/internal/job"
	"cgoforum/internal/mq/consumer"
	"cgoforum/internal/mq/publisher"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
	"cgoforum/internal/service"
	"cgoforum/ioc"
	jwtpkg "cgoforum/pkg/jwt"
	"cgoforum/pkg/vectorizer"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Init logger
	var logger *zap.Logger
	if cfg.Server.Mode == "release" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Init infrastructure
	db := ioc.InitDB(&cfg.Database, logger)
	rdb := ioc.InitRedis(&cfg.Redis, logger)
	rmqCh := ioc.InitRabbitMQ(&cfg.RabbitMQ, logger)
	defer rmqCh.Close()

	searchClient := ioc.InitMeilisearch(&cfg.Meilisearch, logger)
	defer searchClient.Close()

	jwtHandler, err := jwtpkg.NewHandler(
		cfg.JWT.PrivateKeyPath,
		cfg.JWT.PublicKeyPath,
		cfg.JWT.AccessDuration(),
		cfg.JWT.RefreshDuration(),
		cfg.JWT.Issuer,
	)
	if err != nil {
		logger.Fatal("failed to init jwt handler", zap.Error(err))
	}

	// Init repositories
	userDAO := dao.NewUserDAO(db)
	articleDAO := dao.NewArticleDAO(db)
	followDAO := dao.NewFollowDAO(db)
	likeDAO := dao.NewLikeDAO(db)
	collectDAO := dao.NewCollectDAO(db)
	statDAO := dao.NewStatDAO(db)
	eventLogDAO := dao.NewEventLogDAO(db)

	authCache := cache.NewAuthCache(rdb)
	feedCache := cache.NewFeedCache(rdb)
	interactionCache := cache.NewInteractionCache(rdb)
	rankCache := cache.NewRankCache(rdb)

	eventPub := publisher.NewEventPublisher(rmqCh, logger)
	embedder := vectorizer.NewSentenceTransformerClient(&cfg.Vectorizer)

	// Init services
	authSvc := service.NewAuthService(userDAO, authCache, jwtHandler, logger)
	articleSvc := service.NewArticleService(articleDAO, feedCache, eventPub, logger)
	interactionSvc := service.NewInteractionService(
		followDAO,
		likeDAO,
		collectDAO,
		articleDAO,
		statDAO,
		interactionCache,
		feedCache,
		eventPub,
		logger,
	)
	rankSvc := service.NewRankService(rankCache, feedCache, articleDAO, statDAO, logger)
	searchSvc := service.NewSearchService(
		articleDAO,
		statDAO,
		feedCache,
		interactionCache,
		searchClient,
		cfg.Meilisearch.Index,
		embedder,
		logger,
	)

	// Init handlers
	authHandler := handler.NewAuthHandler(authSvc, jwtHandler, logger)
	articleHandler := handler.NewArticleHandler(articleSvc, logger)
	interactionHandler := handler.NewInteractionHandler(interactionSvc, logger)
	rankHandler := handler.NewRankHandler(rankSvc, logger)
	searchHandler := handler.NewSearchHandler(searchSvc, logger)

	// Set gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create gin engine
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://127.0.0.1:5173",
			"http://localhost:5173",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authMW := middleware.AuthRequired(jwtHandler)
	adminMW := middleware.IsAdmin()

	api := engine.Group("/api")
	authHandler.RegisterRoutes(api, authMW, adminMW)
	articleHandler.RegisterRoutes(api, authMW)
	interactionHandler.RegisterRoutes(api, authMW)
	rankHandler.RegisterRoutes(api)
	searchHandler.RegisterRoutes(api)

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Start MQ consumers
	consumerCtx, cancelConsumers := context.WithCancel(context.Background())
	defer cancelConsumers()

	searchIndexer := consumer.NewSearchIndexer(rmqCh, searchClient, cfg.Meilisearch.Index, logger)
	if err := searchIndexer.Start(consumerCtx); err != nil {
		logger.Warn("failed to start search indexer", zap.Error(err))
	}

	statSyncer := consumer.NewStatSyncer(rmqCh, statDAO, eventLogDAO, logger)
	if err := statSyncer.Start(consumerCtx); err != nil {
		logger.Warn("failed to start stat syncer", zap.Error(err))
	}

	likePersist := consumer.NewLikePersistConsumer(rmqCh, likeDAO, eventLogDAO, logger)
	if err := likePersist.Start(consumerCtx); err != nil {
		logger.Warn("failed to start like persist consumer", zap.Error(err))
	}

	activityTracker := consumer.NewActivityTracker(rmqCh, rdb, eventLogDAO, logger)
	if err := activityTracker.Start(consumerCtx); err != nil {
		logger.Warn("failed to start activity tracker", zap.Error(err))
	}

	embeddingStub := consumer.NewEmbeddingStub(rmqCh, articleDAO, embedder, logger)
	if err := embeddingStub.Start(consumerCtx); err != nil {
		logger.Warn("failed to start embedding consumer", zap.Error(err))
	}

	hotRankJob := job.NewHotRankJob(rankSvc, logger)
	hotRankJob.Start(consumerCtx)

	activityCleanupJob := job.NewActivityWindowCleanupJob(rdb, logger)
	activityCleanupJob.Start(consumerCtx)

	// Create HTTP server
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server starting", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server listen failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
