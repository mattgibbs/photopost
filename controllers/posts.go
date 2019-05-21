package controllers

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/mattgibbs/photopost/model"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

var allowed_file_types = [...]string{"image/jpeg", "image/gif", "image/png"}

type PostController struct {
	datastore model.Datastore
}

func NewPostController(ds model.Datastore) *PostController {
	c := new(PostController)
	c.datastore = ds
	return c
}

func (c *PostController) PostIndex(w http.ResponseWriter, r *http.Request) {
	var filters []interface{}
	start_time := r.FormValue("start_time")
	end_time := r.FormValue("end_time")
	if start_time != "" || end_time != "" {
		etf := model.PostTimeFilter{}
		var err error
		shortForm := "2006-Jan-02"
		if start_time != "" {
			etf.Newer_than, err = time.Parse(shortForm, start_time)
			if err != nil {
				http.Error(w, "start_time must be in 2006-Jan-02 format.", http.StatusBadRequest)
				return
			}
		}
		if end_time != "" {
			etf.Older_than, err = time.Parse(shortForm, end_time)
			if err != nil {
				http.Error(w, "end_time must be in 2006-Jan-02 format.", http.StatusBadRequest)
				return
			}
		}
		filters = append(filters, etf)
	}

	if title_contains := r.FormValue("title_contains"); title_contains != "" {
		titleFilter := model.TitleFilter{Contains: title_contains}
		filters = append(filters, titleFilter)
	}

	posts, err := c.datastore.FindPostsWithFilters(filters)
	if err != nil {
		log.Fatalf("Error while fetching entries: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		panic(err)
	}
}

func (c *PostController) PostShow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postid, err := strconv.Atoi(vars["postid"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, err := c.datastore.FindPost(postid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		panic(err)
	}
}

func (c *PostController) PostCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20) //Limit to 10 MB file size
	f, fh, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	log.Printf("Saving a new file: %s with size %+v and type %+v", fh.Filename, fh.Size, fh.Header)
	is_allowed_type := false
	for i := range allowed_file_types {
		if fh.Header.Get("Content-Type") == allowed_file_types[i] {
			is_allowed_type = true
		}
	}
	if is_allowed_type == false {
		http.Error(w, "Uploaded image is not an allowed file type.", http.StatusUnprocessableEntity)
		return
	}
	fBytes, err := ioutil.ReadAll(io.LimitReader(f, 10<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	imageHash := sha1.New()
	imageHash.Write(fBytes)
	hashBytes := imageHash.Sum(nil)
	fileExtensions, mimeErr := mime.ExtensionsByType(fh.Header.Get("Content-Type"))
	if mimeErr != nil {
		http.Error(w, mimeErr.Error(), http.StatusUnprocessableEntity)
		return
	}
	if fileExtensions == nil {
		http.Error(w, "Could not determine file extension for uploaded image.", http.StatusUnprocessableEntity)
		return
	}

	imageFile := filepath.Join("uploads", fmt.Sprintf("%x%s", hashBytes, fileExtensions[0]))
	var post model.Post
	post.Title = r.FormValue("title")
	post.Text = r.FormValue("text")
	post.ImageFile = imageFile
	post.Author = r.FormValue("author")
	valid, validation_err := post.Validate()
	if !valid {
		http.Error(w, validation_err.Error(), http.StatusUnprocessableEntity)
		return
	}
	err = ioutil.WriteFile(imageFile, fBytes, 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = c.datastore.SavePost(&post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(&post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
