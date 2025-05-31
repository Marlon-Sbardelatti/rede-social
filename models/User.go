package models

type User struct {
	Id       int64 `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Follows []int64 `json:"follows,omitempty"`
}
