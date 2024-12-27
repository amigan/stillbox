# stillbox

*"When they say Stillbox, you know it's real."*

A Golang scanner call server and Angular frontend. Basically a rewrite of [rdio-scanner](https://github.com/chuot/rdio-scanner)
with a cleaner *backend* (not frontend, so far; I am not a frontend guy and it shows) and a more opinionated featureset.

Primary differences:
* No directory watch source, for now
* Only supports Postgres DB right now. May add SQLite someday.
* Filter calls by duration in search
* Both REST and WebSocket APIs
* Uses protobuf instead of JSON for the WebSocket API (no slices of integers for call audio!)
* Talkgroup activity alerting
* "Native" flutter client (Calls) for Android/iOS/macOS/Linux/Windows (in progress)
* RadioReference talkgroup import, SDRtrunk talkgroup playlist export
* Incidents functionality (group past calls together and retain them)
* 3-clause BSD license

**NOTE**

If this message is still here, the database schema *initial migration* and protobuf definitions are **still subject to change**.

Once `stillbox` is actually usable (but not necessarily feature-complete), I will remove this note, and start using DB migrations and
protobuf best practices (i.e. not changing field numbers).
