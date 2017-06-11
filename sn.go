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
	"reflect"
	"strconv"
	"strings"
	"time"
)

const SIMPLENOTE_URL = "simple-note.appspot.com"
const CONTENT string = "text.txt"
const KEY string = ".Key"
const TAGS string = "Tags"
/* MODIFYDATE is used to save the date note were last modified
 on server. Used to determine if note have been modified on
 both server and localy. */
const MODIFYDATE = ".Modifydate"
const FPERM os.FileMode = 0600
const DPERM os.FileMode = 0700

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

	for {
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

/* Params
 * path - Path to where notes are stored.
 * fu - force update of note */
func (n Note) WriteNoteFs(path string, fu bool) error {
	new_file := false

	if err := os.Chdir(path); err != nil {
		panic(err)
	}

	if _, err := os.Stat(n.Key); os.IsNotExist(err) {
		err := os.Mkdir(n.Key, DPERM)
		if err != nil {
			panic(err)
		}

		new_file = true
	}

	if err := os.Chdir(n.Key); err != nil {
		panic(err)
	}

	// Key
	f, oe := os.OpenFile(KEY, os.O_RDWR|os.O_CREATE, FPERM)
	if oe != nil {
		panic(oe)
	}

	if _, err := f.WriteString(n.Key); err != nil {
		panic(err)
	}
	f.Close()

	f, oe = os.OpenFile(MODIFYDATE, os.O_RDWR|os.O_CREATE, FPERM)
	if oe != nil {
		panic(oe)
	}

	if _, err := f.WriteString(n.Modifydate); err != nil {
		panic(err)
	}
	f.Close()

	// Content
	f, err := os.OpenFile(CONTENT, os.O_RDWR|os.O_CREATE, FPERM)
	if err != nil {
		panic(err) // change
	}

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

	// Tags
	f, err = os.OpenFile(TAGS, os.O_RDWR|os.O_CREATE, FPERM)
	if ferr != nil {
		panic(ferr)
	}

	for _, v := range n.Tags {
		if _, err := f.WriteString(v); err != nil {
			panic(err)
		}
		f.WriteString("\n")
	}
	f.Close()

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

	note.modtime = stat.ModTime()

	modifydate, err := ioutil.ReadFile(MODIFYDATE)
	if err != nil {
		return note, err
	}

	note.Modifydate = string(modifydate)

	return note, nil
}

func (ns Index) WriteNotes(path string, overwrite bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		e := os.Mkdir(path, DPERM)
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

func (u User) SyncNote(path string, key string, prio_fs bool) {
	ln, err := ReadNoteFs(path, key)
	if err != nil {
		panic(err)
	}

	sn, err := u.GetNote(key, 0)
	if err != nil {
		panic(err)
	}

	ln_time := parse_unix(ln.Modifydate)
	sn_time := parse_unix(sn.Modifydate)

	if ln.modtime.After(ln_time) {
		if sn_time.After(ln.modtime) && !prio_fs {
			err := sn.WriteNoteFs(path, true)
			if err != nil {
				panic(err)
			}
		} else {
			if sn.Content != ln.Content || reflect.DeepEqual(sn, ln) {
				n, err := u.UpdateNote(&ln)
				if n.Key != ln.Key || err != nil {
					panic(err)
				}
			}

			ln.Modifydate = make_unix(ln.modtime)
			ln.WriteNoteFs(path, true)
		}
	} else {
		if sn_time.After(ln_time) {
			err := sn.WriteNoteFs(path, true)
			if err != nil {
				panic(err)
			}
		}
	}
}

/* prio_fs - If true if both file modtime and server note Modifydate is newer than
 *           saved Modifydate over write note on server else over write on filesystem.
 */
func SyncNotes(path string, u User, prio_fs bool) {

	note_dirs, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("Failed to read dir", path, err)
		return
	}

	for _, d := range note_dirs {
		u.SyncNote(path, d.Name(), prio_fs)
	}

}
