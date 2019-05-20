package model

import (
	"errors"
	"time"
)

type Post struct {
	Id           int64     `json:"id"`
	Title        string    `json:"title"`
	Text         string    `json:"text"`
	ImageFile    string    `json:"imageFile"`
	PostTime     time.Time `json:"postTime"`
	CreationTime time.Time `json:"creationTime"`
	Author       string    `json:"author"`
	Saved        bool      `json:"-"`
}

type Posts []Post

func (e *Post) Validate() (success bool, err error) {
	if e.Author == "" {
		return false, errors.New("A post must have an author.")
	}
	if e.Title == "" {
		return false, errors.New("A post must have a title.")
	}

	if e.ImageFile == "" {
		return false, errors.New("A post must have an image.")
	}
	return true, nil
}
