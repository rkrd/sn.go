package sn

import (
	"fmt"
	"io/ioutil"
	"path"
	//"github/sn.go"
)

func Test_all(email string, pass string) bool {
	u, status := test1_get_auth(email, pass)
	fmt.Println(u, status)
	if !status {
		fmt.Println("test1_get_auth FAIL")
		return false
	}

	n, status := test2_create_note(u)
	if !status {
		fmt.Println("test2_create_note FAIL")
		return false
	}

	status = test3_get_note_list(u, n.Key)
	if !status {
		fmt.Println("test3_get_note_list FAIL")
		return false
	}

	n, status = test4_get_note(u, n.Key)
	if !status {
		fmt.Println("test4_get_note FAIL")
		return false
	}

	_, status = test5_update_note(u, n)
	if !status {
		fmt.Println("test5_update_note FAIL")
		return false
	}

	status = test8_read_write_note_fs(n)
	if !status {
		fmt.Println("test8_read_write_note_fs FAIL")
		return false
	}

	n, status = test6_trash_note(u, n)
	if !status {
		fmt.Println("test6_trash_note FAIL")
		return false
	}

	status = test7_delete_note(u, n)
	if !status {
		fmt.Println("test7_delete_note FAIL")
		return false
	}

	note_list, status := test9_write_notes_fs(u)
	if !status {
		fmt.Println("test9_write_notes_fs FAIL")
		return false
	}

	status = test10_sync_notes(u, note_list)
	if !status {
		fmt.Println("test10_sync_notes FAIL")
		return false
	}

	return true
}

/* Test if authentication works.
 * Should return User struct with email same as inparameter email and nonempty auth
 */
func test1_get_auth(email string, pass string) (User, bool) {
	u, err := GetAuth(email, pass)
	if err != nil {
		return User{}, false
	}

	return u, (u.Email == email && u.Auth != "")
}

/* Test auth string and note creation
 * Create a note with text and tag and check Key and Tags is nonempty
 */
func test2_create_note(u User) (Note, bool) {
	var n Note
	n.Content = "Test string"
	n.Tags = []string{"Test_tag"}

	nn := u.UpdateNote(&n)

	return nn, nn.Key != "" || nn.Tags[0] == "Test_tag"
}

/* Test fetching of notes list.
 * Check that list contians a known key and lenght is not zero
 */
func test3_get_note_list(u User, key string) bool {
	nl, err := u.GetAllNotes()

	if err != nil {
		fmt.Println(err)
		return false
	}

	found := false
	for _, v := range nl.Data {
		if v.Key == key {
			found = true
			break
		}
	}

	return found && nl.Count != 0
}

func test4_get_note(u User, key string) (Note, bool) {
	n, err := u.GetNote(key, 0)

	if err != nil {
		fmt.Println(err)
		return n, false
	}

	return n, n.Content == "Test string"
}

func test5_update_note(u User, n Note) (Note, bool) {
	n.Content = "New test string"
	nn := u.UpdateNote(&n)
	nn, _ = u.GetNote(nn.Key, 0)

	return nn, nn.Content == "New test string"
}

func test6_trash_note(u User, n Note) (Note, bool) {
	tn := u.TrashNote(&n)

	return tn, tn.Deleted == 1
}

func test7_delete_note(u User, n Note) bool {
	ret, err := u.DeleteNote(&n)
	if err != nil {
		fmt.Println(err)
		return false
	}

	return ret
	// Add check that note list does not contain note?
}

/* Write a note to filesystem, read the note from filesystem and check that the contet is the same
 * of both notes. */
func test8_read_write_note_fs(n Note) bool {
	n.WriteNoteFs("/tmp", false)
	nn, err := ReadNoteFs("/tmp", n.Key)

	if err != nil {
		fmt.Println(err)
		return false
	}

	return n.Content == nn.Content
}

/* Fetch all notes and write to filesystem */
func test9_write_notes_fs(u User) (Index, bool) {
	nl, err := u.GetAllNotes()
	if err != nil {
		fmt.Println(err)
		return Index{}, false
	}

	for i, v := range nl.Data {
		n, err := u.GetNote(v.Key, 0)
		if err != nil {
			fmt.Println(err)
			return Index{}, false
		}

		nl.Data[i] = n
	}

	err = nl.WriteNotes("/tmp/notes", true)
	if err != nil {
		fmt.Println(err)
		return Index{}, false
	}

	return nl, true
}

func test10_sync_notes(u User, note_list Index) bool {
	mod_note, err := ReadNoteFs("/tmp/notes", note_list.Data[0].Key)
	if err != nil {
		fmt.Println("---- fail 1 ----")
		fmt.Println(err)
		return false
	}

	err = ioutil.WriteFile(path.Join("/tmp/notes", mod_note.Key, "text.txt"), []byte("apa bepa cepa"), 0644)
	if err != nil {
		fmt.Println("---- fail 2 ----")
		fmt.Println(err)
		return false
	}

	SyncNotes("/tmp/notes", u, true)

	fetch_note, err := u.GetNote(mod_note.Key, 0)
	if err != nil {
		fmt.Println("---- fail 3 ----")
		fmt.Println(err)
		return false
	}

	fmt.Println("test10 summary:")
	mod_note.PrintNote()
	fetch_note.PrintNote()
	fmt.Println("====================")
	return "apa bepa cepa" == fetch_note.Content

	/* Test when both server and local note have changed and server note shall overwrite.
	 * Test when server and local have same content but dates changed.
	 */
}
