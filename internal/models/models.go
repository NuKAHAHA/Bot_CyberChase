package models

import (
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"time"
)

type Contest struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name      string    `gorm:"not null"`
	StartDate time.Time
	EndDate   time.Time
	Status    string `gorm:"default:'pending'"`
	Tasks     []Task
	CreatedAt time.Time
}

type Company struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name          string    `gorm:"unique;not null"`
	Email         string    `gorm:"unique;not null"`
	TempPassword  string    `gorm:"-"`
	PasswordHash  string    `gorm:"not null"`
	ResetRequired bool      `gorm:"default:true"`
	Location      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CurrentTeamID *uuid.UUID `gorm:"type:uuid"`
}

func (c *Company) BeforeSave(tx *gorm.DB) error {
	if c.TempPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(c.TempPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		c.PasswordHash = string(hashed)
	}
	return nil
}

type Task struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Question      string    `gorm:"not null"`
	QuestionFile  string
	CorrectAnswer string `gorm:"not null"`
	TimeLimit     int
	ContestID     uuid.UUID
	CompanyID     uuid.UUID
	CreatedAt     time.Time
}

type Team struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name                 string     `gorm:"unique;not null"`
	ContactInfo          string     `gorm:"not null"`
	ContestID            uuid.UUID  `gorm:"type:uuid;not null"`
	CurrentTaskID        *uuid.UUID `gorm:"type:uuid"`
	CurrentCompanyID     *uuid.UUID `gorm:"type:uuid"`
	CurrentTaskStartTime *time.Time
	AttemptsLeft         int `gorm:"default:3"`
	TotalTime            int `gorm:"default:0"`
	Score                int `gorm:"default:0"`
	CreatedAt            time.Time
}

type TeamTask struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TeamID    uuid.UUID `gorm:"type:uuid;not null"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null"`
	Status    string    `gorm:"not null"`
	TimeSpent int
	Attempts  int
	CreatedAt time.Time
}
