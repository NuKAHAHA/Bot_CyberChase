package company

import (
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/pkg"
	"Cyber-chase/internal/repository"
	"Cyber-chase/internal/service"
	"crypto/rand"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type CompanyHandler struct {
	repo        *repository.Repository
	mailer      MailService
	jwtSecret   string
	teamService service.TeamService
}

func NewCompanyHandler(repo *repository.Repository, mailer MailService, teamService service.TeamService) *CompanyHandler {
	return &CompanyHandler{
		repo:        repo,
		mailer:      mailer,
		jwtSecret:   os.Getenv("JWT_SECRET"),
		teamService: teamService,
	}
}

type CompanyTaskHandler struct {
	repo *repository.Repository
}

func NewCompanyTaskHandler(repo *repository.Repository) *CompanyTaskHandler {
	return &CompanyTaskHandler{repo: repo}
}

func generateTempPassword(length int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[num.Int64()]
	}

	return string(result), nil
}

func (h *CompanyHandler) CreateCompany(c *gin.Context) {
	var input struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Location string `json:"location"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tempPass, err := generateTempPassword(12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate password"})
		return
	}

	company := &models.Company{
		Name:          input.Name,
		Email:         input.Email,
		TempPassword:  tempPass,
		ResetRequired: true,
		Location:      input.Location,
	}

	if err := h.repo.CreateCompany(c.Request.Context(), company); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Company already exists"})
		return
	}

	if err := h.mailer.SendTempPassword(company.Email, tempPass); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send password"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":     company.ID,
		"status": "created",
	})
}

func (h *CompanyHandler) GetAllCompanies(c *gin.Context) {
	companies, err := h.repo.GetAllCompanies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]gin.H, 0)
	for _, comp := range companies {
		response = append(response, gin.H{
			"id":           comp.ID,
			"name":         comp.Name,
			"email":        comp.Email,
			"reset_needed": comp.ResetRequired,
			"location":     comp.Location,
		})
	}

	c.JSON(http.StatusOK, response)
}

func (h *CompanyHandler) UpdateCompany(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Location string `json:"location"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	company, err := h.repo.GetCompanyByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	if input.Name != "" {
		company.Name = input.Name
	}
	if input.Email != "" {
		company.Email = input.Email
	}
	if input.Location != "" {
		company.Location = input.Location
	}

	if err := h.repo.UpdateCompany(c.Request.Context(), company); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *CompanyHandler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	company, err := h.repo.GetCompanyByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	newPass, err := generateTempPassword(12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate password"})
		return
	}

	company.TempPassword = newPass
	company.ResetRequired = true

	if err := h.mailer.SendTempPassword(company.Email, newPass); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send password"})
		return
	}

	if err := h.repo.UpdateCompany(c.Request.Context(), company); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "password_reset",
		"message": "New password sent to company email",
	})
}

func (h *CompanyHandler) DeleteCompany(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := h.repo.DeleteCompany(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *CompanyHandler) CompanyLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	company, err := h.repo.GetCompanyByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(company.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            company.ID.String(),
		"role":           "company",
		"exp":            time.Now().Add(time.Hour * 24).Unix(),
		"reset_required": company.ResetRequired,
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":          tokenString,
		"reset_required": company.ResetRequired,
	})
}

func (h *CompanyHandler) GetMapLink(c *gin.Context) {
	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	company, err := h.repo.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	if company.Location == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Location not set for this company"})
		return
	}

	mapLink := fmt.Sprintf(company.Location)

	c.JSON(http.StatusOK, gin.H{
		"map_link": mapLink,
	})
}

func (h *CompanyHandler) ChangePassword(c *gin.Context) {
	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	company, err := h.repo.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(company.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid old password"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process new password"})
		return
	}

	company.PasswordHash = string(newHash)
	company.ResetRequired = false

	if err := h.repo.UpdateCompany(c.Request.Context(), company); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            company.ID.String(),
		"role":           "company",
		"exp":            time.Now().Add(time.Hour * 24).Unix(),
		"reset_required": false,
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "password_changed",
		"token":  tokenString,
	})
}

