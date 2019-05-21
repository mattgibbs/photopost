package model

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
	"time"
)

func initSQLiteDB(addr string) *sql.DB {
	db, err := sql.Open("sqlite3", addr)
	if err != nil {
		log.Fatalf("Unable to open sqlite database: %s", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Unable to establish communication with sqlite database: %s", err)
	}
	return db
}

type scannable interface {
	Scan(dest ...interface{}) error
}

type ds struct {
	db                *sql.DB
	save_post_stmt    *sql.Stmt
	find_post_stmt    *sql.Stmt
	findall_post_stmt *sql.Stmt
	update_post_stmt  *sql.Stmt
	delete_post_stmt  *sql.Stmt
}

var save_post_sql = "INSERT INTO posts(title, text, image_file, author, post_time, creation_time) VALUES (?, ?, ?, ?, ?, ?)"
var findall_post_sql = `SELECT id, title, text, image_file, author, post_time, creation_time FROM posts`
var find_post_sql = findall_post_sql + " WHERE id = ?"
var findall_post_sql_ordered = findall_post_sql + " ORDER BY post_time DESC"
var delete_post_sql = "DELETE FROM posts WHERE id = ?"
var update_post_sql = "UPDATE posts SET title = ?, text = ?, image_file = ?, author = ?, post_time = ? WHERE id = ?"

func NewSQLiteDatastore(addr string) *ds {
	d := initSQLiteDB(addr)
	createTables(d)
	save_post_stmt, err := d.Prepare(save_post_sql)
	if err != nil {
		log.Fatalf("Error while preparing post save statement: %s", err)
	}
	find_post_stmt, err := d.Prepare(find_post_sql)
	if err != nil {
		log.Fatalf("Error while preparing post find statement: %s", err)
	}
	findall_post_stmt, err := d.Prepare(findall_post_sql_ordered)
	if err != nil {
		log.Fatalf("Error while preparing post findAll statement: %s", err)
	}
	update_post_stmt, err := d.Prepare(update_post_sql)
	if err != nil {
		log.Fatalf("Error while preparing post update statement: %s", err)
	}
	delete_post_stmt, err := d.Prepare(delete_post_sql)
	if err != nil {
		log.Fatalf("Error while preparing post delete statement: %s", err)
	}
	return &ds{
		db:                d,
		save_post_stmt:    save_post_stmt,
		find_post_stmt:    find_post_stmt,
		findall_post_stmt: findall_post_stmt,
		update_post_stmt:  update_post_stmt,
		delete_post_stmt:  delete_post_stmt,
	}
}

func (d *ds) FindPost(id int) (*Post, error) {
	row := d.find_post_stmt.QueryRow(id)
	result, scanErr := scanPostFromRow(row)
	if scanErr != nil {
		return nil, scanErr
	}
	return result, nil
}

func (d *ds) FindAllPosts() ([]*Post, error) {
	rows, err := d.findall_post_stmt.Query()
	if err != nil {
		log.Printf("Error during Post FindAll: %s", err)
		return nil, err
	}
	defer rows.Close()
	return scanPostsFromRows(rows)
}

func (d *ds) FindPostsWithFilters(filters []interface{}) ([]*Post, error) {
	if len(filters) == 0 {
		return d.FindAllPosts()
	}
	query := findall_post_sql + " WHERE "
	var args []interface{}
	var clauses []string
	for _, filter := range filters {
		switch f := filter.(type) {
		case PostTimeFilter:
			if !f.Newer_than.IsZero() {
				clauses = append(clauses, "post_time > ?")
				args = append(args, f.Newer_than.Unix())
			}

			if !f.Older_than.IsZero() {
				clauses = append(clauses, "post_time < ?")
				args = append(args, f.Older_than.Unix())
			}
		case CreationTimeFilter:
			if !f.Newer_than.IsZero() {
				clauses = append(clauses, "creation_time > ?")
				args = append(args, f.Newer_than.Unix())
			}

			if !f.Older_than.IsZero() {
				clauses = append(clauses, "creation_time < ?")
				args = append(args, f.Older_than.Unix())
			}
		case TitleFilter:
			if f.Matching != "" {
				clauses = append(clauses, "title = ?")
				args = append(args, f.Matching)
			}
			if f.Contains != "" {
				clauses = append(clauses, "title LIKE '%' || ? || '%'")
				args = append(args, f.Contains)
			}
		case TextFilter:
			if f.Contains != "" {
				clauses = append(clauses, "text LIKE '%' || ? || '%'")
				args = append(args, f.Contains)
			}
		case PostIdFilter:
			addIdFilterClause(f.PostIds, "id", clauses, args)
		case AuthorFilter:
			if f.Matching != "" {
				clauses = append(clauses, "author = ?")
				args = append(args, f.Matching)
			}
			if f.Contains != "" {
				clauses = append(clauses, "author LIKE '%' || ? || '%'")
				args = append(args, f.Contains)
			}
		default:
			return nil, errors.New("Unknown filter type.")
		}
	}
	query = query + strings.Join(clauses, " AND ")
	query = query + " ORDER BY post_time DESC"
	rows, queryErr := d.db.Query(query, args...)
	defer rows.Close()
	if queryErr != nil {
		return nil, queryErr
	}
	return scanPostsFromRows(rows)
}

func addIdFilterClause(ids []int64, columnName string, clauses []string, args []interface{}) {
	if len(ids) > 0 {
		idClause := "%s IN (%s)"
		questionMarks := make([]string, len(ids))
		for i, _ := range questionMarks {
			questionMarks[i] = "?"
		}
		clauses = append(clauses, fmt.Sprintf(idClause, columnName, strings.Join(questionMarks, ",")))
		for _, id := range ids {
			args = append(args, id)
		}
	}
}

func scanPostsFromRows(rows *sql.Rows) ([]*Post, error) {
	var err error
	posts := make(map[int64]*Post)
	for rows.Next() {
		rowErr := rows.Err()
		if rowErr != nil {
			log.Printf("Row Errow during Post FindAll: %s", rowErr)
			err = rowErr
			break
		}
		result, scanErr := scanPostFromRow(rows)
		if scanErr != nil {
			log.Printf("Error while scanning row during Post FindAll: %s", scanErr)
			err = scanErr
			continue
		}
		posts[result.Id] = result
	}

	postList := []*Post{}
	for id := range posts {
		postList = append(postList, posts[id])
	}
	return postList, err
}

func scanPostFromRow(row scannable) (*Post, error) {
	//id, title, text, image_file, author, post_time, creation_time
	post := Post{}
	var postTimestamp int64
	var creationTimestamp int64
	err := row.Scan(&post.Id, &post.Title, &post.Text, &post.ImageFile, &post.Author, &postTimestamp, &creationTimestamp)
	if err != nil {
		return nil, err
	}
	post.PostTime = time.Unix(postTimestamp, 0)
	post.CreationTime = time.Unix(creationTimestamp, 0)
	return &post, nil
}

func (d *ds) SavePost(post *Post) (int64, error) {
	transaction, txErr := d.db.Begin()
	defer transaction.Rollback()
	//First, insert a new row into the 'posts' table.
	if txErr != nil {
		log.Printf("Error while creating post save transaction: %s", txErr)
		return -1, txErr
	}
	stmt, prepErr := transaction.Prepare(save_post_sql)
	defer stmt.Close()
	if prepErr != nil {
		log.Printf("Error while preparing post insert statement: %s", prepErr)
		return -1, prepErr
	}
	lastId, saveErr := savePostWithStatement(post, stmt)
	if saveErr != nil {
		log.Printf("Error while saving new post: %s", saveErr)
		return -1, saveErr
	}

	commitErr := transaction.Commit()
	if commitErr != nil {
		log.Printf("Error while commiting post save transaction: %s", commitErr)
		return -1, commitErr
	}
	post.Id = lastId
	return lastId, nil
}

func savePostWithStatement(post *Post, stmt *sql.Stmt) (int64, error) {
	post.CreationTime = time.Now()
	if post.PostTime.IsZero() {
		post.PostTime = post.CreationTime
	}
	//title, text, image_file, author, post_time, creation_time)
	res, execErr := stmt.Exec(post.Title, post.Text, post.ImageFile, post.Author, post.PostTime.Unix(), post.CreationTime.Unix())
	if execErr != nil {
		log.Printf("Error while executing save statement: %s", execErr)
		return -1, execErr
	}
	lastId, lastIdErr := res.LastInsertId()
	if lastIdErr != nil {
		log.Printf("Error while fetching last ID for saved post: %s", lastIdErr)
		return -1, lastIdErr
	}
	return lastId, nil
}

func (d *ds) UpdatePost(post *Post) error {
	//"UPDATE posts SET title = ?, text = ?, image_file = ?, author = ?, post_time = ? WHERE id = ?"
	if post.Id == 0 {
		return errors.New("Cannot update a post without an ID.")
	}
	_, err := d.update_post_stmt.Exec(post.Title, post.Text, post.ImageFile, post.Author, post.PostTime, post.Id)
	return err
}

func (d *ds) DeletePost(post *Post) error {
	if post.Id == 0 {
		return errors.New("Cannot delete a post without an ID.")
	}
	_, err := d.delete_post_stmt.Exec(post.Id)
	return err
}

func (d *ds) Close() {
	log.Print("Closing SQLite Datastore.")
	d.save_post_stmt.Close()
	d.find_post_stmt.Close()
	d.findall_post_stmt.Close()
	d.delete_post_stmt.Close()
	d.update_post_stmt.Close()
	d.db.Close()
}

func createTables(db *sql.DB) {
	transaction, err := db.Begin()
	if err != nil {
		log.Fatalf("Error while creating table transaction: %s", err)
	}
	//Create entries table
	_, err = transaction.Exec("CREATE TABLE IF NOT EXISTS posts (id integer PRIMARY KEY, title string NOT NULL, text string, image_file string NOT NULL, author string NOT NULL, post_time integer NOT NULL, creation_time integer NOT NULL)")
	if err != nil {
		log.Fatalf("Error while creating posts table: %s", err)
	}
	transaction.Commit()
}
