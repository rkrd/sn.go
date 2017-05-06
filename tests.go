package sn

import (
	"fmt"
	//"github/sn.go"
)

func Test_all(email string, pass string) bool {
	u, status := test1_get_auth(email, pass)
	fmt.Println(u, status)
	if !status {
		return false
	}

	n, status := test2_create_note(u)
	n.PrintNote()
	if !status {
		return false
	}

	status = test3_get_note_list(u, n.Key)
	if !status {
		return false
	}

	n, status = test4_get_note(u, n.Key)
	n.PrintNote()
	if !status {
		return false
	}

	nn, status := test5_update_note(u, n)
	nn.PrintNote()
	if !status {
		return false
	}

	status = test8_read_write_note_fs(n)
	if !status {
		return false
	}

	n, status = test6_trash_note(u, n)
	n.PrintNote()
	if !status {
		return false
	}

	status = test7_delete_note(u, n)
	if !status {
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
		fmt.Println(v)
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

	fmt.Println(tn.Content, tn.Deleted)
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

func test8_read_write_note_fs(n Note) bool {
	n.WriteNoteFs("/tmp", false)
	nn, err := ReadNoteFs("/tmp", n.Key)

	if err != nil {
		fmt.Println(err)
		return false
	}

	fmt.Println("=================================== Test 8 ===================================")
	fmt.Println(n.Content, nn.Content)
	fmt.Println("=================================== Test 8 ===================================")

	return n.Content == nn.Content
}
