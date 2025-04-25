# etracker: Equilibrium Tracker

`etracker` is an experimental BitTorrent tracker which uses error messages and
peer selection algorithms to promote healthy BitTorrent swarms. You can think
of it as a hybrid of a public and private tracker: Like a public tracker, it is
accessible to everyone. But like a private tracker, users are incentivized to
maintain good seeding statistics (as measured by percentage of snatches seeded
and overall ratio). Unlike a private tracker, this is enforced at the tracker
level by distributing fewer peers to clients with a history of poor behavior.

A public demo (with a restricted infohash allowlist) is currently available:
https://etracker.moerner.com/

Features & goals:

- Implemented: Track good seeders in the BitTorrent swarm, as measured by
  ratio and percentage of snatches seeded.
- Implemented: When a peer starts to leech a torrent, return a list of peers
  which is scaled as a function of the peer's historical performance.
- Feature Goal: When abnormalities are detected in a swarm (broken clients, cheating
  clients), use the tracker warning or error message to communicate with peers
  which IP's they should block.

The intended upshot of these features is to encourage healthy swarms with
long-term seeding, without the use of a required minimum ratio.

`etracker` is implemented with a Go backend, using Postgres and Redis, and a
React frontend.

`etracker` is distributed without copyright under the Unlicense:
https://unlicense.org/.

# Setup and Installation

`etracker` is designed to be deployed with Docker. The included "docker" directory includes a Dockerfile to build a Docker image. Check out the docker file in `compose.yml` for configuration options. For example, you can build and run it with a command like the following:

```bash
$ export PGUSER="etracker_pguser"
$ export PGPASSWORD="etracker_secretpgpassword"
$ export ETRACKER_AUTHORIZATION="mysupersecretapikey"
$ export ETRACKER_FRONTEND_HOSTNAME="example.com"
$ export ETRACKER_REDIS="etracker_secretredispassword"
$ docker build . -t user/etracker
$ docker compose up -d etracker
```

By default this will start the tracker on `localhost:3000`; you can adjust the
port by setting `$ETRACKER_BACKEND_PORT`. `etracker`'s frontend should be deployed behind a
reverse proxy like
[Caddy](https://caddyserver.com/docs/quick-starts/reverse-proxy) to handle TLS.

By default, `etracker` uses an allowlist for infohashes. You may turn this off by setting the environmental variable `$ETRACKER_DISABLE_ALLOWLIST` to "true". At this time, infohashes can only be added by inserting them into the infohashes table directly, or by making an appropriate POST request to the `/api/infohash` endpoint, with the correct API key in the Authorization header. The API key is set via the environmental variable `$ETRACKER_AUTHORIZATION`. The `scripts/add_infohash.py` script will calculate the infohash of a local torrent file and add it to the allowlist. For example:

```bash
$ python3 scripts/add_infohash.py http://localhost:3000 "$ETRACKER_AUTHORIZATION" torrent_file.torrent
```

`etracker` is written in Go and includes a test suite. You can run the test suite by installing Go and Docker (for the testing database), and then run:

```
$ make test
```

# Technical Discussion: Free-Riding

It is well-known that BitTorrent suffers from a potential free-riding problem:
Clients which only leech and do not seed lead to uploading bandwidth falling
disproportionately on a subset of the swarm which are willing to use their
bandwidth for seeding without any compensation. While there is nothing wrong
per se with a protocol which reflects revealed preferences in this way, there
are also reasons to do more to actively reward long-term seeding.

It is generally assumed that this is a weakness built in to the BitTorrent
protocol. Some private communities get around it by imposing out-of-protocol
restrictions on access to their torrent files: Only users with a certain ratio
of upload to download, or who meet minimum seeding time requirements, are
allowed to download torrent files from their HTTP servers.

However, the semi-official wiki which discusses the BitTorrent spec states the
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
unique, but not required to be unique by the BitTorrent spec. (If too many are
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
distributed BitTorrent network, but incentivizes long-term seeding by giving
temporary (and decreasing over time) speed advantages to long-term seeders. It
does this simply at the tracker level, rather than requiring the tracker to be
coupled with an enforcing frontend, as in some private communities.

# Technical Discussion: Problematic Clients

A further issue faced by BitTorrent trackers are problematic clients in a
swarm. This can take two forms: First, broken clients may re-request the same
pieces over and over again, wasting the bandwidth of seeders in the swarm. All
trackers face this problem. Second, cheating clients may misreport their
statistics to the tracker. Cheating clients are not an issue on traditional
BitTorrent trackers which do not track statistics; however, they are an issue
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

Second, there is intervention at the tracker level: Since BitTorrent is a
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

- Blog post on public deployment: https://moerner.com/posts/etracker-public-demo/
- Slides from a presentation on `etracker` at the Recurse Center: https://moerner.com/static/presentation-etracker.pdf
- Benchmarking bencode implementations: https://moerner.com/posts/isnt-go-reflection-slow/
- Constructing PostgreSQL queries with non-parameterizable placeholders: https://moerner.com/posts/postgresql-parameter-placeholders/
- Brainstorming peer distribution algorithms: https://moerner.com/posts/brainstorming-peer-distribution-algorithms/

# Further Resources

- The BitTorrent Protocol Specification: https://www.bittorrent.org/beps/bep_0003.html
- BitTorrent Protocol Specification v1.0: https://wiki.theory.org/BitTorrentSpecification
