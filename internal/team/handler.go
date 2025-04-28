package team

/*
import (
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

type TeamHandler struct {
	repo      *repository.Repository
	jwtSecret string
	bot       *tele.Bot
}

func NewTeamHandler(repo *repository.Repository, bot *tele.Bot) *TeamHandler {
	return &TeamHandler{
		repo:      repo,
		jwtSecret: os.Getenv("JWT_SECRET"),
		bot:       bot,
	}
}

type RegisterRequest struct {
	Name      string `json:"name" binding:"required"`
	ContactID string `json:"contact_id" binding:"required"`
	ContestID string `json:"contest_id" binding:"required"`
}

func (h *TeamHandler) RegisterTeam(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contestID, err := uuid.Parse(req.ContestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contest id"})
		return
	}

	team := &models.Team{
		ID:           uuid.New(),
		Name:         req.Name,
		ContactInfo:  req.ContactID,
		ContestID:    contestID,
		AttemptsLeft: 3,
	}

	if err := h.repo.CreateTeam(c.Request.Context(), team); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "team already exists"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  team.ID.String(),
		"role": "team",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": tokenString,
		"team":  team,
	})
}

func (h *TeamHandler) LoginTeam(c *gin.Context) {
	var req struct {
		ContactID string `json:"contact_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.repo.GetTeamByContactInfo(c.Request.Context(), req.ContactID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  team.ID.String(),
		"role": "team",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"team":  team,
	})
}

func (h *TeamHandler) GetCurrentTask(c *gin.Context) {
	teamID, err := uuid.Parse(c.GetString("teamID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	team, err := h.repo.GetTeamByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	if team.CurrentTaskID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active task"})
		return
	}

	task, err := h.repo.GetTaskByID(c.Request.Context(), *team.CurrentTaskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task":       task,
		"attempts":   team.AttemptsLeft,
		"start_time": team.CurrentTaskStartTime,
	})
}

func (h *TeamHandler) SubmitAnswer(c *gin.Context) {
	teamID, err := uuid.Parse(c.GetString("teamID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req struct {
		Answer string `json:"answer" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.repo.GetTeamByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	if team.CurrentTaskID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active task"})
		return
	}

	task, err := h.repo.GetTaskByID(c.Request.Context(), *team.CurrentTaskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	timeSpent := int(time.Since(*team.CurrentTaskStartTime).Seconds())
	if timeSpent > task.TimeLimit {
		timeSpent = task.TimeLimit
	}

	// Record attempt
	teamTask := &models.TeamTask{
		ID:        uuid.New(),
		TeamID:    teamID,
		TaskID:    task.ID,
		Attempts:  team.AttemptsLeft,
		TimeSpent: timeSpent,
	}

	if req.Answer == task.CorrectAnswer {
		team.Score++
		team.TotalTime += timeSpent
		teamTask.Status = "completed"
	} else {
		team.AttemptsLeft--
		teamTask.Status = "failed"
	}

	if team.AttemptsLeft <= 0 {
		team.CurrentTaskID = nil
		team.CurrentCompanyID = nil
		team.CurrentTaskStartTime = nil
	}

	err = h.repo.WithTransaction(c.Request.Context(), func(ctx context.Context) error {
		if err := h.repo.UpdateTeam(ctx, team); err != nil {
			return err
		}
		return h.repo.CreateTeamTask(ctx, teamTask)
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process answer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"correct":    req.Answer == task.CorrectAnswer,
		"attempts":   team.AttemptsLeft,
		"score":      team.Score,
		"time_spent": timeSpent,
	})
}

func (h *TeamHandler) GetNextLocation(c *gin.Context) {
	teamID, err := uuid.Parse(c.GetString("teamID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	team, err := h.repo.GetTeamByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	if team.CurrentTaskID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task already in progress"})
		return
	}

	company, err := h.repo.GetNextAvailableCompany(c.Request.Context(), team.ContestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no available locations"})
		return
	}

	task, err := h.repo.GetTaskByCompanyID(c.Request.Context(), company.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no tasks available"})
		return
	}

	now := time.Now()
	team.CurrentTaskID = &task.ID
	team.CurrentCompanyID = &company.ID
	team.CurrentTaskStartTime = &now
	team.AttemptsLeft = 3

	if err := h.repo.UpdateTeam(c.Request.Context(), team); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"location":   company.Location,
		"task_id":    task.ID,
		"start_time": now,
	})
}

func (h *TeamHandler) GetLeaderboard(c *gin.Context) {
	teams, err := h.repo.GetAllTeams(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get leaderboard"})
		return
	}

	type LeaderboardEntry struct {
		Rank     int    `json:"rank"`
		TeamName string `json:"team_name"`
		Score    int    `json:"score"`
		Time     int    `json:"time"`
	}

	entries := make([]LeaderboardEntry, 0, len(teams))
	for i, team := range teams {
		entries = append(entries, LeaderboardEntry{
			Rank:     i + 1,
			TeamName: team.Name,
			Score:    team.Score,
			Time:     team.TotalTime,
		})
	}

	c.JSON(http.StatusOK, entries)
}

// Add these functions to your TeamHandler struct in team/handler.go

func (h *TeamHandler) handleRegister(c tele.Context) error {
	text := c.Message().Text
	args := strings.Split(text, " ")
	if len(args) < 2 {
		return c.Send("Для регистрации введите: /register <название команды>")
	}

	teamName := strings.Join(args[1:], " ")
	contactID := fmt.Sprint(c.Sender().ID)

	// Check if team already exists
	_, err := h.repo.GetTeamByContactInfo(context.Background(), contactID)
	if err == nil {
		return c.Send("Команда уже зарегистрирована. Используйте /login для входа.")
	}

	// Get the active contest
	contests, err := h.repo.GetAllContests(context.Background())
	if err != nil || len(contests) == 0 {
		return c.Send("Нет активных соревнований")
	}

	// Use the first active contest (you might want to refine this logic)
	var activeContest *models.Contest
	for _, contest := range contests {
		if contest.Status == "active" {
			activeContest = &contest
			break
		}
	}

	if activeContest == nil {
		return c.Send("Нет активных соревнований")
	}

	// Create new team
	team := &models.Team{
		ID:           uuid.New(),
		Name:         teamName,
		ContactInfo:  contactID,
		ContestID:    activeContest.ID,
		AttemptsLeft: 3,
	}

	if err := h.repo.CreateTeam(context.Background(), team); err != nil {
		return c.Send("Ошибка при регистрации команды")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  team.ID.String(),
		"role": "team",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return c.Send("Ошибка авторизации")
	}

	return c.Send(fmt.Sprintf("Команда %s успешно зарегистрирована! Ваш токен: %s", teamName, tokenString))
}

func (h *TeamHandler) handleLogin(c tele.Context) error {
	contactID := fmt.Sprint(c.Sender().ID)

	team, err := h.repo.GetTeamByContactInfo(context.Background(), contactID)
	if err != nil {
		return c.Send("Команда не найдена. Используйте /register <название команды> для регистрации.")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  team.ID.String(),
		"role": "team",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return c.Send("Ошибка авторизации")
	}

	return c.Send(fmt.Sprintf("Вы успешно авторизованы как команда %s! Ваш токен: %s", team.Name, tokenString))
}

func (h *TeamHandler) handleNextLocation(c tele.Context) error {
	teamID, err := h.getTeamID(c)
	if err != nil {
		return c.Send("Требуется авторизация. Используйте токен, полученный при регистрации.")
	}

	team, err := h.repo.GetTeamByID(context.Background(), teamID)
	if err != nil {
		return c.Send("Команда не найдена")
	}

	if team.CurrentTaskID != nil {
		return c.Send("У вас уже есть активное задание. Завершите его перед получением нового.")
	}

	company, err := h.repo.GetNextAvailableCompany(context.Background(), team.ContestID)
	if err != nil {
		return c.Send("Нет доступных локаций. Возможно, вы завершили все задания.")
	}

	task, err := h.repo.GetTaskByCompanyID(context.Background(), company.ID)
	if err != nil {
		return c.Send("Нет доступных заданий для этой локации.")
	}

	now := time.Now()
	team.CurrentTaskID = &task.ID
	team.CurrentCompanyID = &company.ID
	team.CurrentTaskStartTime = &now
	team.AttemptsLeft = 3

	if err := h.repo.UpdateTeam(context.Background(), team); err != nil {
		return c.Send("Ошибка при получении нового задания")
	}

	return c.Send(fmt.Sprintf("Следующая локация: %s\nВремя начала: %s\nИспользуйте /task для получения задания",
		company.Location,
		now.Format("15:04:05")))
}

func (h *TeamHandler) StartBot() {
	h.bot.Handle("/start", h.handleStart)             // Старт/регистрация
	h.bot.Handle("/register", h.handleRegister)       // Регистрация команды
	h.bot.Handle("/login", h.handleLogin)             // Авторизация
	h.bot.Handle("/task", h.handleCurrentTask)        // Текущее задание
	h.bot.Handle("/answer", h.handleAnswer)           // Отправить ответ
	h.bot.Handle("/next", h.handleNextLocation)       // Следующая локация
	h.bot.Handle("/leaderboard", h.handleLeaderboard) // Лидерборд

	go h.bot.Start()
}

func (h *TeamHandler) handleStart(c tele.Context) error {
	contactID := fmt.Sprint(c.Sender().ID)

	team, err := h.repo.GetTeamByContactInfo(context.Background(), contactID)
	if errors.Is(err, repository.ErrNotFound) {
		return c.Send("Для регистрации введите /register <название команды>")
	}
	if err != nil {
		return c.Send("Ошибка сервера")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  team.ID.String(),
		"role": "team",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return c.Send("Ошибка авторизации")
	}

	return c.Send(fmt.Sprintf("Добро пожаловать, %s! Ваш токен: %s", team.Name, tokenString))
}

func (h *TeamHandler) handleCurrentTask(c tele.Context) error {
	teamID, err := h.getTeamID(c)
	if err != nil {
		return c.Send("Требуется авторизация")
	}

	team, err := h.repo.GetTeamByID(context.Background(), teamID)
	if err != nil {
		return c.Send("Команда не найдена")
	}

	if team.CurrentTaskID == nil {
		return c.Send("Нет активных заданий. Используйте /next для получения нового задания")
	}

	task, err := h.repo.GetTaskByID(context.Background(), *team.CurrentTaskID)
	if err != nil {
		return c.Send("Задание не найдено")
	}

	msg := fmt.Sprintf("Текущее задание:\n%s\nОсталось попыток: %d", task.Question, team.AttemptsLeft)
	return c.Send(msg)
}

func (h *TeamHandler) handleAnswer(c tele.Context) error {
	teamID, err := h.getTeamID(c)
	if err != nil {
		return c.Send("Требуется авторизация. Пожалуйста, добавьте токен после команды: /answer <ответ> <токен>")
	}

	text := c.Message().Text
	args := strings.Split(text, " ")
	if len(args) < 2 {
		return c.Send("Пожалуйста, введите ответ: /answer <ответ>")
	}

	// Extract the answer (everything after /answer)
	answer := strings.Join(args[1:len(args)-1], " ")
	if answer == "" {
		return c.Send("Ответ не может быть пустым")
	}

	team, err := h.repo.GetTeamByID(context.Background(), teamID)
	if err != nil {
		return c.Send("Команда не найдена")
	}

	if team.CurrentTaskID == nil {
		return c.Send("Нет активных заданий. Используйте /next для получения нового задания")
	}

	task, err := h.repo.GetTaskByID(context.Background(), *team.CurrentTaskID)
	if err != nil {
		return c.Send("Задание не найдено")
	}

	timeSpent := int(time.Since(*team.CurrentTaskStartTime).Seconds())
	if timeSpent > task.TimeLimit {
		timeSpent = task.TimeLimit
	}

	// Record attempt
	teamTask := &models.TeamTask{
		ID:        uuid.New(),
		TeamID:    teamID,
		TaskID:    task.ID,
		Attempts:  team.AttemptsLeft,
		TimeSpent: timeSpent,
	}

	if answer == task.CorrectAnswer {
		team.Score++
		team.TotalTime += timeSpent
		teamTask.Status = "completed"

		// Clear current task since it's completed
		team.CurrentTaskID = nil
		team.CurrentCompanyID = nil
		team.CurrentTaskStartTime = nil

		err = h.repo.WithTransaction(context.Background(), func(ctx context.Context) error {
			if err := h.repo.UpdateTeam(ctx, team); err != nil {
				return err
			}
			return h.repo.CreateTeamTask(ctx, teamTask)
		})

		if err != nil {
			return c.Send("Ошибка при обработке ответа")
		}

		return c.Send(fmt.Sprintf("Правильный ответ! Ваш счет: %d, Затраченное время: %d сек.", team.Score, timeSpent))
	} else {
		team.AttemptsLeft--
		teamTask.Status = "failed"

		if team.AttemptsLeft <= 0 {
			team.CurrentTaskID = nil
			team.CurrentCompanyID = nil
			team.CurrentTaskStartTime = nil

			err = h.repo.WithTransaction(context.Background(), func(ctx context.Context) error {
				if err := h.repo.UpdateTeam(ctx, team); err != nil {
					return err
				}
				return h.repo.CreateTeamTask(ctx, teamTask)
			})

			if err != nil {
				return c.Send("Ошибка при обработке ответа")
			}

			return c.Send("Неверный ответ. У вас не осталось попыток. Используйте /next для получения нового задания.")
		}

		err = h.repo.WithTransaction(context.Background(), func(ctx context.Context) error {
			if err := h.repo.UpdateTeam(ctx, team); err != nil {
				return err
			}
			return h.repo.CreateTeamTask(ctx, teamTask)
		})

		if err != nil {
			return c.Send("Ошибка при обработке ответа")
		}

		return c.Send(fmt.Sprintf("Неверный ответ. Осталось попыток: %d", team.AttemptsLeft))
	}
}

func (h *TeamHandler) getTeamID(c tele.Context) (uuid.UUID, error) {
	token := c.Message().Payload
	if token == "" {
		return uuid.Nil, errors.New("no token")
	}

	claims, err := h.verifyToken(token)
	if err != nil {
		return uuid.Nil, err
	}

	return uuid.Parse(claims["sub"].(string))
}

func (h *TeamHandler) verifyToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] != "team" {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}

func (h *TeamHandler) handleLeaderboard(c tele.Context) error {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/team/leaderboard", os.Getenv("API_URL")))
	if err != nil {
		return c.Send("Ошибка при получении лидерборда")
	}

	defer resp.Body.Close()

	var entries []struct {
		Rank     int    `json:"rank"`
		TeamName string `json:"team_name"`
		Score    int    `json:"score"`
		Time     int    `json:"time"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return c.Send("Ошибка обработки данных")
	}

	var msg strings.Builder
	msg.WriteString("🏆 Лидерборд:\n")
	for _, entry := range entries {
		msg.WriteString(fmt.Sprintf("%d. %s - %d баллов (Время: %d сек)\n",
			entry.Rank, entry.TeamName, entry.Score, entry.Time))
	}
	return c.Send(msg.String())
}


*/
