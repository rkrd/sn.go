package sn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type User struct {
	Email string
	Auth  string
}

type Note struct {
	Modifydate string   `json:"modifydate"`
	Tags       []string `json:"tags"`
	Deleted    int      `json:"deleted"`
	CreateDate string   `json:"createdate"`
	Systemtags []string `json:"systemtags"`
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

func getNotes(user User, mark string) ([]Note, string, error) {
	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)
	v.Add("mark", mark)

	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: "api2/index"}
	u.RawQuery = v.Encode()
	s, err := http.Get(u.String())
	defer s.Body.Close()
	if err != nil {
		return nil, "", err
	}

	d := json.NewDecoder(s.Body)

	var n Index
	err = d.Decode(&n)

	return n.Data, n.Mark, nil
}

func GetAllNotes(user User) []Note {
	var mark string
	var notes []Note

	for {
		d, m, err := getNotes(user, mark)
		if err != nil {
			panic(err)
		}

		notes = append(notes, d...)

		if m == "" {
			break
		}
	}

	return notes
}

func GetNote(user User, key string, version uint) Note {
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
		//return nil, "", err
		panic(err)
	}

	d := json.NewDecoder(r.Body)
	var n Note
	err = d.Decode(&n)

	if err != nil {
		panic(err)
	}

	return n
}

func UpdateNote(user User, key string, note *Note) Note {
	v := url.Values{}
	v.Add("auth", user.Auth)
	v.Add("email", user.Email)

	var path string
	if key != "" {
		path = fmt.Sprintf("api2/data/%s", key)
	} else {
		path = "api2/data"
	}
	u := url.URL{Scheme: "https", Host: "simple-note.appspot.com", Path: path}
	u.RawQuery = v.Encode()

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(note)

	r, err := http.Post(u.String(), "application/json; charset=utf-8", b)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		panic(r.Status)
	}

	d := json.NewDecoder(r.Body)
	var n Note
	err = d.Decode(&n)

	if err != nil {
		panic(err)
	}

	return n
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

