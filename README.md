# 6.824 Raft

[地址](http://nil.csail.mit.edu/6.824/2021/schedule.html)

> 目前完成lab2 C，剩下的后续补充

[toc]



必须看：

* “raft-extended”

* "students-guide-to-raft"

## Leader Election

实验提示及5.2节，可以得出：

1. 只有3个角色：leader、candidate、follower，相关的状态转换

2. 参考论文Figure2，可以知道三个角色的结构体中的属性和需要实现的方法

3. RPC结构体均用大写（虽然点很小，但坑得很惨）

4. 可以先不实现log，即心跳可以先不实现

5. 超时后的选举需要实现（leaderElection）

6. 需要在合适的地方及时重置选举时间（resetElectionTimer）
   
   * you get an AppendEntries RPC from the current leader (i.e., if the term in the AppendEntries arguments is outdated, you should not reset your timer)
   
   * you are starting an election
   
   * you grant a vote to another peer

7. 不建议全部都写在raft中，分开命名，并且正确命名相关的函数（如：leaderElection（），刚开始傻傻的按着代码上写，没多想，写着写着突然不知道哪些是发送的，哪些是follower/candidate接收并回复，出bug的时候完全没有思路）

8. 很有必要理解发送RPC的顺序，“sendRequestVote”是发送RPC(req)，阻塞的，根据“ok”来判断，直接读取“reply”(resp)，这样就算一个请求的来回。实际请求响应的逻辑，在“RequestVote”中（根据Figure2中的描述实现，需要根据情况额外添加内容，比如重置选举时间）。作为一个Java开发，日常的req、resp概念在这里完全不适用，坑了很久

9. candidate成为Leader，一定要使用sync.Once 的Do方法，否则测试的时候会没完没了的选举(选举出来了还选举)，又坑了很久。对go的API不熟，看了别人的代码后才了解，这东西就相当于java中的单例，双重检查的单例，头大

10. DPrint是个好东西，在util里早给你实现好，不dubug的时候可以关掉。话说回来，全程debug只能靠打日志，QAQ

11. 锁的用法一定要慎重，用好Lock就够了，CAS的技术，作为实验来说，是nice to have的功能，非常非常难。也不是所有的"rf.mu.Lock()"后面都是"defer rf.mu.UnLock()"，一定要慎重判断 

## Log

上一节很多偷懒没实现，混过去了，这一节都要补回来😭

根据论文及students-guide

* 先看Figure6，除了log index外，其它的分别是，1,1,1,2,3...这些是Term，x<-3 y<-1是Command，供上层状态机使用。raft只保存command，不涉及其它逻辑，具体可以查看raft_diagram的流程。raft只是replicate-state- machine一部分，刚做的时候完全不知道是什么回事

* 这里就需要一个log，log里头存放Entries，一个Entry数组，Entry里面有Index，Term，Command。Command没有太多要求，直接interface{}

* 正常实现appendEntries，figure2里头说也可以当作heartbeat，而且和普通的replicate log entries参数完全一样，毫无头绪。参考别人代码，发现可以合并在一块，"lastLog.Index >= rf.nextIndex[serverId] || heartbeat"，编程技巧上的不足啊

* leader不给自己发送心跳，直接重置

* *commitIndex* 已知可以提交的日志条目下标，*lastApplied*已经提交了的日志条目下标，*nextIndex[]*指leader保留的对应follower的下一个需要传输的日志条目，初始化为(lastIndex+1)，见论文"When a leader first comes to power,
  it initializes all nextIndex values to the index just after the
  last one in its log (11 in Figure 7)."  *matchIndex[]*指leader已经传输给对应follower的日志条目下标，即对应follower目前所拥有的总的日志条目，通常为nextIndex - 1，follower 确认后才更新。更新match使用 prevLogIndex + len(entries)，后再更新next。match是安全的，next是乐观的

* AppendEntries中的entries, heartbeat时候为empty，否则为log entires。这里借鉴别人代码，很巧妙的实现"lastLog.Index - nextIndex + 1"，兼容了两者逻辑

* leader commit rule也很绕，里面说的majority，简单实现就是大于一半

* leader commitIndex更新后，Broadcast所有“apply log[lastApplied] to state machine” 的apply协程(applier())，通知它们可以开始干活，否则已知wait()。参考别人代码，不这样用确实很难处理，ApplyMsg是开了goroutine去处理的

* follower接收AppendEntries，重置选举时间，把candidate变成follower，处理日志冲突。这里也比较绕，有截断、有拼接

* RequestVote需要添加upToDate实现，candidate变成leader后，需要初始化nextIndex和matchIndex

* entries[]不能传递slice！！！

* raft中的"Start"是state machine 的入口，仔细查看相关测试用例代码

* leader什么时候提交的问题???

## Persistence

currentTerm、votedFor、log[]更改的地方都需要persist()

需要解决fast log backtracking问题，因为每次都让nextIndex--太慢了



考虑三种情况（例子来自讲义：6.824 2020 Lecture 7: Raft (2)）：

|     | Case 1  | Case 2  | Case 3  |
|:--- |:------- | ------- | ------- |
| S1  | 4 5 5   | 4 4 4   | 4       |
| S2  | 4 6 6 6 | 4 6 6 6 | 4 6 6 6 |

S2 是任期 6 的 Leader，S1 刚重启加入集群，S2 发送给 S1 的 `AppendEntries RPC` 中 prevLogTerm=6

我们用三个变量来快速处理日志冲突：

* XTerm：冲突 entry 的任期，如果存在的话

* XIndex：XTerm 的第一条 entry 的 index

* XLen：缺失的 log 长度，case 3 中 S1 的 XLen 为 1

上述情况对应：

* Case 1：Leader 中没有 XTerm(即 5)，`nextIndex = XIndex`

* Case 2：Leader 有 XTerm(即 4)，`nextIndex = leader's last entry for XTerm`

* Case 3：Follower 的日志太久没追上，`nextIndex = XLen`

具体实现：

* 首先得确定在哪实现，显然，应该在append_entry的Reply结构体中添加，还需添加一个Conflict字段做判断

* AppendEntries中处理上述逻辑，follower在对比leader发送的args

* “rf.log.lastLog().Index < args.PrevLogIndex” 和 “rf.log.at(args.PrevLogIndex).Term != args.PrevLogTerm” 需要分开判断，因为对应的case 3 和  case 2

* append entries rpc 3 的校验需要移到 Entries的循环里，因为figure8的Test Case，leader 和peer 的log顺序会完全不一样，在循环里truncate最稳妥

* append entries rpc 4 去除 “rf.log.at(entry.Index).Term != entry.Term”判断，总体改为“entry.Index > rf.log.lastLog().Index”，这样更准确

* leaderSendEntries中判断冲突，if Conflict 成立，根据case判断nextIndex的取值，需要添加一个“findLastLogIndexTerm”函数

* 测试下来，之前2A、2B做得不严谨的地方全炸出来了，figure8 case打印的日志简直数量直接💥

## Log Compaction

Todo

### 总结

实验顺序：

1. 实现选举

2. 实现日志复制

3. 实现持久化

4. 实现快照
