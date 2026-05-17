BitTorrent: The Engineering behind the BitTorrent protocol

At one point in time, around 2008–12, almost everyone was using PirateBay, uTorrent, KickAss Torrent, etc. to download their favorite movies, series, and other stuff. Shoutout to my uncle for introducing me to this software when I was just around 5 years old. I weirdly feel proud of using such a revolutionary technology in the early stages, but only in recent years am I realizing the importance of such a protocol.

And just in the last 2 years have I been interested in the entire P2P, Blockchain scene. So as a curious dude, I wondered how the entire thing works. So grab 2 cans of Red Bull, a few chips, and snacks because this is gonna be a pretty deep and in-depth view of the engineering behind the BitTorrent protocol.

Table of Contents:-
Introduction
Architecture
Choke Algorithm
Piece Selection Algorithm
Kademlia Distributed Hash Table (DHT) Algorithm
Conclusion
Introduction
Press enter or click to view image in full size

It’s a Peer-to-peer (P2P) network where every server/node/computer is connected to one another and all have the same capabilities. The BitTorrent protocol was created to make downloads faster and more efficient.

Normally, when one client wants a file, it downloads that entire file from a single server that’s somewhere. Now imagine multiple clients trying to download that same file from that single server, gonna be slow right? That’s where BitTorrent came in and thought, why not divide that file into small chunks and distribute them to multiple clients? This way a client can download chunk A from the client that holds it and similarly download the rest of the chunks, and finally combine them all to form the file they need. How amazing is that?

The file that gets uploaded gets divided into pieces, which in turn gets divided into blocks. So every client transfers blocks of a piece of a file to the one that’s requesting it.

Peers are all the computers that are part of the network and the active ones are called active peers. They can either be a seeder who has the entire file or small blocks of the file or can also be a leecher.

Leechers are the nodes that are downloading the blocks from the seeders/peers.

Seeders are the nodes that have the complete file downloaded in their system. The more the number of seeders faster the download speed for the leechers. So for example, if there is only one seeder, it’s a classic client-server model.

Architecture
Pieces
Files are split into pieces of equal length (except for the last piece). And this piece is then passed through a SHA1 hash function which is a 20-byte hash function. Although SHA1 has been declared deprecated by the National Institute of Standards and Technology (NIST) and the use of SHA1 has been ceased by almost every company, including tech giants like Google and Microsoft, the BitTorrent protocol remains the same and continues the use of this hashing algorithm. And since there is only one output for an input, the SHA1 hash remains the same for every piece, and all the SHA1 hashes of each piece are stored in the .torrent file (which we will talk about later) in an attribute “pieces”.

So when one peer downloads a piece of the file they need, it informs the rest of the peers that “I have this piece” so that the rest of the peers can start downloading that piece from this peer. And the same way, every other peer does the same thing.

.torrent file
This is the file you basically need to download a file. This .torrent file contains all the information you need to download all the pieces of the file which is distributed across the network. This file is basically like a dictionary with key-value pairs. What all does it hold?

announce: This includes the URL of the tracker. Now what’s a tracker? A tracker is simply an HTTP server that keeps track of all the peers and what piece/block of the file they have.
created by: The program that created
creation date: Unix time of the file i.e the number of seconds that have passed since 1st January 1970
encoding: By default it’s UTF-8
comment: Some comments made by the author of the .torrent file
info: This in itself is a dictionary that varies depending on whether it’s a folder you’re downloading or a single file. If it’s a single file, then it contains the name, length, and MD5 sum. If it’s a directory then it will contain the name of the directory and then a dictionary for files that contains the length, path (eg: [a,b,c.txt] => a/b/c.txt), and the MD5 sum. This also contains information about the pieces, which is basically the concatenation of the SHA1 Hash of every piece of the file.
Now normally one would think these key values are simply stored as a JSON object, but not, these are bencoded or b-encoded. It serves the same purpose as that of JSON or YAML. The encoding algorithm that is used here is fairly easy and is something you can do yourself in any programming language of your choice. Integers are encoded as i<integer in base 10 ASCII>e (eg: i69e, i0e, i-420e), etc.

