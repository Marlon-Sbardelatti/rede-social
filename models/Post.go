package models

import "time"

type Post struct {
	Id          int64   `json:"id"`
	Description string  `json:"description,omitempty"`
	Images      []string `json:"images"`
	CreatedAt   time.Time `json:"createdAt"`
}

func NewPost(description string, images []string) *Post {
	return &Post{
		Description: description,
		Images: images,
		CreatedAt: time.Now(),
	}
}
