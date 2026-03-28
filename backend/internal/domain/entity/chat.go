package entity

import "time"

type ChatSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:255" json:"title"`
	Provider  string    `gorm:"size:50;not null" json:"provider"`
	Model     string    `gorm:"size:100;not null" json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"index;not null" json:"session_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Role      string    `gorm:"size:20;not null" json:"role"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Provider  string    `gorm:"size:50;not null" json:"provider"`
	Model     string    `gorm:"size:100;not null" json:"model"`
	CreatedAt time.Time `json:"created_at"`
}
