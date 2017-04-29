package tests

import (
	"fmt"
	"github/sn.go"
)

func test_all() bool {
	u, status := test1_get_auth("info@example.com", "foobar")
	fmt.Println(u, status)
	if !status {
		return false
	}

	n, status := test2_create_note(u)
	fmt.Println(n, status)
	if !status {
		return false
	}

	status = test3_get_note_list(u, n.Key)
	if !status {
		return false
	}

	return true
}

/* Test if authentication works.
 * Should return User struct with email same as inparameter email and nonempty auth
 */
func test1_get_auth(email string, pass string) (sn.User, bool) {
	u, err := sn.GetAuth(email, pass)
	if err != nil {
		return sn.User{}, false
	}

	return u, (u.Email == email && u.Auth != "")
}

/* Test auth string and note creation
 * Create a note with text and tag and check Key and Tags is nonempty
 */
func test2_create_note(u sn.User) (sn.Note, bool) {
	var n sn.Note
	n.Content = "Test string"
	n.Tags = []string{"Test_tag"}

	nn := u.UpdateNote(&n)

	return nn, nn.Key != "" || nn.Tags[0] == "Test_tag"
}

/* Test fetching of notes list.
 * Check that list contians a known key and lenght is not zero
 */
func test3_get_note_list(u sn.User, key string) bool {
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
