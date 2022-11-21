# Last node standing

The objective of the challenge is to implement a simple distributed concensus system based on libp2p and the Vocdoni SDK voting library.  The result of the challenge is a node agent written in go that supports the following functions:

## Phase 1) Discovery

The program takes a libp2p groupKey as an argument. This key is used to find and connect nodes within the libp2p network.

Using this p2p network the nodes exchange their public keys.

## Phase 2) Random leader election

The group decides a master node, based on some quick consensus approach (for example based on the node public key modulus).

## Phase 3) Weakest node voting

Using the provided SDK, the master node creates a vote census based on the fetched public keys.

Next, the master node creates an election with the all the nodes as candidates.

Once the election is created, the nodes vote one of their group peers according to their local criteria concerning the performance of the connection of the node. That is, they vote the worst connected node (network latency, IP geo or any other metric).

Once all votes are submited, as can be checked through the SDK, the most-voted node is kicked out the libp2p group key by all the rest of the group peers.

**Alternative:** Instead of connection information, use a proof-of-work mechanism. Each node needs to solve as many PoW puzzles as possible, the node that generated less puzzles is kicked.

## Phase 4) Last node standing

This process is repeated until there is only one standing node.

## Tecnical Info

For the voting implmentation, you can use the following libraries:

- Vocdoni API Golang SDK for creating the census, election and submit votes

[https://github.com/vocdoni/vocdoni-node/tree/master/apiclient](https://github.com/vocdoni/vocdoni-node/tree/master/apiclient)

You can use the development network, the API can consule an existing gateway, such as: https://gw1.dev.vocdoni.net/v2

Use the Master branch

- As transport layer use our libp2p customization

[https://github.com/vocdoni/vocdoni-node/tree/master/ipfssync/subpub](https://github.com/vocdoni/vocdoni-node/tree/master/ipfssync/subpub)


You can take as much time as you find necessary, and implement to the extent that you find meaningful.

Feel free to join our Discord server to ask questions:

[https://discord.gg/xzz9BgHx](https://discord.gg/xzz9BgHx)

