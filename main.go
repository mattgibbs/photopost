package main

import (
	"fmt"
	"github.com/mattgibbs/photopost/config"
	"github.com/mattgibbs/photopost/controllers"
	"github.com/mattgibbs/photopost/model"
	"log"
	"net/http"
	"os"
)

var datastore model.Datastore
var postController *controllers.PostController
var configuration config.Config

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: photopost <config file>")
		os.Exit(0)
	}
	configuration, err := config.LoadConfig(os.Args[1])
	if err != nil {
		log.Fatalf("Error: Could not load config file. %s", err)
	}
	log.Println("Starting photopost server.")
	log.Printf("Configuration: %+v\n", configuration)
	datastore := model.NewSQLiteDatastore(fmt.Sprintf("%s?mode=rwc", configuration.DatabaseURL))
	postController = controllers.NewPostController(datastore, configuration)
	defer datastore.Close()
	router := NewRouter()
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", configuration.Port), router))
}
