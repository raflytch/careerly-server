package main

import (
	"log"
	"os"

	"github.com/raflytch/careerly-server/internal/config"
	"github.com/raflytch/careerly-server/internal/database"
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/repository"
	"github.com/raflytch/careerly-server/internal/routes"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/imagekit"
	"github.com/raflytch/careerly-server/pkg/jwt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()

	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	redisClient, err := database.NewRedisConnection(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	jwtManager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.ExpiryHours)

	imagekitClient := imagekit.NewClient(imagekit.Config{
		PublicKey:   cfg.ImageKit.PublicKey,
		PrivateKey:  cfg.ImageKit.PrivateKey,
		URLEndpoint: cfg.ImageKit.URLEndpoint,
	})

	userRepo := repository.NewUserRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)

	authService := service.NewAuthService(userRepo, cacheRepo, cfg.Google, jwtManager)
	userService := service.NewUserService(userRepo, cacheRepo)

	authMiddleware := middleware.NewAuthMiddleware(authService)

	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService, imagekitClient)

	app := fiber.New(fiber.Config{
		AppName:      "Careerly API",
		ErrorHandler: customErrorHandler,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, PATCH, OPTIONS",
		AllowCredentials: false,
	}))

	routes.Setup(app, routes.Handlers{
		Auth: authHandler,
		User: userHandler,
	}, routes.Middlewares{
		Auth: authMiddleware,
	})

	port := cfg.App.Port
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
		os.Exit(1)
	}
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error":   err.Error(),
	})
}
