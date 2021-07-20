# Raft Lab 0 第一周作业

## Github Repo

https://github.com/Lhtie/Raft.git

## 阅读理解

### 1. Try to prove the Leader Completeness in your own words.  

用反证法证明，首先假设某个leader T拥有一条commit过的日志 L，而在T之后第一个日志中没有L的leader为U。

T在commit L的时候一定发送给了所有服务器，并且超过半数的服务器都在日志中同步了L，而在之后的在U之前的leader任期中，L都会在leader的日志中出现（U是第一个日志中没有L的leader），并且不会被覆盖，所有原来同步了L的服务器依然持有L。

由于leader不会覆盖已有日志，则在U选举的时候，就没有日志L。而等到U进行选举时，U一定收到了超过半数的服务器的投票，由于又有超过半数的服务器还持有L，所以一定有一个服务器S既存有这条L，又给U投了票。

因为S给U投票，一定满足U的日志比S的新，由“新”的定义，分两种清空。

第一种是S和U的最后一条日志的任期相同，那么U的日志一定比S的长，所以U的日志一定包含所有S的日志（S和U的最后一条日志任期相同，保证了这一点，因为每次都会使更新后的日志和leader的日志一致），S包含日志L，则U包含日志L，矛盾。

第二种是S和U的最后一条日志任期不同，并且U的任期更大，那么在U最后一条日志被更新时的那个任期上的leader一定是包含有L的（U是第一个日志中没有L的leader），于是，更新这条日志的同时间接保证了U拥有日志L，矛盾。

所以S不可能给U投票，矛盾。所以U一定包含了T的所有日志。

### 2. Suppose we have the scenario shown in the Raft paper's Figure 7: a cluster of seven servers,with the log contents shown. The first server crashes (the one at the top of the figure), and cannot be contacted. A leader election ensues. For each of the servers marked (a), (d), and (f), could that server be elected? If yes, which servers would vote for it? If no, what specific Raft mechanism(s) would prevent it from being elected?  

![raft-图7](C:\Users\lhtie\Documents\ACM Class\大一下\PPCA\Raft\raft-zh_cn-master\images\raft-图7.png)

对于（a），下面的（b）、（e）、（f）都没有（a）新，会给他投票，总票数为4，选举成功。

对于（d），其他的服务器都会给他投票，选举成功。

对于（f），其他的服务器的日志都比它新，获得总票数为1，不会选举成功。

### 3. Each figure below shows a possible log configuration for a Raft server (the contents of log entries are not shown; just their indexes and terms). Considering each log in isolation, could that log configuration occur in a proper implementation of Raft? If the answer is "no," explain why not.  

![image-20210714120324582](C:\Users\lhtie\AppData\Roaming\Typora\typora-user-images\image-20210714120324582.png)

a. 不可能，任期号只会递增，每次添加日志只会向后添加，所以日志的任期号必须递增。

b. 可能。

c. 可能。

d. 不可能，leader添加日志会从最后添加，不会跳过某个索引。而follower的日志更新之后，与leader保持一致，也不会跳过某个索引。

### 4. Suppose that a hardware or software error corrupts the nextIndex value stored by the leader for a particular follower. Could this compromise the safety of the system? Explain your answer briefly.  

如果leader针对某个follower的nextIndex不可用了，leader会正常地将附加条目更新给其他大多数正常的follower中，并且成功将这条日志commit。由于那个follower的日志比较落后，不会当选leader。直到下一个leader上任的时候，nextIndex会重新初始化，等到更新到那个follower的时候，不断检查一致性，直到成功之后把所有entry同步过去，正常运行。

### 5.Try to list at least two differences between Raft and Paxos, and explain them briefly.

1. 选举过程：Paxos在任期为t的时候，只有满足`t mod n = s`的第s个服务器可以成为candidate，保证每个任期最多一个leader。在RequestVote的过程中获取follower最新的条目，保证leader包含所有commit过的条目。而Raft通过投票，获得大多数投票的服务器才能成为leader，而通过限制只能给比自日志更新的candidate投票，来确保新选的leader拥有所有commit过的条目。
2. 提交之前任期的条目：Paxos通过修改之前任期条目的任期号为当前的任期号，再进行复制和commit，把过去的条目当做当前任期的条目处理。而Raft保留之前任期的条目的任期号，通过限制leader只能commit当前任期的已经复制给大多数服务器的条目以及之前的条目，来间接地commit之前任期中添加的条目。

