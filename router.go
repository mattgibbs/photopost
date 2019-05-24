package main

import (
	"github.com/gorilla/mux"
	"net/http"
)

const (
	VIEW_DIR   = "/view/"
	UPLOAD_DIR = "/uploads/"
)

func NewRouter() *mux.Router {
	var routes = Routes{
		Route{
			"Index", "GET", "/", Index,
		},
		Route{
			"PostRandom", "GET", "/posts/random", postController.PostRandom,
		},
		Route{
			"PostShow", "GET", "/posts/{postid}", postController.PostShow,
		},
		Route{
			"PostUpdate", "POST", "/posts/{postid}", postController.PostUpdate,
		},
		Route{
			"PostDelete", "DELETE", "/posts/{postid}", postController.PostDelete,
		},
		Route{
			"PostCreate", "POST", "/posts", postController.PostCreate,
		},
		Route{
			"PostIndex", "GET", "/posts", postController.PostIndex,
		},
	}

	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix(VIEW_DIR).Handler(http.StripPrefix(VIEW_DIR, http.FileServer(http.Dir("."+VIEW_DIR))))
	router.PathPrefix(UPLOAD_DIR).Handler(http.StripPrefix(UPLOAD_DIR, http.FileServer(http.Dir("."+UPLOAD_DIR))))
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
