package sn

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"
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

func write_truncated(fn string, s string) error {
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
 * force_update - force update of note */
func (n Note) WriteNoteFs(path string, force_update bool) error {
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

	if err := write_truncated(KEY, n.Key); err != nil {
		return err
	}

	if err := write_truncated(MODIFYDATE, n.Modifydate); err != nil {
		return err
	}

	// Content

	fs, ferr := os.Stat(CONTENT)
	if ferr != nil {
		new_file = true
	}

	mt := unix_timestamp_parse(n.Modifydate)
	if !new_file && fs.ModTime().After(mt) && !force_update {
		return errors.New("Filesystem note newer than current note.")
	}

	if err := write_truncated(CONTENT, n.Content); err != nil {
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

	ln_time := unix_timestamp_parse(ln.Modifydate)
	sn_time := unix_timestamp_parse(sn.Modifydate)

	if ln.modtime.After(ln_time) {
		if sn_time.After(ln.modtime) && !prio_fs {
			err := sn.WriteNoteFs(path, true)
			if err != nil {
				panic(err)
			}
		} else {
			ln.Modifydate = unix_timestamp_make(ln.modtime)
			if sn.Content != ln.Content || !reflect.DeepEqual(sn, ln) {
				n, err := u.UpdateNote(&ln)
				if n.Key != ln.Key || err != nil {
					panic(err)
				}
			}

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
 *           saved Modifydate overwrite note on server else over write on filesystem.
 */
func SyncNotes(path string, u User, prio_fs bool) {

	note_dirs, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println(err)
		panic("Failed to read dir " + path)
	}

	/*
		for _, d := range note_dirs {
			u.SyncNote(path, d.Name(), prio_fs)
		}
	*/

	map_local := make(map[string]Note)
	for _, d := range note_dirs {
		note, err := ReadNoteFs(path, d.Name())
		if err != nil {
			panic(err)
		}

		map_local[note.Key] = note
	}

	index_server, err := u.GetAllNotes()
	if err != nil {
		fmt.Println("SyncNotes", err)
		panic("SyncNotes failed GetAllNotes")
	}

	for _, note_server := range index_server.Data {
		note_local := map_local[note_server.Key]

		if note_local.Key != "" {
			ts_note_server := unix_timestamp_parse(note_server.Modifydate)

			if note_server.Deleted != 0 {
				fmt.Println("Deleting trashed note ", note_server.Key)
				os.RemoveAll(filepath.Join(path, note_server.Key))
				continue
			}
			if !note_local.modtime.Equal(ts_note_server) {
				fmt.Println("Syncing note: ", note_local.Key)
				u.SyncNote(path, note_local.Key, prio_fs)
			}
		} else {
			if note_server.Deleted != 0 {
				continue
			}
			fmt.Println("Fetching new note: ", note_server.Key)
			n, err := u.GetNote(note_server.Key, 0)
			if err != nil {
				fmt.Println(err)
				return
			}
			n.WriteNoteFs(path, false)
		}
	}
}
