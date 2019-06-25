package auth

import (
	"sync"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

var (
	authenticator C.Authenticator
)

func Authenticator() C.Authenticator {
	return authenticator
}

func SetAuthenticator(au C.Authenticator) {
	authenticator = au
}

type inMemoryAuthenticator struct {
	sync.Map
	logins []C.AuthUser
}

func (au inMemoryAuthenticator) Verify(user string, pass string) bool {
	if realPass, ok := au.Load(user); ok && realPass == pass {
		return true
	}
	return false
}

func (au inMemoryAuthenticator) Enabled() bool        { return true }
func (au inMemoryAuthenticator) Logins() []C.AuthUser { return au.logins }

type noAuthenticator struct{}

func (au noAuthenticator) Verify(user string, pass string) bool { return true }
func (au noAuthenticator) Enabled() bool                        { return false }
func (au noAuthenticator) Logins() []C.AuthUser                 { return []C.AuthUser{} }

func NewAuthenticator(users []C.AuthUser) C.Authenticator {
	if len(users) == 0 {
		return noAuthenticator{}
	}

	au := &inMemoryAuthenticator{}
	for _, user := range users {
		log.Infoln("Loaded user %s:%s", user.User, user.Pass)
		au.Store(user.User, user.Pass)
	}
	return au
}
