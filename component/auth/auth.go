package auth

import (
	"sync"
)

type Authenticator interface {
	Verify(user string, pass string) bool
	Users() []AuthUser
}

type AuthUser struct {
	User string
	Pass string
}

type inMemoryAuthenticator struct {
	storage sync.Map
	logins  []AuthUser
}

func (au *inMemoryAuthenticator) Verify(user string, pass string) bool {
	if realPass, ok := au.storage.Load(user); ok && realPass == pass {
		return true
	}
	return false
}

func (au *inMemoryAuthenticator) Users() []AuthUser { return au.logins }

func NewAuthenticator(users []AuthUser) Authenticator {
	if len(users) == 0 {
		return nil
	}

	au := &inMemoryAuthenticator{logins: users}
	for _, user := range users {
		au.storage.Store(user.User, user.Pass)
	}
	return au
}
