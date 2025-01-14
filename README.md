# etracker: Equilibrium Tracker

`etracker` is an experimental Bittorrent tracker which uses error messages and
peer selection algorithms to promote healthy Bittorrent swarms.

Feature goals:

- Implemented: Track good seeders in the Bittorrent swarm, as measured by
  metrics like seeding sizes, length, and ratio.
- Implemented: When a peer starts to leech a torrent, preferentially give good
  seeders as peers when the client is itself classified as a good seeder.
- When abnormalities are detected in a swarm (broken clients, cheating
  clients), use the tracker warning or error message to communicate with peers
  which IP's they should block.

The intended upshot of these features is to encourage healthy swarms with
long-term seeding, without the use of a required minimum ratio.

This tracker is in a very early stage of development and none of these features
are currently implemented. Pre-alpha code is being shared for transparency in
preliminary benchmarking.

`etracker` is distributed without copyright under the Unlicense:
https://unlicense.org/.

# Setup and Installation

`etracker` is currently configured with environment variables, although it is
intended to switch to TOML in the future. You will need an `.env` file of the
following sort:

```bash
# Required variables
POSTGRES_USER=""
POSTGRES_PASSWORD=""
DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@localhost:5432"
PGDATABASE=""

# Optional variables

# Admin API to add or remove allowed infohashes using the /api endpoint. If
# left blank or not set, the API is disabled.
ETRACKER_AUTHORIZATION=""

# Relative or absolute paths to a TLS cert and key. If these are both set,
# the TLS tracker will automatically be started in addition to the unencrypted
# tracker.
ETRACKER_CERTFILE=""
ETRACKER_KEYFILE=""
```

`etracker` requires a PostgreSQL database. Distributed in the source is an
example compose file in the `docker` directory. Two databases need to be
created by hand: `$PGDATABASE` and `$PGDATABASE_test`

# Technical Discussion: Free-Riding

It is well-known that Bittorrent suffers from a potential free-riding problem:
Clients which only leech and do not seed lead to uploading bandwidth falling
disproportionately on a subset of the swarm which are willing to use their
bandwidth for seeding without any compensation. While there is nothing wrong
per se with a protocol which reflects revealed preferences in this way, there
are also reasons to do more to actively reward long-term seeding.

It is generally assumed that this is a weakness built in to the Bittorrent
protocol. Some private communities get around it by imposing out-of-protocol
restrictions on access to their torrent files: Only users with a certain ratio
of upload to download, or who meet minimum seeding time requirements, are
allowed to download torrent files from their HTTP servers.

However, the semi-official wiki which discusses the Bittorrent spec states the
following about the peer list distributed by the tracker:

> As mentioned above, the list of peers is length 50 by default. If there are
> fewer peers in the torrent, then the list will be smaller. Otherwise, the
> tracker randomly selects peers to include in the response. *The tracker may
> choose to implement a more intelligent mechanism for peer selection when
> responding to a request. For instance, reporting seeds to other seeders could
> be avoided.*

The purpose of `etracker` is to experiment with more aggressive "intelligent"
mechanisms for peer distribution, designed to incentivize long-term seeding. 

Using both an infohash table and a client table, similar to private trackers,
the tracker will store both the current list of seeded torrents, and which
clients are seeding which torrents, with which upload and download stats.
Tentatively, clients will be identified by their peer ID's. These are generally
unique, but not required to be unique by the Bittorrent spec. (If too many are
non-unique, preference will be given to clients which do offer unique peer
ID's, and it may be necessary to introduce an optional, semi-private announce
URL.)

The client table will provide the resources to calculate a score for each
client, which is a function of its recent seeding size, its average seeding
sizes, and the length of time seeding.

Peer lists will then be distributed using a backing off algorithm: Initial
announces will include peers from a leecher's current client tier or lower; future
announces will include peers from successively higher tiers. The result is that
high-tier seeders will get the fastest and most available seeds when leeching;
low-tier seeders will only eventually get the fastest and most available seeds
when leeching. This preserves the goal of distributing content via the
distributed Bittorrent network, but incentivizes long-term seeding by giving
temporary (and decreasing over time) speed advantages to long-term seeders. It
does this simply at the tracker level, rather than requiring the tracker to be
coupled with an enforcing frontend, as in some private communities.

# Technical Discussion: Problematic Clients

A further issue faced by Bittorrent trackers are problematic clients in a
swarm. This can take two forms: First, broken clients may re-request the same
pieces over and over again, wasting the bandwidth of seeders in the swarm. All
trackers face this problem. Second, cheating clients may misreport their
statistics to the tracker. Cheating clients are not an issue on traditional
Bittorrent trackers which do not track statistics; however, they are an issue
on any tracker that does track statistics, such as in private communities, or
a tiering algorithm like `etracker`.

Although to my knowledge, they have not been seriously pursued, there are
resources available at the tracker level to handle these problems.

First, there is detection at the tracker level: A tracker which measures
statistics for each torrent infohash can detect *imbalances* in the swarm
between upload and download amounts. These imbalances can be correlated with
specific clients by tracking abnormal values in the `event` key, or abnormal
gaps between the values in the `left` and `downloaded` keys, and
cross-referencing this with the start of the imbalance.

Second, there is intervention at the tracker level: Since Bittorrent is a
peer-to-peer protocol, there is no way for the tracker to terminate an existing
connection between two peers which is detected to be abnormal. However, the
tracker can provide clients with information on how to resolve this issue by
giving a `failure reason` or `warning message`. The `failure reason` will
produce a client error, which the client can then handle. The `warning message`
is optional and more research needs to be done to determine whether clients
implement this.

Taken together, `etracker` can do the following: Detect a broken client in the
swarm, and temporarily error the torrents of all other clients which received
that broken client as a peer, with a message telling them to block the
problematic client's IP in their client. At present, this still requires manual
intervention by the user to avoid wasting their own bandwidth, but it may be
something which clients could eventually handle automatically.

# Further Discussion

Blog posts about the development of this tracker will be shared here:
https://moerner.com.

- Benchmarking bencode implementations: https://moerner.com/posts/isnt-go-reflection-slow/
- Constructing PostgreSQL queries with non-parameterizable placeholders: https://moerner.com/posts/postgresql-parameter-placeholders/
- Brainstorming peer distribution algorithms: https://moerner.com/posts/brainstorming-peer-distribution-algorithms/

# Further Resources

- The BitTorrent Protocol Specification: https://www.bittorrent.org/beps/bep_0003.html
- BitTorrent Protocol Specification v1.0: https://wiki.theory.org/BitTorrentSpecification
