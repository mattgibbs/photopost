package controllers

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/mattgibbs/photopost/config"
	"github.com/mattgibbs/photopost/model"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

var allowed_file_types = [...]string{"image/jpeg", "image/gif", "image/png"}

type PostController struct {
	datastore     model.Datastore
	configuration *config.Config
}

func NewPostController(ds model.Datastore, configuration *config.Config) *PostController {
	c := new(PostController)
	c.datastore = ds
	c.configuration = configuration
	rand.Seed(time.Now().Unix())
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
	c.showPostWithID(w, r, postid)
}

func (c *PostController) PostRandom(w http.ResponseWriter, r *http.Request) {
	//Get the list of all ids
	ids, err := c.datastore.PostIDs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		panic(err)
		return
	}

	if ids == nil || len(ids) == 0 {
		http.Error(w, "Database has no photos.", http.StatusInternalServerError)
		return
	}

	id := int(ids[rand.Intn(len(ids))])
	c.showPostWithID(w, r, id)
}

func (c *PostController) showPostWithID(w http.ResponseWriter, r *http.Request, id int) {
	post, err := c.datastore.FindPost(id)
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

func validateImageFile(fh *multipart.FileHeader) bool {
	isAllowedType := false
	for i := range allowed_file_types {
		if fh.Header.Get("Content-Type") == allowed_file_types[i] {
			isAllowedType = true
		}
	}
	return isAllowedType
}

func (c *PostController) filenameForImageFile(fBytes *[]byte, fh *multipart.FileHeader) (string, error) {
	imageHash := sha1.New()
	imageHash.Write(*fBytes)
	hashBytes := imageHash.Sum(nil)
	fileExtensions, mimeErr := mime.ExtensionsByType(fh.Header.Get("Content-Type"))
	if mimeErr != nil {
		return "", mimeErr
	}
	if fileExtensions == nil {
		return "", errors.New("Could not determine file extension for uploaded image.")
	}

	return filepath.Join(c.configuration.UploadsPath, fmt.Sprintf("%x%s", hashBytes, fileExtensions[0])), nil
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
	isAllowedType := validateImageFile(fh)
	if isAllowedType == false {
		http.Error(w, "Uploaded image is not an allowed file type.", http.StatusUnprocessableEntity)
		return
	}
	fBytes, err := ioutil.ReadAll(io.LimitReader(f, 10<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	imageFilename, err := c.filenameForImageFile(&fBytes, fh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var post model.Post
	post.Title = r.FormValue("title")
	post.Text = r.FormValue("text")
	post.ImageFile = imageFilename
	post.Author = r.FormValue("author")
	if len(r.FormValue("postTime")) > 0 {
		postTimeString := r.FormValue("postTime")
		t, err := time.Parse(time.RFC3339, postTimeString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		post.PostTime = t
	}
	valid, validation_err := post.Validate()
	if !valid {
		http.Error(w, validation_err.Error(), http.StatusUnprocessableEntity)
		return
	}
	err = ioutil.WriteFile(imageFilename, fBytes, 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	new_post_id, err := c.datastore.SavePost(&post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Location", fmt.Sprintf("posts/%v", new_post_id))
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(&post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *PostController) PostUpdate(w http.ResponseWriter, r *http.Request) {
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
	r.ParseMultipartForm(10 << 20) //Limit to 10 MB file size
	f, fh, err := r.FormFile("image")
	var fBytes []byte
	var imageFilename string
	if err == nil {
		defer f.Close()
		isAllowedType := validateImageFile(fh)
		if isAllowedType == false {
			http.Error(w, "Uploaded image is not an allowed file type.", http.StatusUnprocessableEntity)
			return
		}
		fBytes, err = ioutil.ReadAll(io.LimitReader(f, 10<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		imageFilename, err = c.filenameForImageFile(&fBytes, fh)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		post.ImageFile = imageFilename
	}
	if len(r.FormValue("title")) > 0 {
		post.Title = r.FormValue("title")
	}
	if len(r.FormValue("text")) > 0 {
		post.Text = r.FormValue("text")
	}
	if len(r.FormValue("author")) > 0 {
		post.Author = r.FormValue("author")
	}
	if len(r.FormValue("postTime")) > 0 {
		postTimeString := r.FormValue("postTime")
		t, err := time.Parse(time.RFC3339, postTimeString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		post.PostTime = t
	}

	valid, validation_err := post.Validate()
	if !valid {
		http.Error(w, validation_err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if len(fBytes) > 0 && len(imageFilename) > 0 {
		err = ioutil.WriteFile(imageFilename, fBytes, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	err = c.datastore.UpdatePost(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *PostController) PostDelete(w http.ResponseWriter, r *http.Request) {
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
	err = c.datastore.DeletePost(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusNoContent)
}

func (c *PostController) PostRotate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postid, err := strconv.Atoi(vars["postid"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	direction, err := strconv.Atoi(r.FormValue("direction"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, err := c.datastore.FindPost(postid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var directionArg string
	if direction >= 0 {
		directionArg = "90"
	} else {
		directionArg = "-90"
	}
	cmd := exec.Command("mogrify", "-rotate", directionArg, post.ImageFile)
	err = cmd.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
