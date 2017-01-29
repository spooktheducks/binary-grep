package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var carveLen = flag.Uint("carve-len", 0, "number of bytes to carve for each result")
var carveExt = flag.String("carve-ext", "dat", "extension of carved output files")

func main() {
	flag.Usage = usage
	flag.Parse()

	pattern, filenames := getArgs()

	bytePattern, err := hex.DecodeString(pattern)
	if err != nil {
		panic(err)
	}

	procLimiter := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		procLimiter <- struct{}{}
	}

	chResults := make(chan result)
	chDones := []chan struct{}{}
	for _, filename := range filenames {
		chDone := make(chan struct{})
		chDones = append(chDones, chDone)
		go find(filename, bytePattern, procLimiter, chResults, chDone)
	}

	chResultsDone := make(chan struct{})
	go func() {
		defer close(chResultsDone)
		for x := range chResults {
			os.Stdout.WriteString(fmt.Sprintf("%s:%d\n", x.filename, x.offset))
			if *carveLen > 0 {
				carve(x)
			}
		}
	}()

	for i := range chDones {
		<-chDones[i]
	}

	close(chResults)
	<-chResultsDone
}

type result struct {
	filename string
	offset   int
	fileData []byte
}

func find(filename string, bytePattern []byte, procLimiter chan struct{}, chResults chan result, chDone chan struct{}) {
	<-procLimiter
	defer func() {
		procLimiter <- struct{}{}
		close(chDone)
	}()

	_, err := os.Stderr.WriteString(fmt.Sprintf("searching %s...\n", filename))
	if err != nil {
		panic(err)
	}

	basename := path.Base(filename)

	fileData, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	idx := 0
	for searchStart := 0; searchStart < len(fileData)-len(bytePattern); searchStart += idx + len(bytePattern) {
		idx = bytes.Index(fileData[searchStart:], bytePattern)
		if idx == -1 {
			break
		}

		chResults <- result{filename: basename, offset: idx + searchStart, fileData: fileData}
	}
}

func carve(r result) {
	filename := fmt.Sprintf("%s-%d.%s", r.filename, r.offset, *carveExt)
	err := ioutil.WriteFile(filename, r.fileData[r.offset:r.offset+int(*carveLen)], 0666)
	if err != nil {
		panic(err)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("    $ binary-grep <hex pattern> <file glob>")
	fmt.Println("")
	fmt.Println("Arguments:")
	flag.PrintDefaults()
}

func getArgs() (string, []string) {
	args := flag.Args()

	if len(args) < 2 {
		usage()
		os.Exit(1)
	}

	pattern := args[0]

	glob := args[1]
	homeDir := os.Getenv("HOME")
	glob = strings.Replace(glob, "~", homeDir, 1)

	filenames, err := filepath.Glob(glob)
	if err != nil {
		panic(err)
	}

	return pattern, filenames
}
