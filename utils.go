package main

import (
	"bufio"
	"os"
)

func linesInFile(fileName string) (i int64) {
	f, _ := os.Open(fileName)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		i++
	}
	return i
}
