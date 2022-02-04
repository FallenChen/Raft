package mr

import (
	"fmt"
	// "log"
)

type TaskPhase int

const (
	MapPhase	TaskPhase = 0
	ReducePhase	TaskPhase = 1
)

type Task struct {
	FileName	string
	NReduce		int
	NMaps		int
	Seq		int
	Phase		TaskPhase
	Alive		bool
}
// intermediate files naming convention
func reduceName(mapIdx, reduceIdx int) string {
	return fmt.Sprintf("mr-%d-%d", mapIdx, reduceIdx)
}

func mergeName(reduceIdx int) string {
	return fmt.Sprintf("mr-out-%d", reduceIdx)
}