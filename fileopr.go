package sn

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"time"
	"bufio"
	"path/filepath"
)

const CONTENT string = "text.txt"
const KEY string = ".Key"
const TAGS string = "Tags"
/* MODIFYDATE is used to save the date note were last modified
 on server. Used to determine if note have been modified on
 both server and localy. */
const MODIFYDATE = ".Modifydate"
const FPERM os.FileMode = 0600
const DPERM os.FileMode = 0700

func trunc_write(fn string, s string) error {
	f, oerr := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, FPERM)
	if oerr != nil {
		return oerr
	}
	defer f.Close()

	_, err := f.WriteString(s)

	return err
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

	if err := trunc_write(KEY, n.Key); err != nil {
		return err
	}

	if err := trunc_write(MODIFYDATE, n.Modifydate); err != nil {
		return err
	}

	// Content

	fs, ferr := os.Stat(CONTENT)
	if ferr != nil {
		new_file = true
	}

	mt := parse_unix(n.Modifydate)
	if !new_file && fs.ModTime().After(mt) && !fu {
		return errors.New("Filesystem note newer than current note.")
	}

	if err := trunc_write(CONTENT, n.Content); err != nil {
		return err
	}

	if err := os.Chtimes(CONTENT, time.Now(), mt); err != nil {
		return err
	}

	// Tags
	f, ferr := os.OpenFile(TAGS, os.O_RDWR|os.O_CREATE, FPERM)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, v := range n.Tags {
		w.WriteString(v)
		f.WriteString("\n")
	}
	return w.Flush()
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

	/*
	for _, d := range note_dirs {
		u.SyncNote(path, d.Name(), prio_fs)
	}
	*/

	var fidx Index
	for _, d := range note_dirs {
		n, err := ReadNoteFs(path, d.Name())
		if err != nil {
			fmt.Println(err)
			return
		}
		fidx.Data = append(fidx.Data, n)
	}

	sidx, err := u.GetAllNotes()
	if err != nil {
		fmt.Println("SyncNotes",err)
		return
	}

	for _, sn := range sidx.Data {
		found := -1

		for i, fn := range fidx.Data {
			if fn.Key == sn.Key {
				found = i
				break
			}
		}

		if found != -1 {
			fn := fidx.Data[found]
			sn_time := parse_unix(sn.Modifydate)

			if sn.Deleted != 0 {
				fmt.Println("Deleting trashed note ", sn.Key)
				os.RemoveAll(filepath.Join(path, sn.Key))
				continue
			}
			if fn.modtime != sn_time {
				fmt.Println("Syncing note: ", fn.Key)
				u.SyncNote(path, fn.Key, prio_fs)
			}
		} else {
			if sn.Deleted != 0 {
				continue
			}
			fmt.Println("Fetching new note ", sn.Key)
			n, err := u.GetNote(sn.Key, 0)
			if err != nil {
				fmt.Println(err)
				return
			}
			n.WriteNoteFs(path, false)
		}
	}
}
