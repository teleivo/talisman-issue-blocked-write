// This snippet is a potential solution to
// https://github.com/thoughtworks/talisman/issues/301
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"sync"
)

func main() {

	var wg sync.WaitGroup
	// if we do not limit the number of concurrent git ls-tree operations we
	// will open too many files.
	commits := getAllCommits()
	trees := make(chan []byte, len(commits))
	sem := make(chan struct{}, 10) // maximum number git ls-tree that run at any time
	for _, commit := range commits {
		wg.Add(2) // TODO wait for both goroutines?
		go getCommitTree(commit, trees, sem, &wg)
	}
	wg.Wait()
	fmt.Println("done reading")
	fmt.Println(string(<-trees)) // print sample
}

func getAllCommits() []string {
	// TODO can this be done more efficiently? with -z for nul byte separation
	// and by passing a reader of sorts where one can range over Fields? like
	// strings.Fields just for nul byte separator
	out, err := exec.Command("git", "log", "--all", "--pretty=%H").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	commits := strings.Split(string(out), "\n")
	return commits[:len(commits)-1] // the last element is an empty string due to a trailing newline in the output
}

func getCommitTree(commit string, results chan []byte, sem chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	r, w := io.Pipe()
	defer w.Close()
	cmd := exec.Command("git", "ls-tree", "-r", commit)
	cmd.Stdout = w
	go func() {
		defer wg.Done()
		// TODO can this be done efficiently? ReadAll will probably allocate
		// a buffer
		// br := bufio.NewReader(r)
		// br.ReadBytes(\NUL) // read slice until nul byte which gives one blob
		// can we just read one blob at a time or is that too slow?
		// read all of it at once with ioutil.ReadAll() ?
		b, _ := ioutil.ReadAll(r)
		results <- b
	}()
	sem <- struct{}{}
	if err := cmd.Run(); err != nil {
		// TODO handle errors. Also what should we do with the STDERR
		// output?
		fmt.Println(err)
	}
	<-sem
}
