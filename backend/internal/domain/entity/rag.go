package entity

import "time"

type RAGDocument struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Content   string    `gorm:"type:longtext;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RAGChunk struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"index;not null" json:"document_id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Embedding  string    `gorm:"type:longtext;not null" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}
