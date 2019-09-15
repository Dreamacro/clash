package constant

import (
	"os"
	P "path"
)

const Name = "clash"

// Path is used to get the configuration path
var Path *path

type path struct {
	homedir string
}

func init() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		homedir, _ = os.Getwd()
	}

	homedir = P.Join(homedir, ".config", Name)
	Path = &path{homedir: homedir}
}

// SetHomeDir is used to set the configuration path
func SetHomeDir(root string) {
	Path = &path{homedir: root}
}

func (p *path) HomeDir() string {
	return p.homedir
}

func (p *path) Config() string {
	return P.Join(p.homedir, "config.yaml")
}

func (p *path) MMDB() string {
	return P.Join(p.homedir, "Country.mmdb")
}
