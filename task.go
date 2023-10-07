package grsync

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Task is high-level API under rsync
type Task struct {
	rsync *Rsync

	state *State
	log   *Log
	mutex sync.Mutex
}

// State contains information about rsync process
type State struct {
	TimeRemaining   string `json:"remain"`   // Time Remaining (hh:mm:ss)
	DownloadedTotal string `json:"total"`    // Amount of downloaded Data in unknown unit
	Speed           string `json:"speed"`    // Speed of download in unknown unit
	Progress        int    `json:"progress"` // Progress in percent (0-100)
}

// Log contains raw stderr and stdout outputs
type Log struct {
	Stderr string `json:"stderr"`
	Stdout string `json:"stdout"`
}

// State returns information about rsync processing task
// lock mutex to avoid accessing it while ProcessStdout is writing to it
func (t *Task) State() State {
	t.mutex.Lock()
	c := *t.state
	t.mutex.Unlock()
	return c
}

// Log return structure which contains raw stderr and stdout outputs
func (t *Task) Log() Log {
	t.mutex.Lock()
	l := Log{
		Stderr: t.log.Stderr,
		Stdout: t.log.Stdout,
	}
	t.mutex.Unlock()
	return l
}

// GetFileList is a helper function that returns a partially parsed list of files if RsyncOptions.ListOnly is true.
// The Information is returned as a slice of slices of strings in the following format:
// Index	Value
// 0		Permissions
// 1		Size in Bytes
// 2		Date
// 3		Time
// 4		Name
func (t *Task) GetFileList() (files [][]string) {
	r := regexp.MustCompile(`([rwx-]{10}) (\d+) ((?:\d+/){2}\d+) ((?:\d+:){2}\d+) (.*)`)
	for _, l := range strings.Split(t.Log().Stdout, "\n") {
		if r.MatchString(l) {
			files = append(files, r.FindStringSubmatch(l)[1:])
		}
	}
	return
}

// Run starts rsync process with options
func (t *Task) Run() error {
	stderr, err := t.rsync.StderrPipe()
	if err != nil {
		return err
	}

	stdout, err := t.rsync.StdoutPipe()
	if err != nil {
		_ = stderr.Close()
		return err
	}

	var wg sync.WaitGroup
	go processStdout(&wg, t, stdout)
	go processStderr(&wg, t, stderr)
	wg.Add(2)

	if err = t.rsync.Start(); err != nil {
		// Close pipes to unblock goroutines
		_ = stdout.Close()
		_ = stderr.Close()

		wg.Wait()
		return err
	}

	wg.Wait()

	return t.rsync.Wait()
}

// NewTask returns new rsync task
func NewTask(source, destination string, useSshPass, createDir bool, rsyncOptions RsyncOptions) *Task {
	// Force set required options
	rsyncOptions.HumanReadable = true
	rsyncOptions.Partial = true
	rsyncOptions.Progress = true

	return &Task{
		rsync: NewRsync(source, destination, useSshPass, createDir, rsyncOptions),
		state: &State{},
		log:   &Log{},
	}
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\n' {
			// We have a line terminated by single newline.
			return i + 1, data[0:i], nil
		}
		advance = i + 1
		if len(data) > i+1 && data[i+1] == '\n' {
			advance += 1
		}
		return advance, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func processStdout(wg *sync.WaitGroup, task *Task, stdout io.Reader) {
	defer wg.Done()

	progressMatcher := newMatcher(`(\d+%)`)
	speedMatcher := newMatcher(`(\d+\.\d+.{2}\/s)`)
	totalMatcher := newMatcher(`^\s*(\d+.\d+[A-Za-z]*)`)
	timeRemainingMatcher := newMatcher(`(\d+:){2}\d+`)

	// Extract data from strings:
	// 15.17G  10%   92.23MB/s    0:23:54
	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanLines)
	for scanner.Scan() {
		logStr := scanner.Text()
		task.mutex.Lock()

		if totalMatcher.Match(logStr) {
			task.state.DownloadedTotal = totalMatcher.Extract(logStr)
		}

		if progressMatcher.Match(logStr) {
			tt := progressMatcher.Extract(logStr)
			task.state.Progress, _ = strconv.Atoi(strings.TrimRight(tt, "%"))
		}

		if timeRemainingMatcher.Match(logStr) {
			task.state.TimeRemaining = timeRemainingMatcher.All(logStr)[0]
		}

		if speedMatcher.Match(logStr) {
			task.state.Speed = getTaskSpeed(speedMatcher.ExtractAllStringSubmatch(logStr, 2))
		}

		task.log.Stdout += logStr + "\n"
		task.mutex.Unlock()
	}
}

func processStderr(wg *sync.WaitGroup, task *Task, stderr io.Reader) {
	defer wg.Done()

	reader := bufio.NewReader(stderr)
	for {
		logStr, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		task.mutex.Lock()
		task.log.Stderr += logStr + "\n"
		task.mutex.Unlock()
	}
}

func getTaskProgress(remTotalString string) (int, int) {
	const remTotalSeparator = "/"
	const numbersCount = 2
	const (
		indexRem = iota
		indexTotal
	)

	info := strings.Split(remTotalString, remTotalSeparator)
	if len(info) < numbersCount {
		return 0, 0
	}

	remain, _ := strconv.Atoi(info[indexRem])
	total, _ := strconv.Atoi(info[indexTotal])

	return remain, total
}

func getTaskSpeed(data [][]string) string {
	if len(data) < 1 || len(data[0]) < 2 {
		return ""
	}
	return data[0][0]
}
