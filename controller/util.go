package main

import (
	"log"
	"path/filepath"
)

func logPrintf(formato string, args ...interface{}) {
	log.Printf(formato, args...)
}

func absPath(caminho string) string {
	abs, err := filepath.Abs(caminho)
	if err != nil {
		return caminho
	}
	return abs
}
