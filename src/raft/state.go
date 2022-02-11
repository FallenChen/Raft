package raft

type PeerState string

const (
	Follower PeerState = "Follower"
	Candidate PeerState = "Candidate"
	Leader PeerState = "Leader"
)