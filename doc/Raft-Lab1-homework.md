# Raft Lab 1 第二周作业

## 关于Raft的一些问题

#### 1. Suppose the leader waited for all servers (rather than just a majority) to have matchIndex[i] >= N before setting commitIndex to N (at the end of Rules for Servers). What specific valuable property of Raft would this change break?

如果只有当所有服务器复制了某个条目，leader才会commit的话。如果某个follower宕机了，那么之后的所有条目都不会被commit，客户端也得不到响应。如果是leader宕机了，那么新的leader还是能够同步commit过的条目，但是老leader没有commit的那些条目不一定能够最终被新leader同步并commit。当条件改成这样之后，依然能够保持Raft的安全性，但是只有所有服务器在线的时候才能正常工作，只要有一台服务器出现问题，就有很大可能丢失信息。

#### 2. There are five Raft servers. S1 is the leader in term 17. S1 and S2 become isolated in a network partition. S3, S4, and S5 choose S3 as the leader for term 18. S3, S4, and S5 commit and execute commands in term 18. S1 crashes and restarts, and S1 and S2 run many elections, so that their currentTerms are 50. S3 crashes but does not re-start, and an instant later the network partition heals, so that S1, S2, S4, and S5 can communicate. Could S1 become the next leader? How could that happen, or what prevents it from happening? Your answer should refer to specific rules in the Raft paper, and explain how the rules apply in this specific scenario.

在网络隔离的期间，S1和S2都不是leader，所以最后S1的日志还是隔离前的版本，如果在隔离期间，S3收到了很多日志，并且成功复制给了S4和S5，那么在隔离修复之后，根据只能给至少和自己的日志一样新的人投票的规则，S4和S5都不会给S1投票，S1也不可能成为leader。但是如果在隔离期间，没有新的日志被更新，或者S4和S5的没有成功完成日志复制，那么如果S1首先开始选举，S4和S5先把term更新为51，成为follower，接着直接给S1投票，S2也会给S1投票，S1就可以成功选举为leader。

