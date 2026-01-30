package db

// Bare user functions
func GetUser(userName string) (*User, error) {
	panic("not implemented")
}

func CreateUser(user User) (*User, error) {
	panic("not implemented")
}

func UpdateUser(user User) (*User, error) {
	panic("not implemented")
}

func DeleteUser(user User) error {
	panic("not implemented")
}

// User Auth functions
func AddUserCredential(user User, auth UserCredential) error {
	panic("not implemented")
}

func RemoveUserCredential(user User, auth UserCredential) error {
	panic("not implemented")
}
