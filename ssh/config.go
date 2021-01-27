package ssh

import (
	"fmt"
)

type Config struct {
}

func (c *Config) Debug(format string, args ...interface{}) (int, error) {
	return fmt.Printf(format, args...)
}