Become a Medium member
Strings are encoded as length:content (eg: 7:abhinav).

Lists are encoded as l<content>e. For example, [“a”, “b”, “c”] is bencoded into l1:a1:b1:ce.

Dictionaries are encoded as d<content>e. For example, {“bar”: “spam”, “foo”: 42} is bencoded into d3:bar4:spam3:fooe

Now let’s talk about the tracker which is the MOST important part of the .torrent file. After all, the tracker is the one that instructs where to download the file contents. If you’ve downloaded movies using uTorrent, you would’ve seen a tracker section on the bottom that displays multiple URLs. These are all centralized HTTP servers that contain information about the peers that have parts or the file as a whole. It also tracks who all are downloading at that same time and also helps find other peers to download content from.

Press enter or click to view image in full size

So here is a simple overview of how the architecture looks like:-

Press enter or click to view image in full size

Beautiful right?

Choke Algorithm

How do you maximize the download speed without any central organization? How do you stop a peer from abusing the protocol without a central organization? These are the problems that this algorithm solves.

There were and still are peers who simply just downloaded large files without sharing any piece with the other peers. This allows the peer to selfishly benefit from the protocol without giving anything in return. These people are called free riders. In such cases, the choke algorithm is implemented. Suppose peer B is the one abusing the protocol by downloading from peer A without uploading anything. In this case, peer A chokes peer B, blocking peer B from downloading anything from peer A but peer A still has the power to download from peer B. Doing this, the network traffic relaxes, and it makes the game more fair for every participant of the network.

Now how does it unchoke? Well, it depends on whether it’s a seeder or a leecher. Remember we discussed this earlier? Seeder is the one that uploads, leecher is the one that downloads. So a leecher unchokes the top 3 peers ranked on the basis of their uploading speed. And after some time these are choked again. Here there is a concept of optimistic unchoke where the leecher unchokes a 4th random peer. This way, the peers who are just beginning to participate in the network (i.e. they have nothing to upload) also get a chance to participate. Maybe they can even provide some pieces to their peers.

A seeder unchokes the more recently unchoked peers, this way none of the free riders are unchoked since they will never be the most recently unchoked peers and they heavily rely on the optimistic unchoke done by the leechers. Now if it encounters 2 peers that were unchoked at the same time, the one with the higher download rate is given more priority.

Here’s a small pseudo code of what the choke algorithm would look like.

function ChokeAlgorithm():
    while true:
        // Choose which peers to unchoke and choke
        selectPeersToUnchoke()  // Select a subset of peers to unchoke
        chokeAllOtherPeers()     // Choke all peers not in the unchoke subset

        // Wait for a short interval before repeating the process
        wait(interval)

function selectPeersToUnchoke():
    // Determine which peers to unchoke based on criteria such as download/upload speed,
    // connection quality, and reciprocity (uploading back to us)
    sortPeersByUploadSpeed()
    unchokeTop3Peers()

function unchokeTop3Peers():
    // Unchoke the top 3 peers
    for each peer in top3Peers:
        unchoke(peer)

function chokeAllOtherPeers():
    // Choke all peers except those that were selected to be unchoked
    for each peer in allPeers:
        if peer not in unchokedPeers:
            choke(peer)

function choke(peer):
    // Send a choke message to the specified peer, indicating that we will not accept
    // incoming requests for downloading from them
    sendChokeMessage(peer)

function unchoke(peer):
    // Send an unchoke message to the specified peer, indicating that we will accept
    // incoming requests for downloading from them
    sendUnchokeMessage(peer)
