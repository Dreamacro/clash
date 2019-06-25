package constant

type Authenticator interface {
	Verify(user string, pass string) bool
	Enabled() bool
	Logins() []AuthUser
}

type AuthUser struct {
	User string
	Pass string
}