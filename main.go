package main

import (
	"github.com/mattgibbs/photopost/controllers"
	"github.com/mattgibbs/photopost/model"
	"log"
	"net/http"
)

var datastore model.Datastore
var postController *controllers.PostController

func main() {
	datastore := model.NewSQLiteDatastore("posts.db?mode=rwc")
	postController = controllers.NewPostController(datastore)
	defer datastore.Close()
	router := NewRouter()
	log.Fatal(http.ListenAndServe(":8888", router))
}
