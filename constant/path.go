package constant

import (
	"os"
	P "path"
)

const Name = "clash"

// Path is used to get the configuration path
var Path *path

type path struct {
	homeDir    string
	configFile string
}

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir, _ = os.Getwd()
	}

	homeDir = P.Join(homeDir, ".config", Name)
	Path = &path{homeDir: homeDir, configFile: "config.yaml"}
}

// SetHomeDir is used to set the configuration path
func SetHomeDir(root string) {
	Path.homeDir = root
}

// SetConfig is used to set the configuration file
func SetConfig(file string) {
	Path.configFile = file
}

func (p *path) HomeDir() string {
	return p.homeDir
}

func (p *path) Config() string {
	return p.configFile
}

func (p *path) MMDB() string {
	return P.Join(p.homeDir, "Country.mmdb")
}
