#!/usr/bin/env bash
# tests/seed.sh — empties the database and builds the fixture cast the tests run against.
#
# The tests used to append to whatever the previous runs had left behind, which is why every
# fixture carried a run suffix. The database is rebuilt here instead, so a name is just a name:
# every run starts from the same rows and an assertion can compare a literal.
#
# The server must already be listening: the cast is created through the real endpoints, so
# doLogin is what writes a user and this script never inserts one by hand.
# It does not need a restart — the schema is untouched, only the rows go.
set -u
cd /Users/lorenzo/WASAText || exit 1
H=localhost:3000
DB=db/wasatext.db
DEFAULT_PHOTO=00000000-0000-4000-8000-000000000000

id() { sed 's/.*"id":"\([^"]*\)".*/\1/'; }
login() { curl -s -X POST "$H/session" -H 'Content-Type: application/json' -d "{\"username\":\"$1\"}" | id; }

# The server has to be up, or every row below silently fails to be written
if ! curl -s -o /dev/null --max-time 2 "$H/liveness"; then
	echo "seed: nothing is listening on $H — start 'go run ./cmd/webapi/' first" >&2
	exit 1
fi

# ...and it has to be writing into the file this script reads. A server that finds :3000 taken exits
# with 'bind: address already in use' while the previous one keeps answering, and if the database was
# deleted meanwhile that one still holds the old, now unlinked, file: it answers every request happily
# while sqlite3 here reads a different database entirely, and every count below silently disagrees
# with what the endpoints see. One write through the API, read back with the CLI, settles it
PROBE="probe-$$"
curl -s -o /dev/null -X POST "$H/session" -H 'Content-Type: application/json' -d "{\"username\":\"$PROBE\"}"
if [ "$(sqlite3 "$DB" "SELECT COUNT(*) FROM users WHERE username='$PROBE';")" != "1" ]; then
	echo "seed: the server on $H is not writing into $DB — an older instance is still bound to the port" >&2
	echo "      stop every webapi process, wait for :3000 to be free, and start one again" >&2
	exit 1
fi
sqlite3 "$DB" "DELETE FROM users WHERE username='$PROBE';"

# ---------- reset ----------
# foreign_keys is a per-connection pragma and the sqlite3 CLI does not inherit the DSN the server opens
# (cmd/webapi/main.go passes _foreign_keys=on): without it a DELETE cascades nothing and strands rows
sqlite3 "$DB" "PRAGMA foreign_keys=ON;
	-- commentMessage, uncommentMessage and getMessageComments build their own reaction lifecycle after this clean slate
	DELETE FROM comments;
	DELETE FROM messages;
	DELETE FROM chat_members;
	DELETE FROM chats;
	DELETE FROM users;" || exit 1

# Every photo of the past runs goes, and the one committed in the repo stays:
# it is the default every new user shows, so the tests can count on it being there
find db/photos -type f ! -name "$DEFAULT_PHOTO" -delete

# ---------- cast ----------
# The four the sections are written around: dave belongs to nothing and is the outsider through §18
for u in alice bob carl dave; do login "$u" > /dev/null; done

# §3 reads these two together: '_' is a LIKE wildcard and a legal username character,
# so a prefix of 'a_' must find a_b and never axb
login a_b > /dev/null
login axb > /dev/null

# §10 fills a group to schemas.GroupMaxMembers, and addToGroup checks UsersExist before it counts,
# so all of them have to be real users and not just well formed ids
# §3 reads the same 99 to check the LIMIT 20 of the search: one cast, two uses
for i in $(seq -w 1 99); do login "fill$i" > /dev/null; done

# ---------- report ----------
sqlite3 "$DB" "SELECT 'users: '||(SELECT COUNT(*) FROM users)
			   ||'  chats: '||(SELECT COUNT(*) FROM chats)
			   ||'  members: '||(SELECT COUNT(*) FROM chat_members)
			   ||'  messages: '||(SELECT COUNT(*) FROM messages)
			   ||'  comments: '||(SELECT COUNT(*) FROM comments)
			   ||'  photos: $(ls db/photos | wc -l | tr -d " ")';"
