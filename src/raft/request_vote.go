package raft

import "sync"

type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

type RequestVoteReply struct {
	// Your data here (2A).
	Term int
	VoteGranted bool
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// rules for servers
	// all servers 2
	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}

	// request vote rpc receiver 1
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// request vote rpc receiver 2
	// Raft determines which of two logs is more up-to-date by comparing 
	// the index and term of the last entries in the logs. 
	// If the logs have last entries with different terms, 
	// then the log with the later term is more up-to-date. 
	// If the logs end with the same term, 
	// then whichever log is longer is more up-to-date.

	myLastLog := rf.log.lastLog()
	upToDate := args.LastLogTerm > myLastLog.Term ||
			(args.LastLogTerm == myLastLog.Term && args.LastLogIndex >= myLastLog.Index)

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && upToDate {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.persist()
		rf.resetElectionTimer()
		DPrintf("[%v]: term %v vote %v", rf.me, rf.currentTerm, rf.votedFor)
	} else {
		reply.VoteGranted = false
	}
	reply.Term = rf.currentTerm

}


func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


func (rf *Raft) candidateRequestVote(serverId int, args *RequestVoteArgs, voteCounter *int, becomeLeader *sync.Once) { {
	DPrintf("[%d]: term %v send vote request to %d\n", rf.me, args.Term, serverId)
	reply := RequestVoteReply{}
	ok := rf.sendRequestVote(serverId, args, &reply)
	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if reply.Term > args.Term {
		DPrintf("[%d]: %d 在新的term，更新term，结束\n", rf.me, serverId)
		rf.setNewTerm(reply.Term)
		return
	}

	if reply.Term < args.Term {
		DPrintf("[%d]: %d 的term %d 已经失效，结束\n", rf.me, serverId, reply.Term)
		return
	}

	if !reply.VoteGranted {
		DPrintf("[%d]: %d 没有投给me，结束\n", rf.me, serverId)
		return
	}

	DPrintf("[%d]: from %d term一致，且投给%d\n", rf.me, serverId, rf.me)

	*voteCounter++

	if *voteCounter > len(rf.peers) / 2 && 
		rf.currentTerm == args.Term &&
		rf.state == Candidate {
			DPrintf("[%d]: 获得多数选票，可以提前结束\n", rf.me)
			becomeLeader.Do(func ()  {
				DPrintf("[%d]: 当前term %d 结束\n", rf.me, rf.currentTerm)
				rf.state = Leader
				lastLogIndex := rf.log.lastLog().Index

				for i, _ := range rf.peers{
					rf.nextIndex[i] = lastLogIndex + 1
					rf.matchIndex[i] = 0
				}
				rf.appendEntries(true)
			})
		}
	}
}