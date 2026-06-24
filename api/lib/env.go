package lib

import (
	"os"
	"strings"
)

type Environment int

const (
	Testing Environment = iota
	Development
	Staging
	Production
)

func Detect() Environment {
	env := os.Getenv("APP_ENV")

	switch strings.ToLower(env) {
	case "dev", "development":
		return Development
	case "stage", "staging":
		return Staging
	case "prod", "production":
		return Production
	default:
		return Testing
	}
}

func (e Environment) String() string {
	switch e {
	case Development:
		return "development"
	case Staging:
		return "staging"
	case Production:
		return "production"
	default:
		return "testing"
	}
}

func (e Environment) ToGinMode() string {
	switch e {
	case Development:
		return "debug"
	case Staging:
		return "debug"
	case Production:
		return "release"
	default:
		return "test"
	}
}
