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
	"strconv"
	"strings"
	"time"
	// "path/filepath"
)

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

	fmt.Println("debug")
	fmt.Println(no)
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

/* Params fu - force update of note */
func (n Note) update_note_fs(fu bool) error {
	new_file := false
	if _, err := os.Stat("text.txt"); os.IsNotExist(err) {
		new_file = true
		f, oe := os.OpenFile("Key", os.O_RDWR|os.O_CREATE, 0755)
		if oe != nil {
			panic(oe)
		}

		if _, err := f.WriteString(n.Key); err != nil {
			panic(err)
		}
		f.Close()

	}

	f, err := os.OpenFile("text.txt", os.O_RDWR|os.O_CREATE, 0755)
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

	if terr := os.Chtimes("text.txt", time.Now(), mt); terr != nil {
		panic(terr)
	}

	return nil
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

	if err := os.Chdir(path); err != nil {
		panic(err)
	}

	for _, v := range ns.Data {
		if _, err := os.Stat(v.Key); os.IsNotExist(err) {
			err := os.Mkdir(v.Key, 0777)
			if err != nil {
				panic(err)
			}
		} else if !overwrite {
			continue
		}

		if err := os.Chdir(v.Key); err != nil {
			panic(err)
		}

		if err := v.update_note_fs(overwrite); err != nil {
			panic(err)
		}

		os.Chdir(path)
	}

	return nil
}