func (h *CompanyTaskHandler) CreateTask(c *gin.Context) {
	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	// Parse form
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
		return
	}

	contestIDStr := c.PostForm("contest_id")
	if contestIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contest ID is required"})
		return
	}

	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contest ID"})
		return
	}

	_, err = h.repo.GetContestByID(c.Request.Context(), contestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contest not found"})
		return
	}

	question := c.PostForm("question")
	correctAnswer := c.PostForm("correct_answer")
	timeLimitStr := c.PostForm("time_limit")

	if question == "" && c.Request.MultipartForm.File["question_file"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either question text or question file is required"})
		return
	}

	if correctAnswer == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Correct answer is required"})
		return
	}

	// Create a new task
	task := &models.Task{
		ID:            uuid.New(),
		Question:      question,
		CorrectAnswer: correctAnswer,
		ContestID:     contestID,
		CompanyID:     companyID,
	}

	// Set time limit if provided
	if timeLimitStr != "" {
		var timeLimit int
		fmt.Sscanf(timeLimitStr, "%d", &timeLimit)
		task.TimeLimit = timeLimit
	}

	// Handle file upload if present
	if files := c.Request.MultipartForm.File["question_file"]; len(files) > 0 {
		file := files[0]
		filePath, err := pkg.SaveFile(file, task.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
			return
		}
		task.QuestionFile = filepath.Base(filePath)
	}

	if err := h.repo.CreateTask(c.Request.Context(), task); err != nil {
		// Cleanup uploaded file if task creation fails
		if task.QuestionFile != "" {
			pkg.DeleteTaskFiles(task.ID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":     task.ID,
		"status": "created",
	})
}

func (h *CompanyTaskHandler) GetCompanyTasks(c *gin.Context) {
	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	tasks, err := h.repo.GetTasksByCompanyID(c.Request.Context(), companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// Update UpdateTask to handle file uploads
func (h *CompanyTaskHandler) UpdateTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	task, err := h.repo.GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	if task.CompanyID != companyID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to update this task"})
		return
	}

	// Parse form for file upload
	if c.ContentType() == "multipart/form-data" {
		if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		question := c.PostForm("question")
		correctAnswer := c.PostForm("correct_answer")
		timeLimitStr := c.PostForm("time_limit")

		if question != "" {
			task.Question = question
		}
		if correctAnswer != "" {
			task.CorrectAnswer = correctAnswer
		}
		if timeLimitStr != "" {
			var timeLimit int
			fmt.Sscanf(timeLimitStr, "%d", &timeLimit)
			task.TimeLimit = timeLimit
		}

		// Handle file update if present
		if files := c.Request.MultipartForm.File["question_file"]; len(files) > 0 {
			// Delete existing file if there is one
			if task.QuestionFile != "" {
				pkg.DeleteTaskFiles(taskID)
			}

			// Save new file
			file := files[0]
			filePath, err := pkg.SaveFile(file, task.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
				return
			}
			task.QuestionFile = filepath.Base(filePath)
		}
	} else {
		var input struct {
			Question      string `json:"question"`
			CorrectAnswer string `json:"correct_answer"`
			TimeLimit     *int   `json:"time_limit"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Question != "" {
			task.Question = input.Question
		}
		if input.CorrectAnswer != "" {
			task.CorrectAnswer = input.CorrectAnswer
		}
		if input.TimeLimit != nil {
			task.TimeLimit = *input.TimeLimit
		}
	}

	if err := h.repo.UpdateTask(c.Request.Context(), task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "task": task})
}

func (h *CompanyTaskHandler) DeleteTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.repo.GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil || task.CompanyID != companyID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete this task"})
		return
	}

	// Delete task files if they exist
	if task.QuestionFile != "" {
		if err := pkg.DeleteTaskFiles(taskID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task files"})
			return
		}
	}

	if err := h.repo.DeleteTask(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *CompanyTaskHandler) GetTaskFile(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.repo.GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	if task.QuestionFile == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No file associated with this task"})
		return
	}

	filePath := pkg.GetFilePath(taskID, task.QuestionFile)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Serve the file
	c.FileAttachment(filePath, task.QuestionFile)
}

func (h *CompanyHandler) GetUnassignedTeams(c *gin.Context) {
	teams, err := h.teamService.GetUnassignedTeams()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, teams)
}

func (h *CompanyHandler) ApproveTeam(c *gin.Context) {
	companyID, err := uuid.Parse(c.GetString("companyID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid company ID"})
		return
	}

	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	if err := h.teamService.ApproveTeam(teamID, companyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "team approved"})
}
