# Talisman Issue - Blocked Write

## Summary

`talisman --scan` intermittently but frequently hangs on a `git ls-tree -r`
which is executed to gather all blobs and the commits they were in.

Here is where the command is executed
https://github.com/thoughtworks/talisman/blob/f6a09e119a1ddae1b3773c3204a1ebe1250e10f0/scanner/scanner.go#L45

This shows that a goroutine is created per commit to gather the tree and its
blobs
https://github.com/thoughtworks/talisman/blob/f6a09e119a1ddae1b3773c3204a1ebe1250e10f0/scanner/scanner.go#L33-L40

## How To Reproduce It

To reproduce this issue you might simply need to run `talisman --scan` on a
repo like https://github.com/apache/bookkeeper

If that does not cause the scan to hang try running the code in this repo.

```sh
go run write.go
```

You can adjust the count and bs arguments passed to `dd` as explained in the
code's comments.

How do you know talisman is not making any progress?

This issue will appear before any tests on the file contents are executed. So
you will only see the `Running Scan` ACSII art and nothing else. You can also
do a

```sh
ps --ppid $(pgrep talisman) -F

 PID  PPID  C    SZ   RSS PSR STIME TTY          TIME CMD
4149  3874  0 18113     0   0 Apr22 pts/1    00:00:00 git ls-tree -r c9dc301feb48ca170c3d6205a36fca63a4950c5a
4162  3874  0 18113     0   2 Apr22 pts/1    00:00:00 git ls-tree -r 33ea58027b0a3ba160f7ac19d20568709f453f4d
```

You should also see one or more `git ls-tree -r` commands that just never
finish.

## Root Cause

