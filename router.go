package main

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewRouter() *mux.Router {
	var routes = Routes{
		Route{
			"Index", "GET", "/", Index,
		},
		Route{
			"PostShow", "GET", "/posts/{postid}", postController.PostShow,
		},
		Route{
			"PostCreate", "POST", "/posts", postController.PostCreate,
		},
	}

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
		router.Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}
	return router
}
