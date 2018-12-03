package log

import (
	"encoding/json"
	"errors"

	yaml "gopkg.in/yaml.v2"
)

var (
	// LogLevelMapping is a mapping for LogLevel enum
	LogLevelMapping = map[string]LogLevel{
		"error":   ERROR,
		"warning": WARNING,
		"info":    INFO,
		"debug":   DEBUG,
		"silent":  SILENT,
	}
)

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	SILENT
)

type LogLevel int

// UnmarshalYAML unserialize Mode with yaml
func (l *LogLevel) UnmarshalYAML(data []byte) error {
	var tp string
	yaml.Unmarshal(data, &tp)
	level, exist := LogLevelMapping[tp]
	if !exist {
		return errors.New("invalid mode")
	}
	*l = level
	return nil
}

// MarshalYAML serialize Mode with yaml
func (l LogLevel) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(l.String())
}

// UnmarshalJSON unserialize Mode with json
func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var tp string
	json.Unmarshal(data, &tp)
	level, exist := LogLevelMapping[tp]
	if !exist {
		return errors.New("invalid mode")
	}
	*l = level
	return nil
}

// MarshalJSON serialize Mode with json
func (l LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l LogLevel) String() string {
	switch l {
	case INFO:
		return "info"
	case WARNING:
		return "warning"
	case ERROR:
		return "error"
	case DEBUG:
		return "debug"
	case SILENT:
		return "silent"
	default:
		return "unknown"
	}
}
