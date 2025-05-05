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
