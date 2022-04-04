package raft

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (rf *Raft) appendEntries(heartbeat bool) {
	lastLog := rf.log.lastLog()
	for serverId, _ := range rf.peers {
		if serverId == rf.me {
			// not sent heart beat to self
			// reset election timer
			rf.resetElectionTimer()
			continue
		}

		// rules for leader 3
		// combine heartbeat and append entries
		if lastLog.Index >= rf.nextIndex[serverId] || heartbeat {

			nextIndex := rf.nextIndex[serverId]


			prevLog := rf.log.at(nextIndex - 1)

			args := AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLog.Index,
				PrevLogTerm:  prevLog.Term,
				Entries:      make([]Entry, lastLog.Index-nextIndex+1), // init
				LeaderCommit: rf.commitIndex,
			}
			// https://stackoverflow.com/questions/38923237/goroutines-sharing-slices-trying-to-understand-a-data-race
			copy(args.Entries, rf.log.slice(nextIndex))
			go rf.leaderSendEntries(serverId, &args)
		}

	}

}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) leaderSendEntries(serverId int, args *AppendEntriesArgs) {
	var reply AppendEntriesReply
	ok := rf.sendAppendEntries(serverId, args, &reply)
	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if reply.Term > rf.currentTerm {
		rf.setNewTerm(reply.Term)
		return
	}

	if args.Term == rf.currentTerm {
		// leader rule 3.1
		if reply.Success {
			match := args.PrevLogIndex + len(args.Entries)
			next := match + 1
			// if the logs end with the same term , then whichever log is longer is
			// more up-to-date
			rf.nextIndex[serverId] = next
			rf.matchIndex[serverId] = match
			DPrintf("[%v]: %v append success next %v match %v", rf.me, serverId, rf.nextIndex[serverId], rf.matchIndex[serverId])

		} else if rf.nextIndex[serverId] > 1 {
			rf.nextIndex[serverId]--
		}

		rf.leaderCommitRule()
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("[%d]: (term %d) follower 收到 [%v] AppendEntries %v, prevIndex %v, prevTerm %v", rf.me, rf.currentTerm, args.LeaderId, args.Entries, args.PrevLogIndex, args.PrevLogTerm)

	// rules for servers
	// all servers 2
	reply.Success = false
	reply.Term = rf.currentTerm

	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
		return
	}

	// append entries rpc 1
	if args.Term < rf.currentTerm {
		return
	}

	rf.resetElectionTimer()

	// candidate rule 3
	if rf.state == Candidate {
		rf.state = Follower
	}

	// append entries rpc 2
	if rf.log.lastLog().Index < args.PrevLogIndex || rf.log.at(args.PrevLogIndex).Term != args.PrevLogTerm {
		return
	}

	// append entries rpc 3
	if args.PrevLogIndex + 1 < rf.log.len() && rf.log.at(args.PrevLogIndex + 1).Term != args.Term {
		rf.log.truncate(args.PrevLogIndex + 1)
	}	

	for idx, entry := range args.Entries {

		// append entries rpc 4
		if entry.Index >= rf.log.len() || rf.log.at(entry.Index).Term != entry.Term {
			rf.log.append(args.Entries[idx:]...)
			DPrintf("[%d]: follower append [%v]", rf.me, args.Entries[idx:])
			break
		}
	}

	// append entries rpc 5
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.log.lastLog().Index)
		rf.apply()
	}

	reply.Success = true

}

func (rf *Raft) leaderCommitRule() {
	// leader rule 4
	if rf.state != Leader {
		return
	}

	N := rf.commitIndex
	for n := rf.commitIndex + 1; n <= rf.log.lastLog().Index; n++ {
		if rf.log.at(n).Term != rf.currentTerm {
			continue
		}

		counter := 1

		for serverId := 0; serverId < len(rf.peers); serverId++ {
			// exclude self
			if serverId != rf.me && rf.matchIndex[serverId] >= n {
				counter++
			}
			if counter > len(rf.peers)/2 {
				N = n
				break
			}
		}

	}

	if N == rf.commitIndex {
		return
	}

	rf.commitIndex = N
	DPrintf("[%v] leader尝试提交 commitIndex: %v", rf.me, rf.commitIndex)
	rf.apply()
}
