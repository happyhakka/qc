package version

import (
	"fmt"
)

const (
	MajorVersion  = "1"
	MinorVersion  = "0"
	ReviseVersion = "4"
	BuildVersion  = "0108"
)

var (
	BuildTime = "None"
	GitHash   = "None"
	GoVersion = "None"
)

func GetVersion() string {
	return fmt.Sprintf("%s.%s.%s.%s", MajorVersion, MinorVersion, ReviseVersion, BuildVersion)
}
