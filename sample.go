// user_processor.go
package main

import (
	"fmt"
	"strings"
)

type User struct {
	ID    int
	Name  string
	Email string
}

func validateUser(u User) error {
	if !strings.Contains(u.Email, "@") {
		return fmt.Errorf("invalid email for user %d: %s", u.ID, u.Email)
	}
	return nil
}

func main() {
	fmt.Println("Starting user processing...")

	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob-invalid-email"}, // Invalid email
		{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
		{ID: 4, Name: "David", Email: "david@example.com"},
		{ID: 5, Name: "Eve", Email: "eve-another-bad-email"}, // Invalid email
	}

	var validUsers []User
	var invalidUsers []User

	for _, user := range users {
		err := validateUser(user)

		if err != nil {
			invalidUsers = append(invalidUsers, user)
		} else {
			validUsers = append(validUsers, user)
		}
	}

	fmt.Println("\n--- Processing Complete ---")

	fmt.Println("\nFound VALID users:")
	for _, user := range validUsers {
		fmt.Printf("- ID: %d, Name: %s, Email: %s\n", user.ID, user.Name, user.Email)
	}

	fmt.Println("\nFound INVALID users:")
	for _, user := range invalidUsers {
		fmt.Printf("- ID: %d, Name: %s, Email: %s\n", user.ID, user.Name, user.Email)
	}
}
