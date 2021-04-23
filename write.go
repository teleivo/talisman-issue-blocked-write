// This snippet is emulating the part of the code in the talisman scanner
// https://github.com/thoughtworks/talisman/blob/f6a09e119a1ddae1b3773c3204a1ebe1250e10f0/scanner/scanner.go#L33-L40
// which causes
//   talisman --scan
// to hang. Specifically the getBlobsInCommit function.
package main

import (
	"fmt"
	"os/exec"
)

// This is a simplification of what the scanner.getBlobsInCommit does. The
// actual git command that is run does not matter. What matters is that the
// command that is executed writes bytes to STDOUT. The
// exec.Command().CombinedOutput() is waiting for the command to finish. The
// syscall executing the write to STDOUT gets stuck as the pipe buffer that the
// command is writing to is full, so subsequent write() calls will block until
// the pipe buffer is read from (drained). All this happens on the OS level,
// not visible to the Go exec.Command. At least as far as I known.
//
// Related documentation to read:
// man 2 write (tells you that write will block on a full pipe buffer)
// man 7 pipe (tells you the default pipe buffer size)
func main() {
	// the talisman scanner creates as many goroutines as commits; tune this to see when
	// your machine produces bytes faster than it can read them from the pipe
	// buffer.
	commits := 1000
	blobs := make(chan []byte, commits)
	for i := 0; i < commits; i++ {
		go func() {
			// The commented line shows the git command that is actually
			// executed in the talisman scanner. Its iterating through all
			// commits a repos history to gather the tree at that commit
			// blob, _ := exec.Command("git", "ls-tree", "-r", commit).CombinedOutput()

			// This shows that the error has nothing to do with git itself but
			// rather the way the command is executed. I am using dd to write
			// bytes to stdout. You might also have to tune the count based on
			// your machine.
			//
			//   default capacity on my machine is 65,536 bytes
			//
			// I chose bs=4096 as running strace on the stuck git ls-tree
			// showed a write with that amount as the count parameter
			// I chose count=25 to get a total of > 100k which was the tree
			// size of some commits that got stuck in the 'git ls-tree -r'.
			// Less might also suffice.
			blob, _ := exec.Command("dd", "if=/dev/urandom", "status=none", "bs=4096", "count=25").CombinedOutput()
			fmt.Printf("len(tree) = %+v\n", len(blob))
			blobs <- blob
		}()
	}
	for i := 0; i < commits; i++ {
		blob := <-blobs
		_ = blob
		fmt.Printf("got blob i=%+v\n", i)
	}
	fmt.Println("done")
}
