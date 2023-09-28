# grsync — golang rsync wrapper

Quick & Dirty fork of ranchers grsync for my own purposes.
Tested with `rsync version 3.2.3  protocol version 31`

## Changes made:

- Add sshpass-Wrapper to allow passing ssh-password to rsync (sshsync needs to be installed _duh_, see example)
- `Archive`-Flag is not set forcefully
- Changed Progress to reflect current rsync progress format (not a great solution currently, some more parsing & better
  regex would be nice)
- Allow disabling the creation of directory if remote target is used (
  see [#7](https://github.com/zloylos/grsync/issues/7))
- Rsync: Added ListOnly-Param (see examples)
- Updated deps

----

[![codecov](https://codecov.io/gh/zloylos/grsync/branch/master/graph/badge.svg)](https://codecov.io/gh/zloylos/grsync)
[![GoDoc](https://godoc.org/github.com/zloylos/grsync?status.svg)](https://godoc.org/github.com/zloylos/grsync)

Repository contains some helpful tools:

- raw rsync wrapper
- rsync task — wrapper which provide important information about rsync task: progress, remain items, total items and
  speed

## Task wrapper usage

```golang
package main

import (
	"fmt"
	"github.com/ByteSizedMarius/grsync"
	"time"
)

func main() {
	task := grsync.NewTask(
		"/local/source",
		"remote@target::destination",
		true,  // use sshpass
		false, // create directory
		grsync.RsyncOptions{ // optimized for speed in my specific use-case, no guarantees
			Inplace:         true,
			Partial:         true,
			HumanReadable:   true,
			NumericIDs:      true,
			Rsh:             "ssh -T -c aes128-ctr -o Compression=no -x",
			PasswordFile:    "/local/password/file", // contains password that will be used by sshpass
			RsyncBinaryPath: "/usr/bin/rsync",
		},
	)

	go func() {
		for {
			state := task.State()
			fmt.Printf(
				"progress: %d %% / rem. %s / tot. %s / sp. %s \n",
				state.Progress,
				state.TimeRemaining,
				state.DownloadedTotal,
				state.Speed,
			)
			<-time.After(time.Second * 5)
		}
	}()

	if err := task.Run(); err != nil {
		panic(err)
	}

	fmt.Println("well done")
	fmt.Println(task.Log())
}
```

Output:

```
progress: 0 % / rem.  / tot.  / sp.  
progress: 0 % / rem. 0:22:27 / tot. 342.82M / sp. 108.93MB/s 
progress: 0 % / rem. 0:22:16 / tot. 918.52M / sp. 109.41MB/s 
progress: 0 % / rem. 0:22:18 / tot. 1.49G / sp. 108.85MB/s 
progress: 1 % / rem. 0:22:16 / tot. 2.06G / sp. 108.58MB/s 
progress: 1 % / rem. 0:22:16 / tot. 2.63G / sp. 108.19MB/s 
progress: 2 % / rem. 0:22:10 / tot. 3.20G / sp. 108.21MB/s 
```

**File list:**

```golang
package main

import (
	"fmt"
	"github.com/ByteSizedMarius/grsync"
)

func main() {

	task := grsync.NewTask(
		"/local/source",
		"remote@target::destination",
		true,
		false,
		grsync.RsyncOptions{
			HumanReadable:   true,
			ListOnly:        true,
			Rsh:             "ssh -T -c aes128-ctr -o Compression=no -x",
			PasswordFile:    "/root/pass",
			RsyncBinaryPath: "/usr/bin/rsync",
		},
	)

	if err := task.Run(); err != nil {
		panic(err)
	}

	for _, file := range task.GetFileList() {
		fmt.Printf("Permissions: %s, Size: %s, Date: %s, Time: %s, Name: %s\n", file[0], file[1], file[2], file[3], file[4])
	}

	fmt.Println("done")
}

```