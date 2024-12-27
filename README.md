# stillbox

*"When they say stillbox, you know it's real."*

A Golang scanner call server and Angular frontend. Basically a rewrite of [rdio-scanner](https://github.com/chuot/rdio-scanner)
with a cleaner *backend* (not frontend, so far; I am not a frontend guy and it shows) and a more opinionated featureset.

Primary differences:
[x] No directory watch source, for now
[x] Only supports Postgres DB right now. May add SQLite someday.
[x] Filter calls by duration in search
[x] Both REST and WebSocket APIs
[x] Uses protobuf instead of JSON for the WebSocket API (no slices of integers for call audio!)
[x] Talkgroup activity alerting
[x] "Native" flutter client (Calls) for Android/iOS/macOS/Linux/Windows (in progress)
[x] RadioReference talkgroup import, SDRtrunk talkgroup playlist export
[x] Incidents functionality (group past calls together and retain them)
[x] No premiumization nags
[x] 3-clause BSD license

# **NOTE**

If this message is still here, the database schema *initial migration* and protobuf definitions are **still subject to change**.

Once `stillbox` is actually usable (but not necessarily feature-complete), I will remove this note, and start using DB migrations and
protobuf best practices (i.e. not changing field numbers).

# License and Copyright

© 2024, Daniel Ponte <dan AT dynatron DOT me>. Licensed under the 3-clause BSD license. See LICENSE for details.
