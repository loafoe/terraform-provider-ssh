package ssh

import (
	"fmt"
	"os"
)

type Config struct {
	DebugLog  string
	debugFile *os.File
}

func (c *Config) Debug(format string, args ...interface{}) (int, error) {
	if c.debugFile != nil {
		output := fmt.Sprintf(format, args...)
		return c.debugFile.WriteString(output)
	}
	return fmt.Printf(format, args...)
}
