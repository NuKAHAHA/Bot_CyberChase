package models

import (
	"github.com/google/uuid"
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
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name          string     `gorm:"not null"`
	Email         string     `gorm:"unique;not null"`
	PasswordHash  string     `gorm:"not null"`
	TelegramID    int64      `gorm:"unique"`
	ContestID     *uuid.UUID `gorm:"type:uuid"`
	CurrentTaskID *uuid.UUID `gorm:"type:uuid"`
	CompanyID     *uuid.UUID `gorm:"type:uuid"`
	Points        int        `gorm:"default:0"`
	TotalDuration PGInterval `gorm:"type:interval"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TeamAnswer представляет ответ команды на задачу
type TeamAnswer struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TeamID    uuid.UUID `gorm:"type:uuid;not null"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null"`
	Answer    string    `gorm:"not null"`
	IsCorrect bool      `gorm:"default:false"`
	CreatedAt time.Time
}

func UUIDFromString(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

type TeamTaskSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TeamID    uuid.UUID `gorm:"not null"`
	TaskID    uuid.UUID `gorm:"not null"`
	StartTime time.Time
	Attempts  int  `gorm:"default:0"`
	Finished  bool `gorm:"default:false"`
	IsCorrect bool `gorm:"default:false"`
	CreatedAt time.Time
}
