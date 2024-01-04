package consts

import (
	"os"
	"path/filepath"
)

const (
	// SocketDir is the path to expose the socket consumed by KubeCIA.
	SocketDir = "/var/run"
)

// SocketPath returns the path to the socket consumed by KubeCIA.
func SocketPath() string {
	return filepath.Clean("/var/run/kubecia.sock")
}

// DataDir is the path to the data consumed by KubeCIA.
func DataDir() string {
	hd, err := os.UserHomeDir()
	if err != nil {
		hd = "/var/run/kubecia"
	}

	return filepath.Clean(filepath.Join(hd, ".kubecia"))
}
