package raft

import (
	"fmt"
	"strings"
)


type Log struct {
	Entries []Entry
	// Index0  int
}

type Entry struct {
	Command interface{}
	Term    int
	Index   int   // Each log entry also has an integer index identifying its position in the log.
}

func makeEmptyLog() Log {
	log := Log{
		Entries: make([]Entry, 0),
		// Index0:  0,
	}
	return log
}

func (l *Log) at(idx int) *Entry {
	return &l.Entries[idx]
}


func (l *Log) len() int {
	return len(l.Entries)
}

func (l *Log) lastLog() *Entry {
	return l.at(l.len() - 1)
}

func (l *Log) append(entries ...Entry)  {
	l.Entries = append(l.Entries, entries...)
}

func (l *Log) truncate(idx int) {
	// before idx
	l.Entries = l.Entries[:idx]
}


func (l *Log) slice(idx int) []Entry {
	return l.Entries[idx:]
}

func (l *Log) String() string {
	nums := []string{}
	for _, entry := range l.Entries {
		nums = append(nums,  fmt.Sprintf("%4d", entry.Term))
	}
	return fmt.Sprint(strings.Join(nums, "|"))
}