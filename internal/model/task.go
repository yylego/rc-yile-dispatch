package model

import "gorm.io/gorm"

// TaskStatus represents the state of a notification task
type TaskStatus string

const (
	StatusPending  TaskStatus = "pending"  // waiting to be dispatched
	StatusRunning  TaskStatus = "running"  // being dispatched
	StatusSuccess  TaskStatus = "success"  // dispatched OK
	StatusFailed   TaskStatus = "failed"   // dispatched but got non-2xx response
	StatusDeadLine TaskStatus = "deadline" // exceeded max retries, needs manual inspection
)

// Task is the core entity — one notification request to be dispatched
type Task struct {
	gorm.Model
	Method     string     `gorm:"type:varchar(10);not null"`       // HTTP method (GET/POST/PUT/PATCH/DELETE)
	TargetURL  string     `gorm:"type:text;not null"`              // destination URL
	Headers    string     `gorm:"type:text"`                       // JSON-encoded request headers
	Body       string     `gorm:"type:text"`                       // request body
	Status     TaskStatus `gorm:"type:varchar(20);not null;index"` // task status
	Retries    int        `gorm:"not null;default:0"`              // number of retries so far
	MaxRetries int        `gorm:"not null;default:5"`              // max retries before deadline
	NextRunAt  int64      `gorm:"not null;index"`                  // next execution time (unix seconds)
	LastError  string     `gorm:"type:text"`                       // last dispatch error message
	Callback   string     `gorm:"type:varchar(64)"`                // business reference tag (optional)
}
