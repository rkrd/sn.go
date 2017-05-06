package sn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const CONTENT string = "text.txt"
const KEY string = "Key"

type User struct {
	Email string
	Auth  string
}

/*
{"modifydate": "1493471435.803190"
 "tags": ["Test_tag"]
 "deleted": 0
 "createdate": "1493471435.803190"
 "systemtags": []
 "version": 1
 "syncnum": 1
 "key": "3ba322992cdd11e7b405951ca8038d7c"
 "minversion": 1}
*/

type Note struct {
	Modifydate string   `json:"modifydate"`
	Tags       []string `json:"tags"`
	Deleted    int      `json:"deleted"`
	CreateDate string   `json:"createdate"`
	Systemtags []string `json:"systemtags,omitempty"`
	Content    string   `json:"content,omitempty"`
	Version    uint     `json:"version`
	Syncnum    int      `json:"syncnum"`
	Key        string   `json:"key"`
	Minversion uint     `json:"minversion"`
}

type Index struct {
	Count int
	Data  []Note
	Time  string
	Mark  string
}

func (n Note) PrintNote() {
	fmt.Println(parse_unix(n.Modifydate))
	fmt.Println("Tags", n.Tags)
	fmt.Println("Content", n.Content)
}

func GetAuth(email string, pass string) (User, error) {
	//client := http.Client{Timeout: time.Second * 10}
	var uri url.URL
	uri.Scheme = "https"
	uri.Host = "simple-note.appspot.com"
	uri.Path = "/api/login"

	v := url.Values{}
	v.Add("email", email)
	v.Add("password", pass)

	b64 := base64.StdEncoding.EncodeToString([]byte(v.Encode()))
	r, err := http.Post(uri.String(), "application/x-www-form-urlencoded", strings.NewReader(b64))
	defer r.Body.Close()

	if err != nil {
		return User{}, err
	}

	auth, err := ioutil.ReadAll(r.Body)

	var user User
	user.Email = email
	user.Auth = string(auth)
	return user, err
}

func getNotes(user User, mark string) (Index, error) {
	var i Index

	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)
	v.Add("mark", mark)

	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: "api2/index"}
	u.RawQuery = v.Encode()
	s, err := http.Get(u.String())
	defer s.Body.Close()
	if err != nil {
		panic(err)
	}
	if s.StatusCode != 200 {
		return i, errors.New(fmt.Sprintf("GetNote returned: %d", s.StatusCode))
	}

	d := json.NewDecoder(s.Body)

	err = d.Decode(&i)
	if err != nil {
		panic(err)
	}

	return i, nil
}

func (user User) GetAllNotes() (Index, error) {
	var i Index

	for {
		ii, err := getNotes(user, i.Mark)

		if err != nil {
			return i, err
		}

		i.Mark = ii.Mark
		i.Count += ii.Count
		i.Time = ii.Time
		i.Data = append(i.Data, ii.Data...)

		fmt.Printf("Marki %s Markii %s %d %d\n", i.Mark, ii.Mark, i.Count, ii.Count)
		if ii.Mark == "" {
			break
		}
	}

	return i, nil
}

func (user User) GetNote(key string, version int) (Note, error) {
	var no Note

	if len(key) == 0 {
		return no, errors.New("Note has no key empty")
	}

	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)

	//https://app.simplenote.com/api2/data
	var path string
	if version != 0 {
		path = fmt.Sprintf("api2/data/%s/%d", key, version)
	} else {
		path = fmt.Sprintf("api2/data/%s", key)
	}
	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: path}
	u.RawQuery = v.Encode()

	r, err := http.Get(u.String())
	defer r.Body.Close()
	if err != nil {
		panic(err)
	}
	if r.StatusCode != 200 {
		return no, errors.New(fmt.Sprintf("GetNote returned: %d on note: %s URL: %s", r.StatusCode, key, u.String()))
	}

	d := json.NewDecoder(r.Body)
	err = d.Decode(&no)

	if err != nil {
		panic(err)
	}

	return no, nil
}

/* Used to update an existing note or create a new note if Key of Note is
 * not set. */
func (user User) UpdateNote(n *Note) Note {
	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)

	var path string
	if n.Key != "" {
		path = fmt.Sprintf("api2/data/%s", n.Key)
	} else {
		path = "api2/data"
	}
	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: path}
	u.RawQuery = v.Encode()

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(n)

	r, err := http.Post(u.String(), "application/json; charset=utf-8", b)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		panic(r.Status)
	}

	d := json.NewDecoder(r.Body)
	var no Note
	err = d.Decode(&no)

	fmt.Println(no.Key)

	if err != nil {
		panic(err)
	}

	return no
}

