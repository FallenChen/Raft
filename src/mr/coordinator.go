package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

const (
	TaskStatusReady = 0
	TaskStatusInQueue = 1
	TaskStatusRunning = 2
	TaskStatusMapDone = 3
	TaskStatusReduceDone = 4
	TaskStaustError = 5
)

const (
	MaxTaskRunTime  = time.Second * 10
	ScheduleInterval = time.Millisecond * 1000
)

type TaskStat struct {
	Status		int
	WorkerId	int
	StartTime       time.Time
}

type Coordinator struct {
	// Your definitions here.
	files		[]string
	nReduce		int
	taskPhase	TaskPhase
	taskStats       []TaskStat
	mutex		sync.Mutex
	workerSeq	int
	done 		bool
	taskCh	        chan Task

}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (c *Coordinator) getTask(taskSeq int) Task {
	task := Task{
		FileName: "",
		NReduce: c.nReduce,
		NMaps: len(c.files),
		Seq: taskSeq,
		Phase: c.taskPhase,
		Alive: true,
	}
	// log.Fatalf("c:%+v, taskseq:%d, lenfiles:%d, lents:%d", c, taskSeq, len(c.files), len(c.taskStats))
	if task.Phase == MapPhase {
		task.FileName = c.files[taskSeq]
	}
	return task
}

func (c *Coordinator) schedule(){
	// log.Printf("schedule")
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.done {
		return
	}

	mapFinish := 0
	reduceFinish := 0
	for i, stat := range c.taskStats {
		switch stat.Status {
		case TaskStatusReady:
			c.taskCh <- c.getTask(i)
			c.taskStats[i].Status = TaskStatusInQueue
		case TaskStatusInQueue:
		case TaskStatusRunning:
			if time.Now().Sub(stat.StartTime) > MaxTaskRunTime {
				c.taskStats[i].Status = TaskStatusInQueue
				c.taskCh <- c.getTask(i)
			}
		case TaskStatusMapDone:
			mapFinish += 1
		case TaskStatusReduceDone:
			reduceFinish += 1
		case TaskStaustError:
			c.taskStats[i].Status = TaskStatusInQueue
			c.taskCh <- c.getTask(i)
		default:
			panic("unknown task status")
		}
	}

	if mapFinish == len(c.files) {
		log.Printf("all map task done")
		c.initReduceTask()
	}
	if reduceFinish == c.nReduce {

		log.Printf("all reduce task done")
		c.done = true
	}

}

func (c *Coordinator) isDone() bool{
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.done
}


func (c *Coordinator) initMapTask() {
	// log.Printf("initMapTask")
	c.taskPhase = MapPhase
	c.taskStats = make([]TaskStat, len(c.files))
}
// reduces can't start until the last map task is done
func (c *Coordinator) initReduceTask() {
	// log.Printf("initReduceTask")
	c.taskPhase = ReducePhase
	c.taskStats = make([]TaskStat, c.nReduce)
}

func (c *Coordinator) RegWorker(args *RegisterArgs, reply *RegisterReply) error {
	// log.Printf("RegWorker")
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.workerSeq += 1
	reply.WorkerId = c.workerSeq
	return nil
}

func (c *Coordinator) doGetOneTask(args *TaskArgs, task *Task){
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if task.Phase != c.taskPhase {
		panic("req Task phase not match")
	}
	c.taskStats[task.Seq].Status = TaskStatusRunning
	c.taskStats[task.Seq].WorkerId = args.WorkerId
	c.taskStats[task.Seq].StartTime = time.Now()

}

func (c *Coordinator) GetOneTask(args *TaskArgs, reply *TaskReply) error {
	task := <-c.taskCh
	reply.Task = &task

	if task.Alive {
		c.doGetOneTask(args,&task);
	}
	return nil;
}

func (c *Coordinator) ReportTask(args *ReportTaskArgs, reply *ReportTaskReply) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.taskPhase != args.Phase  || args.WorkerId != c.taskStats[args.Seq].WorkerId {
		return nil
	}

	switch args.Done {
		case TaskStatusMapDone:
			c.taskStats[args.Seq].Status = TaskStatusMapDone
		case TaskStatusReduceDone:
			c.taskStats[args.Seq].Status = TaskStatusReduceDone
		case TaskStaustError:
			c.taskStats[args.Seq].Status = TaskStaustError
	}

	go c.schedule()
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {

	// Your code here.
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.done
}

func (c *Coordinator) tickSchedule(){

	for !c.isDone() {
		go c.schedule()
		time.Sleep(ScheduleInterval)
	}
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// Your code here.
	c.mutex = sync.Mutex{}
	c.files = files;
	c.nReduce = nReduce;
	if nReduce > len(files) {
		c.taskCh = make(chan Task, nReduce)
	} else {
		c.taskCh = make(chan Task, len(files))
	}
	// go c.initDone()
	c.initMapTask()
	go c.tickSchedule()
	c.server()
	log.Printf("coordinator init, files: %v, nReduce: %v", len(files), nReduce)
	return &c
}

