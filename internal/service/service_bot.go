package service

import (
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/repository"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"time"
)

// MailService интерфейс для отправки почты
type MailService interface {
	SendTempPassword(email, password string) error
}

// TeamService интерфейс сервиса для работы с командами
type TeamService interface {
	RegisterTeam(email, name string) error
	AuthenticateTeam(email, password string) (*models.Team, error)
	GetTeamByEmail(email string) (*models.Team, error)
	LinkTelegramToTeam(email string, telegramID int64) error
	JoinContest(teamID uuid.UUID) (*models.Contest, error)
	GetTask(teamID uuid.UUID) (*models.Task, error)
	SubmitAnswer(teamID uuid.UUID, taskID uuid.UUID, answer string) (bool, error)
	GetUnassignedTeams() ([]models.Team, error)
	ApproveTeam(teamID, companyID uuid.UUID) error
	GetTaskSession(teamID, taskID uuid.UUID) (*models.TeamTaskSession, error)
	GetTeamByID(teamID uuid.UUID) (*models.Team, error)
	GetCompanyCredentials(companyID uuid.UUID) (*models.Company, error)
	GetCompanyIDByTeam(teamID uuid.UUID) (uuid.UUID, error)
	GetCompanyByID(companyID uuid.UUID) (*models.Company, error)
}

// TeamServiceImpl имплементация TeamService
type TeamServiceImpl struct {
	repo       repository.TeamRepository
	coreRepo   *repository.Repository
	db         *gorm.DB
	mailClient MailService
}

// NewTeamService создает новый сервис для работы с командами
func NewTeamService(teamRepo repository.TeamRepository, coreRepo *repository.Repository, db *gorm.DB, mailClient MailService) *TeamServiceImpl {
	return &TeamServiceImpl{
		repo:       teamRepo,
		coreRepo:   coreRepo,
		db:         db,
		mailClient: mailClient,
	}
}

// GenerateTemporaryPassword генерирует временный пароль для команды
func GenerateTemporaryPassword() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b)[:10], nil
}

