package auth

import (
	"github.com/Dreamacro/clash/log"
	"io/ioutil"
	"strings"
	"sync"
)

type User struct {
	User string
	Pass string
}

type AuthLoader interface {
	LoadAll() ([]User, error)
}

type FileAuthLoader struct {
	fn string
}

func (fal *FileAuthLoader) LoadAll() ([]User, error) {
	data, err := ioutil.ReadFile(fal.fn)
	if err != nil {
		return nil, err
	}

	users := make([]User, 0)
	for _, line := range strings.Split(string(data), "\n") {
		userData := strings.SplitN(line, ":", 2)
		// Make sure that this line has username and password separated by :
		if len(userData) == 2 {
			users = append(users, User{User: userData[0], Pass: userData[1]})
		}
	}
	return users, nil
}

func NewAuthLoader(url string) AuthLoader {
	return &FileAuthLoader{fn: url}
}

type Authenticator interface {
	Verify(user string, pass string) bool
	Enabled() bool
}

type inMemoryAuthenticator struct {
	sync.Map
}

func (au inMemoryAuthenticator) Verify(user string, pass string) bool {
	if realPass, ok := au.Load(user); ok && realPass == pass {
		return true
	}
	return false
}

func (au inMemoryAuthenticator) Enabled() bool { return true }

type noAuthenticator struct{}

func (au noAuthenticator) Verify(user string, pass string) bool { return true }
func (au noAuthenticator) Enabled() bool                        { return false }

func NewAuthenticator(al AuthLoader) Authenticator {
	users, err := al.LoadAll()
	if err != nil || len(users) == 0 {
		return noAuthenticator{}
	}

	au := &inMemoryAuthenticator{}
	for _, user := range users {
		log.Infoln("Loaded user %s:%s", user.User, user.Pass)
		au.Store(user.User, user.Pass)
	}
	return au
}
