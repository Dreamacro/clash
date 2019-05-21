package constant

import (
	"os"
	"os/user"
	P "path"
)

const Name = "clash"

// Path is used to get the configuration path
var Path *path

type path struct {
	homedir  string
	confname string
}

func init() {
	currentUser, err := user.Current()
	var homedir string
	if err != nil {
		dir := os.Getenv("HOME")
		if dir == "" {
			dir, _ = os.Getwd()
		}
		homedir = dir
	} else {
		homedir = currentUser.HomeDir
	}

	homedir = P.Join(homedir, ".config", Name)
	Path = &path{homedir: homedir}
}

// SetHomeDir is used to set the configuration path
func SetHomeDir(root string) {
	Path = &path{
		homedir:  root,
		confname: "config.yml",
	}
}

func SetHomeDirAndConfName(root string, name string) {
	Path = &path{
		homedir:  root,
		confname: name,
	}
}

func (p *path) HomeDir() string {
	return p.homedir
}

func (p *path) Config() string {
	return P.Join(p.homedir, p.confname)
}

func (p *path) MMDB() string {
	return P.Join(p.homedir, "Country.mmdb")
}