Piece Selection Algorithm
How does the BitTorrent protocol manage so many downloads and ensure its speed? We now know what the tracker does and how the pieces are transferred from p2p, but in what order do the pieces get transferred? Is it in a sequential order like piece0, piece1, piece2? If it’s like that, then what if the seeder that’s seeding these pieces suddenly dips after piece3? How will the rest of the peers get piece4? So we need an algorithm that somehow distributes piece4 to some random peer so that it we don’t have to worry later about whether the seeder shuts down after piece3 or pieceX.

The way BitTorrent does this is, it gives the highest priority to the rarest piece of the file. It distributes the rarest piece of the file with a selected number of peers and then makes sure to transfer the rest of the pieces with as many peers as possible so that it doesn’t matter whether the seeder is active or not. Thus, we’re minimizing the dependency on the seeder for downloading the file and encouraging more P2P connections.

Now how does a peer know which pieces another peer has? I’d actually mentioned this earlier. When a peer downloads a piece completely, it sends a “have” message to all its peers. This way, every other peer internally keeps a record of which peer keeps what piece. Another method is, when 2 peers “handshake” which is also a type of message (we will look into this in detail in Part 2), the next message will be the Bitfield message. It’s basically an array of bits or 1s and 0s, where 1 is written on the pieces it has and 0s on the pieces it doesn’t have. After exchanging the bitfield messages, the peer computes the rarest piece and prioritizes the downloading of that piece first. Now in case the peer that’s sending the bitfield message has only 4 1’s, which means it has only downloaded 4 pieces till now, then it will go for a random piece instead of the rarest piece.

NOTE: A peer will always prioritize the downloading of all the blocks in the piece before moving on to the next piece.

Kademlia Distributed Hash Tab (DHT) Algorithm
A Distributed Hash Table is a distributed system for mapping keys to their respective values. This is a fundamental concept that is being used in IPFS, Blockchain, Gnutella, Napster, and even BitTorrent. It’s similar to a hash table, but here the data is distributed among many nodes and they all combine to form a single distributed hash table. The use of this is to locate the owner of a key. In the BitTorrent protocol, we want to locate the node which owns a key. Sounds familiar? Yes, isn’t that kinda what the tracker did? Yes, historically, the BitTorrent primarily relied on trackers for locating peers which is a very centralized approach. What if an attacker changes the contents of the tracker? What if the attacker shuts down the tracker? This is why most modern BitTorrent clients support DHT’s and Kademlia is one specific algorithm used in the implementation of DHT’s.

In Kademlia, each node is assigned a 160-bit (20-byte) unique ID. These IDs are usually generated using a SHA-1 hash. The reason we are using the Kademlia algorithm is to locate the node that has the key we are looking for. “Key” here refers to the info hash that is stored in the .torrent file. Let’s take a scenario.

N1 => KB

N2 => KA

N3 => KD

N4 => KC

We want to find KC, so we randomly choose a node N1 out of lots of nodes and ask if it has KC, and it says “Nope, let me check which of my friends is closest to KC”. These friends are one Node from each sub-tree which is not related to the tree N1 is part of. The way Kademlia measures the distance between (NX, KC) and (N1, KC) is by doing the XOR operation between (NX & KC) and (N1 & KC). If NX ⊕ KC < N1 ⊕ KC, then we move to NX. This is the magic of Kademlia. Routing is handled perfectly, and whenever we move from one node to another, we are guaranteed that we are moving down the right path and will eventually reach the node that owns KC. This is something that needs an article of its own to understand more in-depth, maybe I can write it sometime in the future.

Conclusion
Well, that’s it. If you’ve reached till here, here is a medal for you! I suggest you read through this article once more and google more information on the side on Google/ChatGPT/YouTube. Overall, I’ve explained whatever I intended to in this article i.e. the Engineering behind the BitTorrent protocol. Even though it’s such an amazing thing, it still has its faults (like it still uses SHA-1 hash, it’s prone to viruses and other attacks, etc.) which can be a completely different 10-minute article. Anyway, thanks for reading, and happy researching!