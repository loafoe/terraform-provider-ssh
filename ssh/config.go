package ssh

import (
	"fmt"
	"os"
)

type SSHConnectionDefaults struct {
	Host              string
	Port              string
	User              string
	Password          string
	PrivateKey        string
	BastionHost       string
	BastionPort       string
	BastionUser       string
	BastionPassword   string
	BastionPrivateKey string
	Agent             bool
}

type Config struct {
	DebugLog   string
	debugFile  *os.File
	Connection SSHConnectionDefaults
}

func (c *Config) Debug(format string, args ...interface{}) (int, error) {
	if c.debugFile != nil {
		output := fmt.Sprintf(format, args...)
		return c.debugFile.WriteString(output)
	}
	return fmt.Printf(format, args...)
}
