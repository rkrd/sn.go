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
	"strconv"
	"strings"
	"time"
)

const SIMPLENOTE_URL = "simple-note.appspot.com"

var Verbose = 0

type User struct {
	Email string
	Auth  string
}

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
	modtime    time.Time
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
	fmt.Println("Key", n.Key)
}

func GetAuth(email string, pass string) (User, error) {
	var uri url.URL
	uri.Scheme = "https"
	uri.Host = SIMPLENOTE_URL
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

	u := url.URL{Scheme: "https", Host: SIMPLENOTE_URL, Path: "api2/index"}
	u.RawQuery = v.Encode()
	s, err := http.Get(u.String())
	defer s.Body.Close()

	if err != nil {
		panic(err)
	}

	if s.StatusCode != 200 {
		return i, errors.New(fmt.Sprintf("getNotes returned: %d", s.StatusCode))
	}

	d := json.NewDecoder(s.Body)
	err = d.Decode(&i)

	return i, err
}

func (user User) GetAllNotes() (Index, error) {
	var i Index
	vprint("GetAllNotes ")

	for {
		vprint(".")
		ii, err := getNotes(user, i.Mark)

		if err != nil {
			return i, err
		}

		i.Mark = ii.Mark
		i.Count += ii.Count
		i.Time = ii.Time
		i.Data = append(i.Data, ii.Data...)

		if ii.Mark == "" {
			break
		}
	}
	vprint("\n")

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
	u := url.URL{Scheme: "https", Host: SIMPLENOTE_URL, Path: path}
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

	return no, err
}

/* Used to update an existing note or create a new note if Key of Note is
 * not set. */
func (user User) UpdateNote(n *Note) (Note, error) {
	var no Note
	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)

	var path string
	if n.Key != "" {
		path = fmt.Sprintf("api2/data/%s", n.Key)
	} else {
		path = "api2/data"
	}
	u := url.URL{Scheme: "https", Host: SIMPLENOTE_URL, Path: path}
	u.RawQuery = v.Encode()

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(n)

	r, err := http.Post(u.String(), "application/json; charset=utf-8", b)
	if err != nil {
		return no, err
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		return no, errors.New(fmt.Sprintf("UpdateNote returned: %d on URL: %s", r.StatusCode, u.String()))
	}

	d := json.NewDecoder(r.Body)
	err = d.Decode(&no)

	return no, err
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

func (user User) TrashNote(n *Note) (Note, error) {
	n.Deleted = 1
	return user.UpdateNote(n)
}

func (user User) DeleteNote(n *Note) (bool, error) {
	tn, err := user.TrashNote(n)

	if err != nil {
		return false, err
	}
	if tn.Deleted != 1 {
		return false, errors.New(fmt.Sprintf("Note not set as deleted: %d", tn.Deleted))
	}

	path := fmt.Sprintf("api2/data/%s", n.Key)

	u := url.URL{Scheme: "https", Host: SIMPLENOTE_URL, Path: path}
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

func SetVerbose(level int) {
	Verbose = level
}

func vprint(s string) {
	if Verbose > 0 {
		fmt.Print(s)
	}
}
