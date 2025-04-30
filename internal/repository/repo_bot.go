package repository

import (
	"Cyber-chase/internal/models"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log"
)

// TeamRepository интерфейс для работы с командами
type TeamRepository interface {
	Create(team *models.Team) error
	FindByID(id uuid.UUID) (*models.Team, error)
	FindByEmail(email string) (*models.Team, error)
	FindByTelegramID(telegramID int64) (*models.Team, error)
	Update(team *models.Team) error
	Delete(id uuid.UUID) error
	SaveAnswer(answer *models.TeamAnswer) error
	GetActiveContest() (*models.Contest, error)
	GetTaskForTeam(teamID uuid.UUID, contestID uuid.UUID) (*models.Task, error)
	GetUnassignedTeams() ([]models.Team, error)
	ApproveTeam(teamID, companyID uuid.UUID) error
}

// GormTeamRepository имплементация TeamRepository с использованием GORM
type GormTeamRepository struct {
	db *gorm.DB
}

// NewTeamRepository создает новый репозиторий для работы с командами
func NewTeamRepository(db *gorm.DB) *GormTeamRepository {
	return &GormTeamRepository{db: db}
}

// Create сохраняет новую команду в базе данных
func (r *GormTeamRepository) Create(team *models.Team) error {
	return r.db.Create(team).Error
}

// FindByID находит команду по ID
func (r *GormTeamRepository) FindByID(id uuid.UUID) (*models.Team, error) {
	var team models.Team
	if err := r.db.Where("id = ?", id).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

// FindByEmail находит команду по email
func (r *GormTeamRepository) FindByEmail(email string) (*models.Team, error) {
	var team models.Team
	if err := r.db.Where("email = ?", email).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

// FindByTelegramID находит команду по Telegram ID
func (r *GormTeamRepository) FindByTelegramID(telegramID int64) (*models.Team, error) {
	var team models.Team
	if err := r.db.Where("telegram_id = ?", telegramID).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

// Update обновляет информацию о команде
func (r *GormTeamRepository) Update(team *models.Team) error {
	return r.db.Save(team).Error
}

// Delete удаляет команду из базы данных
func (r *GormTeamRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Team{}, id).Error
}

// SaveAnswer сохраняет ответ команды на задачу
func (r *GormTeamRepository) SaveAnswer(answer *models.TeamAnswer) error {
	return r.db.Create(answer).Error
}

// GetActiveContest возвращает активный контест
func (r *GormTeamRepository) GetActiveContest() (*models.Contest, error) {
	var contest models.Contest

	// Ищем контест только по статусу
	err := r.db.
		Where("status = ?", "active").
		First(&contest).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("активный контест не найден")
	}

	if err != nil {
		return nil, fmt.Errorf("ошибка при поиске контеста: %v", err)
	}

	log.Printf("Найден активный контест: ID=%s, Name=%s", contest.ID, contest.Name)
	return &contest, nil
}

// GetTaskForTeam возвращает задание для команды в контесте
func (r *GormTeamRepository) GetTaskForTeam(teamID uuid.UUID, contestID uuid.UUID) (*models.Task, error) {
	var team models.Team
	if err := r.db.Where("id = ?", teamID).First(&team).Error; err != nil {
		return nil, err
	}

	if team.CompanyID == nil {
		return nil, errors.New("team has no company assigned")
	}

	var task models.Task
	err := r.db.Where("contest_id = ? AND company_id = ?", contestID, team.CompanyID).
		First(&task).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("no available tasks for this company")
	}

	return &task, err
}

func (r *GormTeamRepository) GetUnassignedTeams() ([]models.Team, error) {
	var teams []models.Team
	err := r.db.Where("company_id IS NULL").Find(&teams).Error
	return teams, err
}

func (r *GormTeamRepository) ApproveTeam(teamID, companyID uuid.UUID) error {
	return r.db.Model(&models.Team{}).
		Where("id = ?", teamID).
		Update("company_id", companyID).Error
}
