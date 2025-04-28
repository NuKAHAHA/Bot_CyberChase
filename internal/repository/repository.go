package repository

import (
	"Cyber-chase/internal/models"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

var ErrNotFound = errors.New("record not found")

func (r *Repository) CreateCompany(ctx context.Context, company *models.Company) error {
	return r.db.WithContext(ctx).Create(company).Error
}

func (r *Repository) GetCompanyByEmail(ctx context.Context, email string) (*models.Company, error) {
	var company models.Company
	err := r.db.WithContext(ctx).First(&company, "email = ?", email).Error
	return &company, err
}

func (r *Repository) GetCompanyByID(ctx context.Context, id uuid.UUID) (*models.Company, error) {
	var company models.Company
	err := r.db.WithContext(ctx).First(&company, "id = ?", id).Error
	return &company, err
}

func (r *Repository) GetAllCompanies(ctx context.Context) ([]models.Company, error) {
	var companies []models.Company
	err := r.db.WithContext(ctx).Find(&companies).Error
	return companies, err
}

func (r *Repository) UpdateCompany(ctx context.Context, company *models.Company) error {
	return r.db.WithContext(ctx).Save(company).Error
}

func (r *Repository) DeleteCompany(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Company{}, "id = ?", id).Error
}

func (r *Repository) CreateContest(ctx context.Context, contest *models.Contest) error {
	return r.db.WithContext(ctx).Create(contest).Error
}

func (r *Repository) GetContestByID(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	var contest models.Contest
	err := r.db.WithContext(ctx).Preload("Tasks").Where("id = ?", id).First(&contest).Error
	return &contest, err
}

func (r *Repository) GetAllContests(ctx context.Context) ([]models.Contest, error) {
	var contests []models.Contest
	err := r.db.WithContext(ctx).Preload("Tasks").Find(&contests).Error
	return contests, err
}

func (r *Repository) UpdateContest(ctx context.Context, contest *models.Contest) error {
	return r.db.WithContext(ctx).Save(contest).Error
}

func (r *Repository) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Contest{}, "id = ?", id).Error
}

func (r *Repository) CreateTask(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error
	return &task, err
}

func (r *Repository) ApproveTask(ctx context.Context, taskID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("is_approved", true).Error
}

func (r *Repository) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Task{}, "id = ?", taskID).Error
}

func (r *Repository) GetTasksByCompanyID(ctx context.Context, companyID uuid.UUID) ([]models.Task, error) {
	var tasks []models.Task
	err := r.db.WithContext(ctx).Where("company_id = ?", companyID).Find(&tasks).Error
	return tasks, err
}

func (r *Repository) UpdateTask(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *Repository) UpdateTaskWithFile(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *Repository) CreateTeam(ctx context.Context, team *models.Team) error {
	return r.db.WithContext(ctx).Create(team).Error
}

func (r *Repository) GetTeamByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := r.db.WithContext(ctx).First(&team, "id = ?", id).Error
	return &team, err
}

func (r *Repository) UpdateTeam(ctx context.Context, team *models.Team) error {
	return r.db.WithContext(ctx).Save(team).Error
}

func (r *Repository) GetAllTeams(ctx context.Context) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.WithContext(ctx).Find(&teams).Error
	return teams, err
}

func (r *Repository) CreateTeamTask(ctx context.Context, teamTask *models.TeamTask) error {
	return r.db.WithContext(ctx).Create(teamTask).Error
}

func (r *Repository) GetTeamTasks(ctx context.Context, teamID uuid.UUID) ([]models.TeamTask, error) {
	var tasks []models.TeamTask
	err := r.db.WithContext(ctx).Where("team_id = ?", teamID).Find(&tasks).Error
	return tasks, err
}

func (r *Repository) GetAllTeamsWithActiveTasks(ctx context.Context) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.WithContext(ctx).Where("current_task_id IS NOT NULL").Find(&teams).Error
	return teams, err
}

func (r *Repository) GetTeamByContactInfo(ctx context.Context, contactID string) (*models.Team, error) {
	var team models.Team
	err := r.db.WithContext(ctx).Where("contact_info = ?", contactID).First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &team, err
}

func (r *Repository) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx)
	})
}

// In repository/repository.go
func (r *Repository) GetNextAvailableCompany(ctx context.Context, contestID uuid.UUID) (*models.Company, error) {
	// Instead of directly querying by contest_id, we need to join with tasks
	var company models.Company

	// First approach: Get companies that have tasks for this contest and no current team
	result := r.db.Table("companies").
		Joins("JOIN tasks ON tasks.company_id = companies.id").
		Where("tasks.contest_id = ? AND companies.current_team_id IS NULL", contestID).
		Order("companies.id").
		First(&company)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}

	return &company, nil
}

func (r *Repository) GetTaskByCompanyID(ctx context.Context, companyID uuid.UUID) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).
		Where("company_id = ?", companyID).
		First(&task).
		Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}
