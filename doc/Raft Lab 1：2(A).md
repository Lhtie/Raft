# Raft Lab 1：2(A)

**提交须知：本Lab中需要完成注释中2(A)部分的代码（推荐在git里创建一个2(A)的分支），并回答下列数个问题。本Lab的ddl为7月25号的23:59分，代码部分的正确性将会由我在本地评测（如果担心正确性问题也可以提前让我进行测试），问题部分还是将提交的文件发至yang_xinyu@sjtu.edu.cn，关于上次问题的feedback会在近几天给出。该Lab的分数将结合Code Review给出，即所有Lab结束后进行。（但如果出现正确性问题我会即时私戳的）**

## Requirement

在本次lab中，需要你们完成 2(A) 部分的代码。

因未作出任何修改，具体要求请参见http://nil.csail.mit.edu/6.824/2018/labs/lab-raft.html

（后续的要求也应该和mit官方文档相同）

## 关于Raft的一些问题

在这部分中，你需要回答两道raft相关的问题。

1. Suppose the leader waited for all servers (rather than just a majority) to have matchIndex[i] >= N before setting commitIndex to N (at the end of Rules for Servers). What specific valuable property of Raft would this change break?
2. There are five Raft servers. S1 is the leader in term 17. S1 and S2 become isolated in a network partition. S3, S4, and S5 choose S3 as the leader for term 18. S3, S4, and S5 commit and execute commands in term 18. S1 crashes and restarts, and S1 and S2 run many elections, so that their currentTerms are 50. S3 crashes but does not re-start, and an instant later the network partition heals, so that S1, S2, S4, and S5 can communicate. Could S1 become the next leader? How could that happen, or what prevents it from happening? Your answer should refer to specific rules in the Raft paper, and explain how the rules apply in this specific scenario.
