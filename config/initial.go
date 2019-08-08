package config

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	C "github.com/Dreamacro/clash/constant"

	log "github.com/sirupsen/logrus"
)

func downloadMMDB(path string) (err error) {
	resp, err := http.Get("http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if !strings.HasSuffix(h.Name, "GeoLite2-Country.mmdb") {
			continue
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, tr)
		if err != nil {
			return err
		}
	}

	return nil
}

// Init prepare necessary files
func Init(dir string) error {
	// initial homedir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return fmt.Errorf("Can't create config directory %s: %s", dir, err.Error())
		}
	}

	// initial config.yaml
	if _, err := os.Stat(C.Path.Config()); os.IsNotExist(err) {
		log.Info("Can't find config, create an empty file")
		os.OpenFile(C.Path.Config(), os.O_CREATE|os.O_WRONLY, 0644)
	}

	// initial mmdb
	if _, err := os.Stat(C.Path.MMDB()); os.IsNotExist(err) {
		log.Info("Can't find MMDB, start download")
		err := downloadMMDB(C.Path.MMDB())
		if err != nil {
			return fmt.Errorf("Can't download MMDB: %s", err.Error())
		}
	}
	return nil
}
