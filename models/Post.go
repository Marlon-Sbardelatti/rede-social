package models

import "time"

type Post struct {
	Id          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	UserName    string    `json:"username"`
	Description string    `json:"description"`
	Images      []string  `json:"images"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewPost(description string, images []string) *Post {
	return &Post{
		Description: description,
		Images:      images,
		CreatedAt:   time.Now(),
	}
}
