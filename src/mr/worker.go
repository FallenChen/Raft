package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"strings"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	w := worker {}
	w.mapf = mapf
	w.reducef = reducef
	w.reigster()
	w.run()
	
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

type worker struct 
{
	id	int
	mapf	func(string, string) []KeyValue
	reducef	func(string, []string) string
}

func (w *worker) run() {
	for 
	{
		t := w.reqTask()
		if !t.Alive {
			log.Fatalf("worker get task not alive, exit")
			return
		}
		w.doTask(t);
	}
}

func (w *worker) reqTask() Task {
	log.Printf("reqTask")
	args := TaskArgs {}
	args.WorkerId = w.id
	reply := TaskReply {}
	if ok := call("Coordinator.GetOneTask", &args, &reply); !ok {
		log.Fatalf("get task failed");
		os.Exit(1)
	}
	log.Printf("get task seq %v", reply.Task.Seq)
	return *reply.Task
}

func (w *worker) doTask(t Task) {
	log.Printf("doTask")
	switch t.Phase {
	case MapPhase:
		w.doMapTask(t)
	case ReducePhase:
		w.doReduceTask(t)
	default:
		panic(fmt.Sprint("unknown phase: %v ", t.Phase))
	}
}

func (w *worker) doMapTask(t Task) {
	log.Printf("doMapTask")
	contents, err := ioutil.ReadFile(t.FileName)
	if err != nil {
		log.Fatalf("read file failed: ", err)
	}

	kvs := w.mapf(t.FileName, string(contents))
	// two-dimensional slice
	reduces := make([][]KeyValue, t.NReduce)
	for _, kv := range kvs {
		i := ihash(kv.Key) % t.NReduce
		reduces[i] = append(reduces[i], kv)
	}

	for i, kvs := range reduces {
		fileName := reduceName(t.Seq, i)
		out, err := os.Create(fileName)
		if err != nil {
			w.reportTask(t, false, err)
			return
		}
		enc := json.NewEncoder(out)
		for _, kv := range kvs {
			// write to file
			err = enc.Encode(&kv)
			if err != nil {
				w.reportTask(t, false, err)
				return
			}
		}
		if err := out.Close(); err != nil {
			w.reportTask(t, false, err)
		}
	}
	w.reportTask(t, true, nil)
}

func (w *worker) doReduceTask(t Task) {
	log.Printf("doReduceTask")
	// for sort
	maps := make(map[string][]string)
	for idx :=0; idx < t.NMaps; idx++ {
		fileName := reduceName(idx, t.Seq)
		// open already map file
		f, err := os.Open(fileName)
		if err != nil {
			w.reportTask(t, false, err)
			return
		}
		dec := json.NewDecoder(f)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			if _, ok := maps[kv.Key]; !ok {
				maps[kv.Key] = make([]string,0,100)
			}
			maps[kv.Key] = append(maps[kv.Key], kv.Value)

		}
	}
	res := make([]string, 0, 100)
	for k,v := range maps {
		res = append(res, fmt.Sprintf("%v %v\n",k, w.reducef(k,v)))
	}
	// performance issue,foreach to write
	// fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

	// only write once
	// chmod 600 (rw-------)
	if err := ioutil.WriteFile(mergeName(t.Seq), []byte(strings.Join(res, "")), 0600); err != nil {
		w.reportTask(t, false, err)
	}
	w.reportTask(t, true, nil)
}

func (w *worker) reigster() {
	log.Printf("reigster")
	args := RegisterArgs {}
	reply := RegisterReply {}
	if ok := call("Coordinator.RegWorker", &args, &reply); !ok {
		log.Fatalf("reg failed");
	}
	w.id = reply.WorkerId
	log.Printf("reg success, worker id: %v", w.id)
}

func (w *worker) reportTask(t Task, done bool, err error) {
	log.Printf("reportTask")
	if err != nil {
		log.Printf("report task failed: %v", err)
	}
	args := ReportTaskArgs {}
	args.Done = done
	args.Seq = t.Seq
	args.Phase = t.Phase
	args.WorkerId = w.id
	reply := ReportTaskReply {}
	if ok := call("Coordinator.ReportTask", &args, &reply); !ok {
		log.Fatalf("report task failed: %v", args);
	}

}


//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Coordinator.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatalf("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
