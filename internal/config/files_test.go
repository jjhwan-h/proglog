package config

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigFile(t *testing.T) {
	file := "test"
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	filePath := configFile(file)
	require.Equal(t, filepath.Join(homeDir, "Desktop/Go/proglog/.proglog", "test"), filePath)
	log.Println(filePath)
}