func parse_unix(ts string) time.Time {
	i := strings.IndexRune(ts, '.')
	sec, err := strconv.ParseInt(ts[:i], 10, 64)
	if err != nil {
		panic(err)
	}
	nsec, err := strconv.ParseInt(ts[i+1:], 10, 64)
	if err != nil {
		panic(err)
	}

	tm := time.Unix(sec, nsec)
	return tm
}

func make_unix(t time.Time) string {
	ts := float64(t.UnixNano() / int64(time.Second))
	return strconv.FormatFloat(ts, 'f', 6, 64)
}

func (user User) TrashNote(n *Note) Note {
	n.Deleted = 1
	return user.UpdateNote(n)
}

func (user User) DeleteNote(n *Note) (bool, error) {
	tn := user.TrashNote(n)

	if tn.Deleted != 1 {
		return false, errors.New(fmt.Sprintf("Note not set as deleted: %d", tn.Deleted))
	}

	path := fmt.Sprintf("api2/data/%s", n.Key)

	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: path}
	v := url.Values{}
	v.Add("email", user.Email)
	v.Add("auth", user.Auth)
	u.RawQuery = v.Encode()

	req, err := http.NewRequest("DELETE", u.String(), nil)

	client := http.Client{Timeout: time.Second * 10}
	r, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		panic(r.Status)
	}

	return true, nil
}

/* Params
 * path - Path to where notes are stored.
 * fu - force update of note */
func (n Note) WriteNoteFs(path string, fu bool) error {
	new_file := false

	if err := os.Chdir(path); err != nil {
		panic(err)
	}

	if _, err := os.Stat(n.Key); os.IsNotExist(err) {
		err := os.Mkdir(n.Key, 0777)
		if err != nil {
			panic(err)
		}

		new_file = true
	}

	if err := os.Chdir(n.Key); err != nil {
		panic(err)
	}

	f, oe := os.OpenFile(KEY, os.O_RDWR|os.O_CREATE, 0755)
	if oe != nil {
		panic(oe)
	}

	if _, err := f.WriteString(n.Key); err != nil {
		panic(err)
	}
	f.Close()

	f, err := os.OpenFile(CONTENT, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err) // change
	}
	defer f.Close()

	fs, ferr := f.Stat()
	if ferr != nil {
		panic(ferr)
	}

	mt := parse_unix(n.Modifydate)
	if !new_file && fs.ModTime().After(mt) && !fu {
		return errors.New("Filesystem note newer than current note.")
	}

	if _, err := f.WriteString(n.Content); err != nil {
		panic(err)
	}
	f.Close()

	if terr := os.Chtimes(CONTENT, time.Now(), mt); terr != nil {
		panic(terr)
	}

	return nil
}

func ReadNoteFs(path string, key string) (Note, error) {
	note := Note{Key: key}

	if err := os.Chdir(path); err != nil {
		panic(err)
	}
	if err := os.Chdir(key); err != nil {
		panic(err)
	}

	f, err := os.Open(CONTENT)
	if err != nil {
		return note, err
	}
	defer f.Close()

	cont, err := ioutil.ReadAll(f)
	if err != nil {
		return note, err
	}
	note.Content = string(cont)

	stat, err := f.Stat()
	if err != nil {
		return note, err
	}

	note.Modifydate = stat.ModTime().String()

	return note, nil
}

func (ns Index) WriteNotes(path string, overwrite bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		e := os.Mkdir(path, 0777)
		if e != nil {
			panic(e)
		}
	} else {
		if !overwrite {
			return errors.New("Directory exists")
		}
	}

	for _, v := range ns.Data {
		if err := v.WriteNoteFs(path, overwrite); err != nil {
			panic(err)
		}
	}

	return nil
}

func visit(path string, f os.FileInfo, err error) error {
	if !f.IsDir() {
		return filepath.SkipDir
	}

	fmt.Printf("Visited: %s\n", path)

	if err := os.Chdir(path); err != nil {
		panic(err)
	}
	defer os.Chdir("..")

	b, err := ioutil.ReadFile(KEY)
	if err != nil {
		fmt.Printf("Could not open Key in directory %s\n", path)
		return nil
		//return filepath.SkipDir
		//return err
	}

	fmt.Println(string(b))

	return nil
}

func ReadNotes(path string) {

	filepath.Walk(path, visit)
}
