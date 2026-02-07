package main

import (
	"fmt"
	"kuberan/internal/config"
	"kuberan/internal/database"
	"kuberan/internal/handlers"
	"kuberan/internal/logger"
	"kuberan/internal/middleware"
	"kuberan/internal/services"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "kuberan/internal/docs" // Import swagger docs
)

// @title           Kuberan API
// @version         1.0
// @description     Kuberan is a personal finance application that allows users to efficiently manage their finances, make budgets, and track investments.
// @termsOfService  http://swagger.io/terms/

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Initialize logger (use ENV var if available, default to development)
	logger.Init(os.Getenv("ENV"))
	defer logger.Sync()

	if err := run(); err != nil {
		logger.Get().Fatalf("Fatal error: %v", err)
	}
}

func run() error {
	log := logger.Get()

	// Load configuration
	appConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize database configuration
	dbConfig, err := database.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to load database configuration: %w", err)
	}

	// Create database manager
	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create database manager: %w", err)
	}

	// Run migrations
	if err := dbManager.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize services
	db := dbManager.DB()
	userService := services.NewUserService(db)
	accountService := services.NewAccountService(db)
	categoryService := services.NewCategoryService(db)
	transactionService := services.NewTransactionService(db, accountService)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService)
	accountHandler := handlers.NewAccountHandler(accountService)
	categoryHandler := handlers.NewCategoryHandler(categoryService)
	transactionHandler := handlers.NewTransactionHandler(transactionService)

	// Initialize Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogging())
	router.Use(middleware.ErrorHandler())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check endpoint
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API v1 group
	v1 := router.Group("/api/v1")

	// Public routes
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	// Protected routes
	protected := v1.Group("/")
	protected.Use(middleware.AuthMiddleware())

	// User profile
	protected.GET("/profile", authHandler.GetProfile)

	// Account routes
	accounts := protected.Group("/accounts")
	accounts.POST("/cash", accountHandler.CreateCashAccount)
	accounts.GET("", accountHandler.GetUserAccounts)
	accounts.GET("/:id", accountHandler.GetAccountByID)
	accounts.PUT("/:id", accountHandler.UpdateCashAccount)
	accounts.GET("/:id/transactions", transactionHandler.GetAccountTransactions)

	// Transaction routes
	transactions := protected.Group("/transactions")
	transactions.POST("", transactionHandler.CreateTransaction)
	transactions.GET("/:id", transactionHandler.GetTransactionByID)
	transactions.DELETE("/:id", transactionHandler.DeleteTransaction)

	// Category routes
	categories := protected.Group("/categories")
	categories.POST("", categoryHandler.CreateCategory)
	categories.GET("", categoryHandler.GetUserCategories)
	categories.GET("/:id", categoryHandler.GetCategoryByID)
	categories.PUT("/:id", categoryHandler.UpdateCategory)
	categories.DELETE("/:id", categoryHandler.DeleteCategory)

	log.Infof("Starting Kuberan backend server on port %s", appConfig.Port)
	log.Infof("Swagger documentation available at http://localhost:%s/swagger/index.html", appConfig.Port)
	return router.Run(":" + appConfig.Port)
}
