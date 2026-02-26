package model

// User represents a user in the system.
type User struct {
	ID    int
	Name  string
	Email string
}

// NewUser creates a new User.
func NewUser(name, email string) *User {
	return &User{Name: name, Email: email}
}

// Validate checks if the user data is valid.
func (u *User) Validate() error {
	return nil
}