The `git ls-tree` command that is executed writes bytes to STDOUT. The Go
[exec.Command().CombinedOutput()](https://golang.org/pkg/os/exec/#Cmd.CombinedOutput)
is waiting for this command to finish. The syscall executing the write to
STDOUT for `git ls-tree` gets stuck as the pipe buffer is full. Subsequent
write() calls will therefore block until the pipe buffer is read from
(drained). All this happens on the OS level, not visible to the Go
exec.Command. At least as far as I known.

Related documentation to read:

- man 2 write (tells you that write will block on a full pipe buffer)
- man 7 pipe (tells you the default pipe buffer size)

This line is where the command is executed
https://github.com/thoughtworks/talisman/blob/f6a09e119a1ddae1b3773c3204a1ebe1250e10f0/scanner/scanner.go#L45

As many goroutines as commits are created. Executing basically the same code
sequentially does not have this issue.

## Detailed Analysis

I did my analysis while debugging a stuck `talisman --scan` on the
https://github.com/apache/bookkeeper repo. I can see the same thing happening
in other repos as well. Scanning the https://github.com/thoughtworks/talisman
works fine for me.

When a talisman scan does not show me the progress bar within a few minutes
its likely that it is stuck.

If I then do a

```
ps --ppid $(pgrep talisman) -F

 PID  PPID  C    SZ   RSS PSR STIME TTY          TIME CMD
4149  3874  0 18113  9688   0 17:40 pts/1    00:00:00 git ls-tree -r c9dc301feb48ca170c3d6205a36fca63a4950c5a
4162  3874  0 18113  8820   1 17:40 pts/1    00:00:00 git ls-tree -r 33ea58027b0a3ba160f7ac19d20568709f453f4d
```

I can see that one or more `git ls-tree -r` commands are still around but do
not finish.

strace shows me the syscalls for these processes

```sh
strace -s 99 -ffp 4149
strace: Process 4149 attached
write(1, "AuditorElector.java\n100644 blob 9830c592904cf4848d4068b64e562b78e815b5dc\tbookkeeper-server/src/main"..., 4096
```

this to

```sh
strace -s 1000000000 -p 4162
write(1, "e/hedwig/client/benchmark/BenchmarkPublisher.java\n100644 blob 0f8cb7f381c7407e63601cf737cddab530d20123\thedwig-client/src/main/java/org/apache/hedwig/client/benchmark/BenchmarkSubscriber.java\n100644 blob 3efe22da20938a875dee575044d9c5e4e9d234b0\thedwig-client/src/main/java/org/apache/hedwig/client/benchmark/BenchmarkUtils.java\n100644 blob e7b15f26a2ffef6e44573adf95a011e296007fd8\thedwig-client/src/main/java/org/apache/hedwig/client/benchmark/BenchmarkWorker.java\n100644 blob cc5e93778a041724de8a050276fcc3497f14c21b\thedwig-client/src/main/java/org/apache/hedwig/client/benchmark/HedwigBenchmark.java\n100644 blob 21ce9d3b34c9bec19eee58fba6001bedb63c2f46\thedwig-client/src/main/java/org/apache/hedwig/client/conf/ClientConfiguration.java\n100644 blob 346d74b34b1a728f38b0a74e036fc88b1c0e8474\thedwig-client/src/main/java/org/apache/hedwig/client/data/MessageConsumeData.java\n100644 blob 63547a0fdafff58646fe83f713c16d9741aa0abd\thedwig-client/src/main/java/org/apache/hedwig/client/data/PubSubData.java\n100644 blob 064cec12d379684adec3a4f33a46f22625919783\thedwig-client/src/main/java/org/apache/hedwig/client/data/TopicSubscriber.java\n100644 blob 5f468e6d3f5b05408946f3485861e8004d13f030\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/AlreadyStartDeliveryException.java\n100644 blob 3e543569f09f1dab37b23542115faeb85c088e85\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/InvalidSubscriberIdException.java\n100644 blob 22b44b16f649b0efd93b9530164ae9aad9b962e5\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/NoResponseHandlerException.java\n100644 blob c9aeb385307340e75c03e24195d333ef0fbc5933\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/ResubscribeException.java\n100644 blob da6d4e7d39ee0a1359a9f2dcb364697e3ae25384\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/ServerRedirectLoopException.java\n100644 blob 4a3c99f0f42beea2858fc203a824a1d93a2a3885\thedwig-client/src/main/java/org/apache/hedwig/client/exceptions/TooManyServerRedirectsException.java\n100644 blob bb2c0bb658b8bdef6f7b535df671a857a0b0df06\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/AbstractResponseHandler.java\n100644 blob 102dfb509a450fef90116e97982960b1f7dda258\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/CloseSubscriptionResponseHandler.java\n100644 blob 436c14f85b5e65be42196f14d5160ecc4db652ee\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/MessageConsumeCallback.java\n100644 blob dacaa7aa715e6099810d58d3831a2a9376d588b0\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/PubSubCallback.java\n100644 blob fc6a0251074488ef169090531dd8c7336e12681d\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/PublishResponseHandler.java\n100644 blob e2c685f91d687e8b709653af50b6fe3dcefa0231\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/SubscribeResponseHandler.java\n100644 blob 3ddd5390553150162e9482d6e2125998cb12fde2\thedwig-client/src/main/java/org/apache/hedwig/client/handlers/UnsubscribeResponseHandler.java\n100644 blob 0c676a13c909580f1aa85105fa54d1eb6469e273\thedwig-client/src/main/java/org/apache/hedwig/client/netty/CleanupChannelMap.java\n100644 blob 94e0a808e7858020c4d0f3692126b7590bc169bb\thedwig-client/src/main/java/org/apache/hedwig/client/netty/FilterableMessageHandler.java\n100644 blob 340cec57553513c96524c12f7f2826648107581e\thedwig-client/src/main/java/org/apache/hedwig/client/netty/HChannel.java\n100644 blob 6fae6bb2588d6d6b666df72793c3628c16fba38e\thedwig-client/src/main/java/org/apache/hedwig/client/netty/HChannelManager.java\n100644 blob 8ae0e8207e171f4d8b79ca9e605f573709884ca0\thedwig-client/src/main/java/org/apache/hedwig/client/netty/HedwigClientImpl.java\n100644 blob 5611bdd0c6e5f6871ec1fd6c751f6b16761aa2e6\thedwig-client/src/main/java/org/apache/hedwig/client/netty/HedwigPublisher.java\n100644 blob 7d2453aa29d477dd823ec0bbeb5a183e3efce531\thedwig-client/src/main/java/org/apache/hedwig/client/netty/HedwigSubscriber.java\n100644 blob 1d4f95555ac34c9ddff68e913c0b865b09de581c\thedwig-client/src/main/java/org/apache/hedwig/client/netty/NetUtil", 4096
```

I can validate the tree content by executing the `git ls-tree -r` command from
the `ps` output.

Looking at the documentation

```sh
man 2 write
```

```
WRITE(2)                                                               Linux Programmer's Manual                                                               WRITE(2)

NAME
       write - write to a file descriptor

SYNOPSIS
       #include <unistd.h>

       ssize_t write(int fd, const void *buf, size_t count);
...
       On Linux, write() (and similar system calls) will transfer at most 0x7ffff000 (2,147,479,552) bytes, returning the number of bytes actually transferred.   (This
       is true on both 32-bit and 64-bit systems.)
```

tells me that both processes are trying to write to STDOUT. Due to the
filedescriptor number 1. https://stackoverflow.com/questions/12902627/the-difference-between-stdout-and-stdout-fileno/12902707#12902707

Validated by

```sh
ls -l /proc/4162/fd
total 0
lr-x------ 1 ivo ivo 64 Apr 22 20:33 0 -> /dev/null
l-wx------ 1 ivo ivo 64 Apr 22 20:33 1 -> 'pipe:[52986]'
l-wx------ 1 ivo ivo 64 Apr 22 20:33 2 -> 'pipe:[52986]'
```

```sh
ls -l /proc/4149/fd
total 0
lr-x------ 1 ivo ivo 64 Apr 22 20:33 0 -> 'pipe:[53074]'
l-wx------ 1 ivo ivo 64 Apr 22 20:33 1 -> 'pipe:[53074]'
l-wx------ 1 ivo ivo 64 Apr 22 20:33 2 -> 'pipe:[53074]'
```

These
https://unix.stackexchange.com/questions/339401/can-writing-to-stdout-place-backpressure-on-a-process
https://groups.google.com/g/nodejs/c/Ua4nmiNPZXY

led me to

```
man 7 pipe
```

```
Since Linux 2.6.11, the pipe capacity
       is 16 pages (i.e., 65,536 bytes in a system with a page size of 4096 bytes).  Since Linux 2.6.35, the default pipe capacity is 16 pages, but the capacity can be
       queried and set using the fcntl(2) F_GETPIPE_SZ and F_SETPIPE_SZ operations.
```

Telling me that the default pipe buffer capacity is 65,536 bytes.

Looking at the io of the processes I see

```sh
cat /proc/4162/io
rchar: 14488
wchar: 65536
syscr: 33
syscw: 36
read_bytes: 1634304
write_bytes: 0
cancelled_write_bytes: 0
```

```sh
cat /proc/4149/io
rchar: 14488
wchar: 65536
syscr: 33
syscw: 20
read_bytes: 491520
write_bytes: 0
cancelled_write_bytes: 0
```

https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/filesystems/proc.rst?id=HEAD#n1730

```
wchar
I/O counter: chars written
The number of bytes which this task has caused, or shall cause to be written
to disk. Similar caveats apply here as with rchar.
```

Which means that these processes have written or want to write the maximum
amount of bytes:

`default capacity is 65,536 bytes`

(assuming 1 byte per character as the git ls-tree output looks like ASCII at
least for these particular trees).

So the processes block on write as the pipe buffers are full.

Just to get an idea of the number of bytes the processes are trying to write to
STDOUT

```sh
git ls-tree -r c9dc301feb48ca170c3d6205a36fca63a4950c5a | dd
341408 bytes (341 kB, 333 KiB)

git ls-tree -r 33ea58027b0a3ba160f7ac19d20568709f453f4d | dd
98917 bytes (99 kB, 97 KiB)
```

Both exceed the default of 65k.

I then wrote a simplyfied version of the talisman scanner that can be found in
this repo at [write.go](./write.go). This allows me to reproduce the issue and
shows that its independent of git. Its rather related to how the command is
executed and not read from fast enough.

### Repository Sizes

TODO write about how they play into this.
TODO Find a go project with more commits than talisman. Does that happen as well? Is
it more related to the size of trees than number of commits?

So I was wondering, why does a scan of the talisman repo exit successfully?
Why does a scan of a lot of other repos get stuck?

I used [GitHub's git-sizer](https://github.com/github/git-sizer) project to
analyze and compare repositories.

Number of bytes per tree of every commit

```sh
git --no-pager log --all --pretty=%H | xargs -I_ sh -c "git ls-tree -r _ | wc --bytes"
```

run these on talisman and bookkeeper to compare them

and this might also explain why its fast for the talisman repo. Java repos
generally tend to have more deeply nested directories. Maybe also longer
filenames due to longer classnames.

### Goroutine Stack Traces

Just out of interest I ran the Go pprof profiler. This is how the goroutine
stack traces look like then

```
985  syscall             syscall.Syscall6(0xf7, 0x1, 0x1035, 0xc0018d3d30, 0x1000004, 0x0, 0x0, 0xc0020774e0, 0x14f, 0x1)
     syscall, 10 minutes
       /usr/local/go/src/syscall/asm_linux_amd64.s:43 +0x5
     os.(*Process).blockUntilWaitable(0xc0002e8120, 0x4, 0x4, 0x203000)
       /usr/local/go/src/os/wait_waitid.go:32 +0x9e
     os.(*Process).wait(0xc0002e8120, 0x8, 0x7feaf0, 0x7feaf8)
       /usr/local/go/src/os/exec_unix.go:22 +0x39
     os.(*Process).Wait(...)
       /usr/local/go/src/os/exec.go:129
     os/exec.(*Cmd).Wait(0xc001a36420, 0x0, 0x0)
       /usr/local/go/src/os/exec/exec.go:507 +0x65
     os/exec.(*Cmd).Run(0xc001a36420, 0xc000fbb770, 0xc001a36420)
       /usr/local/go/src/os/exec/exec.go:341 +0x5f
     os/exec.(*Cmd).CombinedOutput(0xc001a36420, 0x3, 0xc000595788, 0x3, 0x3, 0xc001a36420)
       /usr/local/go/src/os/exec/exec.go:567 +0x91
     main.putBlobsInChannel(0xc000269544, 0x28, 0xc00011ca80)
       /home/ivo/code/talisman-experiments/scanner-profiling/scanner.go:159 +0xe9
     created by main.getBlobsInCommit
       /home/ivo/code/talisman-experiments/scanner-profiling/scanner.go:149 +0xd1
3097 syscall             syscall.Syscall6(0x3d, 0x1042, 0xc0009f4b14, 0x0, 0x0, 0x0, 0x0, 0xc0009f4ac8, 0x46d7e5, 0xc000ab8180)
     syscall, 10 minutes
       /usr/local/go/src/syscall/asm_linux_amd64.s:43 +0x5
     syscall.wait4(0x1042, 0xc0009f4b14, 0x0, 0x0, 0x0, 0xffffffffffffffff, 0x0)
       /usr/local/go/src/syscall/zsyscall_linux_amd64.go:168 +0x76
     syscall.Wait4(0x1042, 0xc0009f4b9c, 0x0, 0x0, 0x853ec0, 0xa069a8, 0x38)
       /usr/local/go/src/syscall/syscall_linux.go:368 +0x51
     syscall.forkExec(0xc000aaa090, 0xc, 0xc000901f40, 0x4, 0x4, 0xc0009f4ce0, 0x37, 0x6890bc1200010400, 0xc00212d000)
       /usr/local/go/src/syscall/exec_unix.go:237 +0x558
     syscall.StartProcess(...)
       /usr/local/go/src/syscall/exec_unix.go:263
     os.startProcess(0xc000aaa090, 0xc, 0xc000901f40, 0x4, 0x4, 0xc0009f4e70, 0xc002135880, 0x37, 0x37)
       /usr/local/go/src/os/exec_posix.go:53 +0x29b
     os.StartProcess(0xc000aaa090, 0xc, 0xc000901f40, 0x4, 0x4, 0xc0009f4e70, 0x37, 0x1ed, 0x203000)
       /usr/local/go/src/os/exec.go:106 +0x7c
     os/exec.(*Cmd).Start(0xc000a10000, 0x1, 0xc0020ea3f0)
       /usr/local/go/src/os/exec/exec.go:422 +0x525
     os/exec.(*Cmd).Run(0xc000a10000, 0xc0020ea3f0, 0xc000a10000)
       /usr/local/go/src/os/exec/exec.go:338 +0x2b
     os/exec.(*Cmd).CombinedOutput(0xc000a10000, 0x3, 0xc000ab7788, 0x3, 0x3, 0xc000a10000)
       /usr/local/go/src/os/exec/exec.go:567 +0x91
     main.putBlobsInChannel(0xc00027e784, 0x28, 0xc00011ca80)
       /home/ivo/code/talisman-experiments/scanner-profiling/scanner.go:159 +0xe9
     created by main.getBlobsInCommit
       /home/ivo/code/talisman-experiments/scanner-profiling/scanner.go:149 +0xd1
```
