package main

import (
	"log"
	"os"
	"time"

	"security-service/internal/audit"
	"security-service/internal/auth"
	"security-service/internal/middleware"
	"security-service/internal/rbac"
	"security-service/internal/security"
	"security-service/internal/store"
	"security-service/internal/user"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize database
	db, err := store.InitDB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Initialize Redis
	rdb, err := store.InitRedis()
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	// Auto-migrate
	if err := db.AutoMigrate(
		&user.User{},
		&rbac.Role{},
		&rbac.Permission{},
		&rbac.UserRole{},
		&audit.Log{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// Seed RBAC data
	rbac.Seed(db)

	// Services
	jwtManager := security.NewJWTManager()
	blacklist := store.NewTokenBlacklist(rdb)
	userRepo := user.NewRepository(db)
	authService := auth.NewService(userRepo, jwtManager, blacklist)
	rbacService := rbac.NewService(db, rdb)
	auditService := audit.NewService(db)

	// Handlers
	authHandler := auth.NewHandler(authService)
	userHandler := user.NewHandler(userRepo)
	rbacHandler := rbac.NewHandler(rbacService)
	auditHandler := audit.NewHandler(auditService)

	r := gin.Default()

	// Global middleware
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Cors())
	r.Use(middleware.RequestID())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.AuditLog(auditService))

	// Public auth routes
	authGroup := api.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login",
		middleware.RateLimitByIP(rdb, 5, time.Minute),
		authHandler.Login)
	authGroup.POST("/refresh", authHandler.RefreshToken)
	authGroup.POST("/logout", authHandler.Logout)

	// Protected routes (JWT required)
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtManager, blacklist))
	protected.Use(middleware.RateLimitByUser(rdb, 30, time.Minute))

	// User routes
	userHandler.RegisterRoutes(protected)

	// RBAC admin routes (require role:manage)
	adminRBAC := protected.Group("")
	adminRBAC.Use(middleware.RequirePermission(rbacService, "role:manage"))
	rbacHandler.RegisterRoutes(adminRBAC)

	// Audit log routes (require log:read — admin + auditor)
	auditGroup := protected.Group("")
	auditGroup.Use(middleware.RequirePermission(rbacService, "log:read"))
	auditHandler.RegisterRoutes(auditGroup)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
