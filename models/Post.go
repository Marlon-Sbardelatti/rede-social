package models

import "time"

type Post struct {
	Id          int64   
	UserID      int64   
	Description string  
	Likes       []int64 
	Images      [][]byte
	CreatedAt   time.Time
}

func NewPost(userId int64, description string, images [][]byte) *Post {
	return &Post{
		UserID: userId,
		Description: description,
		Images: images,
		CreatedAt: time.Now(),
	}
}
