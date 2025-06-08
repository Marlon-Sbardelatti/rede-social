package models

import "time"

type Post struct {
	Id          int64   
	Description string  
	Likes       []int64 
	Images      []string
	CreatedAt   time.Time
}

func NewPost(description string, images []string) *Post {
	return &Post{
		Description: description,
		Images: images,
		CreatedAt: time.Now(),
	}
}
