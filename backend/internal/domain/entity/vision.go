package entity

import "time"

type VisionTask struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Status       string    `gorm:"size:20;index;not null" json:"status"`
	Provider     string    `gorm:"size:50;not null" json:"provider"`
	Model        string    `gorm:"size:100;not null" json:"model"`
	Prompt       string    `gorm:"size:500" json:"prompt"`
	FileName     string    `gorm:"size:255;not null" json:"file_name"`
	MimeType     string    `gorm:"size:100;not null" json:"mime_type"`
	Result       string    `gorm:"type:longtext" json:"result"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	ImageBase64  string    `gorm:"type:longtext" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
