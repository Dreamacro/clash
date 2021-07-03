package provider

import (
	"bytes"
	"crypto/md5"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/Dreamacro/clash/log"
)

var (
	fileMode os.FileMode = 0666
	dirMode  os.FileMode = 0755
)

type parser = func([]byte) (interface{}, error)

type fetcher struct {
	name      string
	vehicle   Vehicle
	updatedAt *time.Time
	ticker    *time.Ticker
	done      chan struct{}
	hash      [16]byte
	parser    parser
	onUpdate  func(interface{})
}

func (f *fetcher) Name() string {
	return f.name
}

func (f *fetcher) VehicleType() VehicleType {
	return f.vehicle.Type()
}

func (f *fetcher) Initial() (interface{}, error) {
	var (
		buf     []byte
		err     error
		isLocal bool
	)
	if stat, fErr := os.Stat(f.vehicle.Path()); fErr == nil {
		buf, err = ioutil.ReadFile(f.vehicle.Path())
		modTime := stat.ModTime()
		f.updatedAt = &modTime
		isLocal = true
	} else {
		buf, err = f.vehicle.Read()
	}

	if err != nil {
		return nil, err
	}

	proxies, err := f.parser(buf)
	if err != nil {
		if !isLocal {
			return nil, err
		}

		// parse local file error, fallback to remote
		buf, err = f.vehicle.Read()
		if err != nil {
			return nil, err
		}

		proxies, err = f.parser(buf)
		if err != nil {
			return nil, err
		}

		isLocal = false
	}

	if f.vehicle.Type() != File && !isLocal {
		if err := safeWrite(f.vehicle.Path(), buf); err != nil {
			return nil, err
		}
	}

	f.hash = md5.Sum(buf)

	// pull proxies automatically
	if f.ticker != nil {
		go f.pullLoop()
	}

	return proxies, nil
}

func (f *fetcher) Update() (interface{}, bool, error) {
	buf, err := f.vehicle.Read()
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	hash := md5.Sum(buf)
	if bytes.Equal(f.hash[:], hash[:]) {
		f.updatedAt = &now
		return nil, true, nil
	}

	proxies, err := f.parser(buf)
	if err != nil {
		return nil, false, err
	}

	if f.vehicle.Type() != File {
		if err := safeWrite(f.vehicle.Path(), buf); err != nil {
			return nil, false, err
		}
	}

	f.updatedAt = &now
	f.hash = hash

	return proxies, false, nil
}

func (f *fetcher) Destroy() error {
	if f.ticker != nil {
		f.done <- struct{}{}
	}
	return nil
}

func (f *fetcher) pullLoop() {
	for {
		select {
		case <-f.ticker.C:
			elm, same, err := f.Update()
			if err != nil {
				log.Warnln("[Provider] %s pull error: %s", f.Name(), err.Error())
				continue
			}

			if same {
				log.Debugln("[Provider] %s's proxies doesn't change", f.Name())
				continue
			}

			log.Infoln("[Provider] %s's proxies update", f.Name())
			if f.onUpdate != nil {
				f.onUpdate(elm)
			}
		case <-f.done:
			f.ticker.Stop()
			return
		}
	}
}

func safeWrite(path string, buf []byte) error {
	dir := filepath.Dir(path)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return err
		}
	}

	return ioutil.WriteFile(path, buf, fileMode)
}

func newFetcher(name string, interval time.Duration, vehicle Vehicle, parser parser, onUpdate func(interface{})) *fetcher {
	var ticker *time.Ticker
	if interval != 0 {
		ticker = time.NewTicker(interval)
	}

	return &fetcher{
		name:     name,
		ticker:   ticker,
		vehicle:  vehicle,
		parser:   parser,
		done:     make(chan struct{}, 1),
		onUpdate: onUpdate,
	}
}
