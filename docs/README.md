# Current features supported
In this version of the node, the following functionalities have been covered

## Phase 1) Discovery
This functionality has been covered by making use of the provided SubPub library.
[https://github.com/vocdoni/vocdoni-node/tree/master/ipfssync/subpub](https://github.com/vocdoni/vocdoni-node/tree/master/ipfssync/subpub)

## Phase 2) Random leader election
This functionality has been covered by following the process below.

- Periodically, public keys are shared in hexadecimal format. The node that has the public key with the highest value will be chosen as the lead node.
- If a new node connects to the network, it will ask for the leader node.
- If a node disconnects and that was the leader, the leader is re-elected.

## Phase 3) Weakest node voting
This functionality has not been fully completed. The following library has been used to implement voting
[https://github.com/vocdoni/vocdoni-node/tree/master/apiclient](https://github.com/vocdoni/vocdoni-node/tree/master/apiclient)

- Election creation and voting have been implemented, but there have been problems with vote verification.
- A PoW has been implemented to see which node solves the most Puzzles in 20 seconds

## Execution

To raise a node it is only necessary to have Golang installed and execute the following command.
```
 go run launch_node.go --port <number of port> --groupKey <key>
```
To raise more nodes you just have to execute the same command changing the port number and the same groupKey

