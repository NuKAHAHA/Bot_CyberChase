package main

import (
	"Cyber-chase/internal/admin"
	"Cyber-chase/internal/company"
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/pkg"
	"Cyber-chase/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
)

func main() {
	dsn := "host=localhost user=Nurdaulet password=123456 dbname=cyberchase port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database")
	}

	if err := godotenv.Load(); err != nil {
		panic("Error loading .env file")
	}

	db.AutoMigrate(&models.Contest{}, &models.Company{}, &models.Task{}, &models.Team{},
		&models.TeamTask{})

	repo := repository.NewRepository(db)
	adminHandler := admin.NewAdminHandler(repo, "admin", "0000")

	mailService := company.NewSMTPMailer(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USER"),
		os.Getenv("SMTP_PASS"),
	)

	companyHandler := company.NewCompanyHandler(repo, mailService)
	companyTaskHandler := company.NewCompanyTaskHandler(repo)

	jwtSecret := os.Getenv("JWT_SECRET")

	router := gin.Default()

	/*bot, err := tele.NewBot(tele.Settings{
		Token: os.Getenv("BOT_TOKEN"),
	})
	if err != nil {
		panic("failed to create bot")
	}

	teamHandler := team.NewTeamHandler(repo, bot)

	go teamHandler.StartBot()
	*/
	public := router.Group("/api/v1")
	{
		public.POST("/admin/login", adminHandler.AdminLogin)
		public.POST("/company/login", companyHandler.CompanyLogin)
		//public.POST("/team/register", teamHandler.RegisterTeam)
		//public.POST("/team/login", teamHandler.LoginTeam)
	}

	adminRoutes := router.Group("/api/v1/admin")
	adminRoutes.Use(pkg.JWTAuthMiddleware(jwtSecret))
	{
		adminRoutes.POST("/companies", companyHandler.CreateCompany)
		adminRoutes.GET("/companies", companyHandler.GetAllCompanies)
		adminRoutes.PUT("/companies/:id", companyHandler.UpdateCompany)
		adminRoutes.POST("/companies/:id/reset-password", companyHandler.ResetPassword)
		adminRoutes.DELETE("/companies/:id", companyHandler.DeleteCompany)

		adminRoutes.POST("/contests", adminHandler.CreateContest)
		adminRoutes.GET("/contests", adminHandler.GetAllContests)
		adminRoutes.PUT("/contests/:id", adminHandler.UpdateContest)
		adminRoutes.DELETE("/contests/:id", adminHandler.DeleteContest)

		adminRoutes.POST("/contests/:id/start", adminHandler.StartContest)
		adminRoutes.POST("/contests/:id/end", adminHandler.EndContest)
	}

	companyRoutes := router.Group("/api/v1/company")
	companyRoutes.Use(pkg.CompanyAuthMiddleware(jwtSecret))
	{
		companyRoutes.POST("/change-password", companyHandler.ChangePassword)

		companyRoutes.GET("/location", companyHandler.GetMapLink)

		companyRoutes.POST("/approve-team", companyHandler.ApproveTeam)

		companyRoutes.POST("/tasks", companyTaskHandler.CreateTask)
		companyRoutes.GET("/tasks", companyTaskHandler.GetCompanyTasks)
		companyRoutes.GET("/tasks/:id/file", companyTaskHandler.GetTaskFile)
		companyRoutes.PUT("/tasks/:id", companyTaskHandler.UpdateTask)
		companyRoutes.DELETE("/tasks/:id", companyTaskHandler.DeleteTask)
	}
	/*
		teamRoutes := router.Group("/api/v1/team")
		teamRoutes.Use(pkg.TeamAuthMiddleware(jwtSecret))
		{
			teamRoutes.GET("/task", teamHandler.GetCurrentTask)
			teamRoutes.POST("/task/submit", teamHandler.SubmitAnswer)
			teamRoutes.GET("/next-location", teamHandler.GetNextLocation)
			teamRoutes.GET("/leaderboard", teamHandler.GetLeaderboard)
		}
	*/
	router.Run(":8080")
}
