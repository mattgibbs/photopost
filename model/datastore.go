package model

import "time"

type Datastore interface {
	//Post Methods
	FindPost(id int) (*Post, error)
	FindAllPosts() ([]*Post, error)
	FindPostsWithFilters(filters []interface{}) ([]*Post, error)
	SavePost(post *Post) (int64, error)
	UpdatePost(post *Post) error
	DeletePost(post *Post) error
	PostIDs() ([]int64, error)
	Close()
}

type PostTimeFilter struct {
	Newer_than time.Time
	Older_than time.Time
}

type CreationTimeFilter struct {
	Newer_than time.Time
	Older_than time.Time
}

type TitleFilter struct {
	Matching string
	Contains string
}

type TextFilter struct {
	Contains string
}

type PostIdFilter struct {
	PostIds []int64
}

type AuthorFilter struct {
	Matching string
	Contains string
}