// RegisterTeam регистрирует новую команду
func (s *TeamServiceImpl) RegisterTeam(email, name string) error {
	// Генерируем временный пароль
	tempPassword, err := GenerateTemporaryPassword()
	if err != nil {
		return err
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	team := &models.Team{
		Name:         name,
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repo.Create(team); err != nil {
		return err
	}

	// Отправляем пароль на почту
	if err := s.mailClient.SendTempPassword(email, tempPassword); err != nil {
		// Если отправка не удалась - удаляем команду
		_ = s.repo.Delete(team.ID)
		return fmt.Errorf("не удалось отправить пароль: %v", err)
	}

	return nil
}

// AuthenticateTeam аутентифицирует команду по email и паролю
func (s *TeamServiceImpl) AuthenticateTeam(email, password string) (*models.Team, error) {
	team, err := s.repo.FindByEmail(email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	// Проверяем пароль
	err = bcrypt.CompareHashAndPassword([]byte(team.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	return team, nil
}

// LinkTelegramToTeam связывает Telegram ID с командой
func (s *TeamServiceImpl) LinkTelegramToTeam(email string, telegramID int64) error {
	team, err := s.repo.FindByEmail(email)
	if err != nil {
		return errors.New("team not found")
	}

	// Проверяем, не привязан ли уже этот Telegram ID к другой команде
	existingTeam, err := s.repo.FindByTelegramID(telegramID)
	if err == nil && existingTeam.ID != team.ID {
		return errors.New("this telegram ID is already linked to another team")
	}

	// Связываем Telegram ID с командой
	team.TelegramID = telegramID
	return s.repo.Update(team)
}

// JoinContest записывает команду на активный контест
func (s *TeamServiceImpl) JoinContest(teamID uuid.UUID) (*models.Contest, error) {

	team, err := s.repo.FindByID(teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}

	// Получаем активный контест
	contest, err := s.repo.GetActiveContest()
	if err != nil {
		return nil, fmt.Errorf("в данный момент нет активных контестов")
	}

	// Привязываем команду к контесту
	team.ContestID = &contest.ID
	if err := s.repo.Update(team); err != nil {
		return nil, err
	}

	return contest, nil
}

// GetTask возвращает задачу для команды
func (s *TeamServiceImpl) GetTask(teamID uuid.UUID) (*models.Task, error) {
	team, err := s.repo.FindByID(teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}

	// Проверяем, что команда участвует в контесте
	if team.ContestID == nil || team.CompanyID == nil {
		return nil, errors.New("team is not assigned to contest or company")
	}

	// Получаем ID задач, которые команда уже решала
	usedIDs, _ := s.repo.GetUsedTaskIDs(team.ID)

	// Получаем новую уникальную задачу
	var task models.Task
	query := s.db.Where("contest_id = ? AND company_id = ?", *team.ContestID, *team.CompanyID)

	if len(usedIDs) > 0 {
		query = query.Where("id NOT IN ?", usedIDs)
	}

	err = query.First(&task).Error

	if err != nil {
		return nil, fmt.Errorf("нет доступных задач: %v", err)
	}

	// Обновляем текущую задачу команды
	team.CurrentTaskID = &task.ID
	if err := s.repo.Update(team); err != nil {
		return nil, err
	}

	// Создаем сессию задачи
	session := &models.TeamTaskSession{
		TeamID:    team.ID,
		TaskID:    task.ID,
		StartTime: time.Now(),
		Attempts:  0,
		Finished:  false,
	}
	_ = s.repo.CreateTaskSession(session)

	return &task, nil
}

// SubmitAnswer проверяет ответ команды на задачу
// SubmitAnswer проверяет ответ команды на задачу
func (s *TeamServiceImpl) SubmitAnswer(teamID uuid.UUID, taskID uuid.UUID, answer string) (bool, error) {
	team, err := s.repo.FindByID(teamID)
	if err != nil {
		return false, errors.New("team not found")
	}

	if team.CurrentTaskID == nil || *team.CurrentTaskID != taskID {
		return false, errors.New("team is not working on this task")
	}

	task, err := s.coreRepo.GetTaskByID(context.TODO(), taskID)
	if err != nil {
		return false, errors.New("task not found")
	}

	session, err := s.repo.GetTaskSession(team.ID, taskID)
	if err != nil {
		return false, errors.New("task session not found")
	}

	if session.Finished || session.Attempts >= 3 || time.Since(session.StartTime) > 10*time.Minute {
		return false, errors.New("Task is finished or timed out")
	}

	session.Attempts++
	isCorrect := task.CorrectAnswer == answer
	session.IsCorrect = isCorrect

	// === ДОБАВЛЕНО: расчёт и накопление времени, если задача завершена ===
	if isCorrect || session.Attempts >= 3 || time.Since(session.StartTime) > 10*time.Minute {
		session.Finished = true

		// === Вычисляем фактическое время выполнения задачи ===
		endTime := time.Now()
		duration := endTime.Sub(session.StartTime)
		if duration > 10*time.Minute {
			duration = 10 * time.Minute // Лимит
		}

		// === Обновляем очки, если ответ правильный ===
		if isCorrect {
			team.Points += 1
		}

		// === Правильно накапливаем время в команде ===
		// Преобразуем текущее TotalDuration в time.Duration
		currentDuration := team.TotalDuration.Duration()

		// Добавляем новую продолжительность
		newTotalDuration := currentDuration + duration

		// Обновляем TotalDuration команды
		team.TotalDuration = models.PGInterval(newTotalDuration)

		err := s.repo.Update(team)
		if err != nil {
			return false, errors.New("failed to update team with duration")
		}
	}
	_ = s.repo.UpdateTaskSession(session)

	teamAnswer := &models.TeamAnswer{
		TeamID:    teamID,
		TaskID:    taskID,
		Answer:    answer,
		IsCorrect: isCorrect,
	}
	_ = s.repo.SaveAnswer(teamAnswer)

	return isCorrect, nil
}

func (s *TeamServiceImpl) GetTeamByEmail(email string) (*models.Team, error) {
	team, err := s.repo.FindByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("team not found")
	}
	return team, nil
}

func (s *TeamServiceImpl) GetUnassignedTeams() ([]models.Team, error) {
	return s.repo.GetUnassignedTeams()
}

func (s *TeamServiceImpl) ApproveTeam(teamID, companyID uuid.UUID) error {
	return s.repo.ApproveTeam(teamID, companyID)
}

func (s *TeamServiceImpl) GetTaskSession(teamID, taskID uuid.UUID) (*models.TeamTaskSession, error) {
	return s.repo.GetTaskSession(teamID, taskID)
}

func (s *TeamServiceImpl) GetTeamByID(teamID uuid.UUID) (*models.Team, error) {
	return s.repo.FindByID(teamID)
}
func (s *TeamServiceImpl) GetCompanyCredentials(companyID uuid.UUID) (*models.Company, error) {
	return s.coreRepo.GetCompanyByID(context.Background(), companyID)
}
func (s *TeamServiceImpl) GetCompanyIDByTeam(teamID uuid.UUID) (uuid.UUID, error) {
	team, err := s.repo.FindByID(teamID)
	if err != nil || team.CompanyID == nil {
		return uuid.Nil, errors.New("team has no company")
	}
	return *team.CompanyID, nil
}

func (s *TeamServiceImpl) GetCompanyByID(companyID uuid.UUID) (*models.Company, error) {
	return s.coreRepo.GetCompanyByID(context.Background(), companyID)
}
