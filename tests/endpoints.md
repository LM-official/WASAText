# WASAText — endpoint tests
Manual tests for the endpoints registered in `service/api/api-handler.go`:
`doLogin`, `setMyUserName`, `setMyPhoto`, `getUsers`, `getMyConversations`, `getConversation`, `sendMessage`, `forwardMessage`, `getPhoto`, `createPrivateChat`, `createGroup`, `setGroupName`, `setGroupPhoto`, `addToGroup`, `leaveGroup`.

Last full run: 2026-08-30, every implemented section below re-checked against a freshly built binary: 435 assertions, all green — 357 from §0–16 and 78 from the §17 scenarios. The invariants held afterwards — no chat without members, no private chat holding a name or a photo of its own, no group missing one, no membership pointing at a row that is gone, and every referenced photo id present on disk with nothing left over.

`forwardMessage` is included in that full run. Its endpoint checks cover text/photo copying, source preservation, source/destination authorization, same-chat and group forwarding, validation and error precedence, `read`/`received` state, exact sender catch-up, comment isolation, rollback of refused writes, and the 10000-message cap. Scenario S16 checks that the copy becomes the destination's message and preview without changing the source.

Two scripts run all of it, so a section does not have to be pasted by hand:
```shell
bash tests/seed.sh            # empties the database and rebuilds the cast
bash tests/run-tests.sh       # §0-16, 357 assertions (calls seed.sh itself)
bash tests/run-scenarios.sh   # §17 S1-S16, 78 assertions
```
They need the server already listening (see Run below). `run-tests.sh` starts by calling `tests/seed.sh`, which empties every table and rebuilds the cast, so a run never depends on what the one before it left behind and a fixture is just a name: `alice`, not `alice1756304412`. `run-scenarios.sh` builds its own users on top of whatever is there and still suffixes them, since it does not reset anything itself.

Bind the port before trusting a run. A server that finds `:3000` taken exits with `bind: address already in use` while the previous one keeps answering, so requests reach one database and the `sqlite3` commands below reach another. Check the log for `API listening` before the first request.

## Run
```shell
go run ./cmd/webapi/          # another terminal
bash tests/seed.sh            # empties the tables and rebuilds the cast
```

`service/database/database.go` creates `./db/wasatext.db` and every table on startup (`CREATE TABLE IF NOT EXISTS`), `service/photos/photos.go` creates `./db/photos`. The file appears at the first run of the server, never at build time: `go build` writes no database, and an empty `db/` before the first `go run ./cmd/webapi/` is normal, not a failure.

`tests/seed.sh` empties every table and builds the cast below through the real endpoints, so the tests start from the same rows every time. It needs no restart: it deletes rows, it does not touch the schema. It runs `PRAGMA foreign_keys=ON` first, because that pragma is per connection and the `sqlite3` CLI does not inherit the DSN the server opens with — without it a `DELETE` cascades nothing and strands rows.

A schema change is the one thing the seed cannot do for you. `CREATE TABLE IF NOT EXISTS` does not alter a table that already exists, so a new column or constraint needs the server stopped and the file deleted:
```shell
rm ./db/wasatext.db
find db/photos -type f ! -name 00000000-0000-4000-8000-000000000000 -delete
```
This fails silently and in the unsafe direction, so it is worth checking that the schema is the one you wrote:
```shell
sqlite3 db/wasatext.db "SELECT name, \"notnull\" FROM pragma_table_info('chat_members');"   # lastReadDate -> 1
```

## Fixtures
The cast `tests/seed.sh` writes. Names are literal: the database is empty at the start of every run, so nothing needs a suffix to stay out of the way of the run before it.

| fixture | what it is for |
|---|---|
| `alice`, `bob`, `carl` | the members every section below is written around |
| `dave` | belongs to nothing: the outsider of the `403` cases in §8–§11 and §13–§15 |
| `a_b`, `axb` | §3, LIKE escaping: `_` is a wildcard and a legal username character |
| `fill01`…`fill99` | §10 fills a group to `GroupMaxMembers`, and `addToGroup` checks `UsersExist` before it counts, so they have to be real. §3 reads the same 99 for its `LIMIT 20` |

```shell
id() { sed 's/.*"id":"\([^"]*\)".*/\1/'; }      # extracts the id from a reply (or use jq -r .id)

A=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d '{"username":"alice"}' | id)
B=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d '{"username":"bob"}'   | id)
echo "A=$A B=$B"                                # the id is both the user id and the bearer token
                                                # doLogin is idempotent, so this reads the seeded ids

PNG=db/photos/00000000-0000-4000-8000-000000000000    # a real PNG already in the repo
printf 'not an image' > /tmp/text.txt
: > /tmp/empty.png
```

## Conventions
- Every failure has the body of `schemas.Error`: `{"code":<status>,"message":"<reason>"}`. The technical cause (broken rule, SQLite message) is only in the server log.
- `curl -i` to read status and headers, `-s` when only the body matters.
- Any endpoint may answer `500` if the database or the filesystem fails; the cases below are the ones a client can trigger.

---

## 0. Authentication — every endpoint but `doLogin`
`service/api/api-authenticate.go` runs before the handler, so these answers are the same on `/me/username`, `/me/photo`, `/me/chats`, `/chats/:chatId`, `/chats/:chatId/messages`, `/chats/:chatId/messages/forwards`, `/users`, `/photos/:photoId`, `/private-chats`, `/groups`, `/groups/:groupId/name`, `/groups/:groupId/photo`, `/groups/:groupId/members` and `/groups/:groupId/members/me`, and nothing is read or written when they fire.

| Authorization header | reply |
|---|---|
| (absent) | `401 {"code":401,"message":"missing bearer token"}` |
| `bearer $A` (lowercase) | `401 missing bearer token` — the prefix is `Bearer ` exactly |
| `Bearer` (no token) | `401 missing bearer token` |
| `Bearer not-a-uuid` | `401 {"code":401,"message":"invalid token format"}` |
| `Bearer ${A^^}` (uppercase UUID) | `401 invalid token format` — only the canonical lowercase form is an `Id` |
| `Bearer {$A}` / unhyphenated | `401 invalid token format` — same reason |
| `Bearer 11111111-2222-4333-8444-555555555555` | `401 {"code":401,"message":"unknown token"}` |

```shell
curl -i localhost:3000/users?username=alice                                   # -> 401 missing bearer token
curl -i localhost:3000/users?username=alice -H "Authorization: Bearer nope"   # -> 401 invalid token format
```

---

## 1. `doLogin` — `POST /session`
The only endpoint without authentication (`security: []`).

```shell
curl -i -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"carl\"}"
```

| body | reply |
|---|---|
| `{"username":"carl"}` first time | `201 {"id":"<uuid>"}` — registered |
| `{"username":"carl"}` again | `200` and the same id — logged in |
| `{"usernam":"carl"}` (typo) | `400 invalid request body` — the field stays empty and breaks the length rule |
| `{"username":""}` / `{}` / `{"username":null}` | `400 invalid request body` |
| `{"username":"a b"}` / `" carl "` / `"cà rl"` | `400` — the pattern is `^[a-zA-Z0-9_.-]+$` |
| `{"username":"<30 chars>"}` | `201` |
| `{"username":"<31 chars>"}` | `400` |
| `{"username":` (truncated) or empty body | `400 invalid request body` |
| `{"username":"carl","admin":true}` | `200`/`201` — unknown fields are ignored, see §18 |

The returned id is the bearer token of every other test: this is the only point where `doLogin` and `authenticate` have to agree.

---

## 2. `setMyUserName` — `PATCH /me/username`
```shell
curl -i -X PATCH localhost:3000/me/username \
  -H "Authorization: Bearer $A" -H 'Content-Type: application/merge-patch+json' \
  -d "{\"username\":\"alice_new\"}"
```

| case | reply |
|---|---|
| free username | `200 {"id":"$A","username":"alice_new","photo":"/photos/00000000-0000-4000-8000-000000000000"}` |
| the username it already has | `200`, unchanged — updating a row to its own value is not a UNIQUE conflict |
| `"bob"` (owned by B) | `400 {"code":400,"message":"username already taken"}`, nothing written |
| `{"username":""}`, `{}`, `{"username":"a b"}`, 31 chars | `400 invalid request body` |
| `Content-Type: application/json` | identical result — the handler never reads the header (§18) |
| bad/absent token | §0, and the username is not touched |

The `photo` field is a URL and never the stored id: `service/api/photo-url.go` is what turns one into the other. A user that never uploaded shows the default id.

---

## 3. `getUsers` — `GET /users?username=<prefix>`
```shell
curl -i "localhost:3000/users?username=alice" -H "Authorization: Bearer $A"
```

| case | reply |
|---|---|
| prefix matching somebody | `200 {"users":[{"id":..,"username":..,"photo":"/photos/<uuid>"},..]}` |
| prefix matching nobody | `404 {"code":404,"message":"no user matches the given username"}` — an empty list is never a 200 |
| `?username=` or the parameter absent | `400 {"code":400,"message":"invalid username"}` |
| `?username=a b` (invalid chars) | `400 invalid username` |
| more than 20 matches | `200` with the first 20 (`LIMIT 20`) |
| own username with own token | `200`, and the caller is in the list — see §19 |

LIKE escaping. `_` and `%` are LIKE wildcards and `_` is a legal username character, so `service/database/get-users.go` escapes the prefix:
```shell
curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"a_b\"}"
curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"axb\"}"
curl -s "localhost:3000/users?username=a_" -H "Authorization: Bearer $A"
```

`a_b` must be in the list and `axb` must not.

The cap:
```shell
for i in $(seq 1 25); do curl -s -X POST localhost:3000/session \
  -H 'Content-Type: application/json' -d "{\"username\":\"lim_$i\"}" > /dev/null; done
curl -s "localhost:3000/users?username=lim" -H "Authorization: Bearer $A" | grep -o '"id"' | wc -l
```

25 users created, the count must be `20`.

---

## 4. `setMyPhoto` — `PATCH /me/photo` (multipart)
```shell
curl -i -X PATCH localhost:3000/me/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG"
```

| case | reply |
|---|---|
| a PNG / JPEG / GIF / WEBP | `200` and the User with a new `photo` URL |
| the same file again | `200` and a different URL — a photo is written once, a new upload is a new id |
| `-F "file=@$PNG"` (wrong field) | `400 {"code":400,"message":"missing photoFile field"}` |
| `-F "photoFile=@/tmp/text.txt"` | `400 {"code":400,"message":"invalid photo"}` — the type comes from the bytes, not from the name |
| `-F "photoFile=@/tmp/empty.png"` | `400 invalid photo` — empty upload |
| `-d '{"photoFile":"x"}'` (not multipart) | `400 {"code":400,"message":"invalid multipart body"}` |
| bad/absent token | §0, and nothing is written to `./db/photos` |

The two size guards (`MaxPhotoBytes` = 31457280, body limit = that + 1 MiB = 32505856):
```shell
{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=512k count=61 2>/dev/null; } > /tmp/big.png   # 30.5 MiB
curl -i -X PATCH localhost:3000/me/photo -H "Authorization: Bearer $A" -F "photoFile=@/tmp/big.png"
# -> 400 invalid photo          (over MaxPhotoBytes, caught by photos.Save reading limit+1)

{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=1m count=33 2>/dev/null; } > /tmp/huge.png    # 33 MiB
curl -i -X PATCH localhost:3000/me/photo -H "Authorization: Bearer $A" -F "photoFile=@/tmp/huge.png"
# -> 400 invalid multipart body (over the body limit, caught by MaxBytesReader before the parser)
```

No leaked files. Every refused upload must leave the directory as it was:
```shell
ls db/photos | wc -l    # before and after each 400 above: same number
```

Garbage collection. Upload twice and the first file is gone, because nothing points at it any more; the default photo is never deleted even when no user shows it:
```shell
P1=$(curl -s -X PATCH localhost:3000/me/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
curl -s -X PATCH localhost:3000/me/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG" > /dev/null
ls db/photos/$P1                                    # -> No such file
ls db/photos/00000000-0000-4000-8000-000000000000   # -> still there
```

---

## 5. `getPhoto` — `GET /photos/{photoId}`
```shell
curl -i localhost:3000/photos/00000000-0000-4000-8000-000000000000 -H "Authorization: Bearer $A" -o /dev/null
```

| case | reply |
|---|---|
| an existing id | `200`, `Content-Type: image/png`, `X-Content-Type-Options: nosniff`, `Cache-Control: private, max-age=31536000, immutable` |
| `/photos/not-a-uuid` | `400 {"code":400,"message":"invalid photo id"}` |
| `/photos/..` or `/photos/a.b` | `400 invalid photo id` — an id is a canonical UUID, so it can hold no `.` |
| `/photos/../../etc/passwd`, raw or percent-encoded | `404`, plain text — the extra segments match no route, so the router answers before the handler (§16) |
| a valid UUID with no file | `404 {"code":404,"message":"photo not found"}` |
| no token | `401` — which is why the frontend cannot use a plain `<img src>` and has to fetch the bytes |

`http.ServeContent` adds the rest:
```shell
curl -i -H "Range: bytes=0-9" -H "Authorization: Bearer $A" localhost:3000/photos/000...000 -o /dev/null
```

The type is the one detected from the bytes: a file renamed `.png` that is really a JPEG is served as `image/jpeg`, and nothing that is not an image can be stored in the first place (§4).

---

## 6. `createPrivateChat` — `POST /private-chats`
```shell
curl -i -X POST localhost:3000/private-chats -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"id\":\"$B\"}"
```

| body / caller | reply |
|---|---|
| A asks for B, first time | `201 {"id":"<chatId>"}` |
| A asks for B again | `200` and the same chatId |
| B asks for A | `200` and the same chatId — `pairKey` sorts the two ids, so the pair is one row either way |
| A asks for A | `400 {"code":400,"message":"cannot open a private chat with yourself"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"the other user does not exist"}` |
| `{"id":"not-a-uuid"}`, `{}`, malformed JSON | `400 invalid request body` |
| bad/absent token | §0 |

The 201→200 pair is the test that matters: it proves the `UNIQUE` on `pairKey` and not a read-then-write, so two identical requests can never open two chats.

---

## 7. `createGroup` — `POST /groups` (multipart: JSON part + file part)
```shell
curl -i -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" \
  -F "photoFile=@$PNG"
```

| case | reply |
|---|---|
| valid name + members + photo | `201 {"id":"<chatId>"}`, and `chat_members` holds `$A` and `$B`: the creator is added by the server |
| the same request again | `201` and a different id — the same people may share many groups (`pairKey` is NULL for a group) |
| `members` containing the caller | `400 {"code":400,"message":"the creator is already a member of the group"}` |
| `members` with a valid UUID owned by nobody | `404 {"code":404,"message":"one or more of the members does not exist"}` |
| `members: []` | `400 {"code":400,"message":"invalid data part"}` — at least one other member |
| `members` with 100 entries | `400 invalid data part` — at most 99, the creator takes the last place |
| `members: ["$B","$B"]` | `400 invalid data part` — duplicates are refused before SQL |
| `name: ""` or over 100 chars | `400 invalid data part` |
| no `data` part, or `data` not JSON | `400 invalid data part` |
| no `photoFile` part | `400 {"code":400,"message":"missing photoFile field"}` — the `chats` CHECK refuses a group without a photo |
| `photoFile=@/tmp/text.txt` | `400 invalid photo` |
| bad/absent token | §0 |

Order of the checks. The photo is read only after the data part and the members are accepted, so every 400/404 above must leave `ls db/photos | wc -l` unchanged.

---

## 8. `setGroupName` — `PATCH /groups/{groupId}/name`
Two more fixtures: a group to rename, and a user who is not one of its members.
```shell
C=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"carl\"}" | id)
G=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)

curl -i -X PATCH localhost:3000/groups/$G/name \
  -H "Authorization: Bearer $A" -H 'Content-Type: application/merge-patch+json' \
  -d '{"name":"CS Study Group"}'
```

| case | reply |
|---|---|
| a member renames | `200 {"id":"$G","chatType":"group","name":"CS Study Group","photo":"/photos/<uuid>"}` |
| the other member renames | `200` — every member may rename, a group has no owner |
| the name it already has | `200`, unchanged — a group name has no `UNIQUE`, so a rename is idempotent |
| exactly 100 chars | `200` |
| `{"name":"👨‍👩‍👧‍👦"}` | `200` — one grapheme cluster, `CountChars` counts 1 and not 11 runes |
| a caller who is not a member | `403 {"code":403,"message":"not a member of the group"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"group not found"}` |
| the id of a private chat | `404 group not found` — a private chat borrows its name and owns none to update |
| `/groups/not-a-uuid/name`, or the uppercase UUID | `400 {"code":400,"message":"invalid group id"}` |
| `{"name":""}`, `{"name":"   "}`, `{}`, `{"name":null}`, 101 chars | `400 invalid request body` |
| malformed or empty body | `400 invalid request body` |
| `Content-Type: application/json` | identical result — the handler never reads the header (§18) |
| `{"name":"x","admin":true}` | `200` — unknown fields are ignored (§18) |
| `GET` / `POST` on the same path | `405 Method Not Allowed` (§16) |
| bad/absent token | §0, and the name is not touched |

The reply is a `GroupChat`: the chat alone, since renaming a group reads neither its members nor its messages, so no preview is derived from them.

The `chatType` is not read back from the row. It is a condition of the write, so a returned row is always a group.

403 vs 404. `RETURNING` gives `sql.ErrNoRows` for both reasons, so one extra read tells them apart: if a group with that id exists, membership was the guard that failed. That read looks at `id` and `chatType` only — columns no endpoint ever changes — so a concurrent rename cannot make it answer the wrong one.

Nothing is written when a call is refused:
```shell
curl -s -X PATCH localhost:3000/groups/$G/name -H "Authorization: Bearer $C" \
  -H 'Content-Type: application/merge-patch+json' -d '{"name":"Hijacked"}'      # -> 403
sqlite3 db/wasatext.db "SELECT name FROM chats WHERE id='$G';"                  # still the previous name
sqlite3 db/wasatext.db "SELECT quote(name), quote(photoId) FROM chats WHERE id='$P';"  # NULL|NULL, the CHECK is never at risk
```

Concurrency. Two members renaming at once both succeed and the last writer wins, with no indication to the first.

---

## 9. `setGroupPhoto` — `PATCH /groups/{groupId}/photo` (multipart)
```shell
curl -i -X PATCH localhost:3000/groups/$G/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG"
```

| case | reply |
|---|---|
| a member uploads a PNG / JPEG / GIF / WEBP | `200 {"id":"$G","chatType":"group","name":"...","photo":"/photos/<new uuid>"}` |
| the same file again | `200` and a different URL — a photo is written once, a new upload is a new id |
| the other member uploads | `200` — every member may change the photo, a group has no owner |
| `-F "file=@$PNG"` (wrong field) | `400 {"code":400,"message":"missing photoFile field"}` |
| `-F "photoFile=@/tmp/text.txt"` | `400 {"code":400,"message":"invalid photo"}` — the type comes from the bytes |
| `-F "photoFile=@/tmp/empty.png"` | `400 invalid photo` — empty upload |
| `-d '{"photoFile":"x"}'` (not multipart) | `400 {"code":400,"message":"invalid multipart body"}` |
| over `MaxPhotoBytes` / over the body limit | `400 invalid photo` / `400 invalid multipart body`, exactly as §4 |
| a caller who is not a member | `403 {"code":403,"message":"not a member of the group"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"group not found"}` |
| the id of a private chat | `404 group not found` — a private chat borrows its photo and owns none to update |
| `/groups/not-a-uuid/photo`, or the uppercase UUID | `400 {"code":400,"message":"invalid group id"}` |
| `GET` / `POST` on the same path | `405 Method Not Allowed` (§16) |
| bad/absent token | §0, and nothing is written to `./db/photos` |

The reply is a `GroupChat`, like §8: replacing a photo reads neither the members nor the messages, so no preview is derived from them.

No leaked files, including the refusals that come after the upload. Who may change the photo is a condition of the `UPDATE` itself, so a `403` and a `404` are only known once the bytes are already on disk; the handler deletes them on every failing branch, not just on `500`:
```shell
ls db/photos | wc -l                                                    # before
curl -s -X PATCH localhost:3000/groups/$G/photo -H "Authorization: Bearer $C" -F "photoFile=@$PNG"   # -> 403
curl -s -X PATCH localhost:3000/groups/00000000-0000-4000-8000-000000000001/photo \
  -H "Authorization: Bearer $A" -F "photoFile=@$PNG"                    # -> 404
ls db/photos | wc -l                                                    # same number
sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';"       # still the previous photo
```

Garbage collection, as in §4: the replaced photo is dropped once nothing points at it, and the reply is written before the release, so a photo that survives a crash is garbage and never a failed request.
```shell
P1=$(curl -s -X PATCH localhost:3000/groups/$G/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
curl -s -X PATCH localhost:3000/groups/$G/photo -H "Authorization: Bearer $A" -F "photoFile=@$PNG" > /dev/null
ls db/photos/$P1                                    # -> No such file
```

403 vs 404, and why this one needs no extra read. `SetGroupPhoto` opens a transaction and reads the old `photoId` first, which is also the answer to "does a group own this id?": if that `SELECT` finds nothing it is a `404`, and the guarded `UPDATE` below is then left with the membership as the only condition that can fail, so `sql.ErrNoRows` there means `403`. §8 needs a second read after the failure because it has no such `SELECT` to reuse.

The transaction is what makes the release safe: reading the old id and writing the new one are one step, so a concurrent update cannot make one caller delete a file the other has just put in use.

---

## 10. `addToGroup` — `POST /groups/{groupId}/members`
```shell
curl -i -X POST localhost:3000/groups/$G/members \
  -H "Authorization: Bearer $A" -H 'Content-Type: application/json' -d "{\"members\":[\"$C\"]}"
```

| case | reply |
|---|---|
| a member adds one or more users | `200 {"id":"$G","chatType":"group","name":"...","photo":"/photos/<uuid>","members":[..]}` |
| the other member adds | `200` — every member may add, a group has no owner |
| a user already inside | `200`, and the list holds it once — `INSERT OR IGNORE` drops the repeated tuple |
| the caller itself | `200`, unchanged — the caller is a member by definition |
| `{"members":[]}` | `200` and the group as it stands — an empty list adds nobody |
| `{}` or `{"members":null}` | `200`, same as the empty list — the field is not `required` in the spec either |
| a caller who is not a member | `403 {"code":403,"message":"not a member of the group"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"group not found"}` |
| the id of a private chat | `404 group not found` — its two members are its pair and it takes no more |
| a member UUID owned by nobody | `404 {"code":404,"message":"one or more of the members does not exist"}` |
| additions that take the group past 100 | `400 {"code":400,"message":"the group is full"}` |
| `{"members":["nope"]}`, duplicate ids, 101 entries | `400 invalid request body` |
| malformed or empty body | `400 invalid request body` |
| `/groups/not-a-uuid/members`, or the uppercase UUID | `400 {"code":400,"message":"invalid group id"}` |
| `GET` / `PATCH` / `DELETE` on the same path | `405 Method Not Allowed` + `Allow: OPTIONS, POST` (§16) |
| bad/absent token | §0, and no membership is written |

The reply is a `GroupWithMembers`: the chat plus the whole member list, and no messages, so no preview is derived. The list is read after the `INSERT`, so its length is what the table holds and never what the request asked — adding 2 people to a group of 50 answers with 52.

An empty list still answers for the group. The existence and the membership are checked before the write is skipped, so adding nobody is never a free `200`:
```shell
curl -i -X POST localhost:3000/groups/$G/members -H "Authorization: Bearer $C" \
  -H 'Content-Type: application/json' -d '{"members":[]}'      # -> 403, not 200
```

The cap counts what the group gains, not what was asked. The count is taken after the insert, so members already inside cost nothing and a request that only repeats them is accepted at exactly 100; one that would pass 100 is rolled back whole and never in part:
```shell
# a group of 2, one request adding 99 different users
curl -s -X POST localhost:3000/groups/$G/members -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"members\":[<99 ids>]}"        # -> 400 the group is full
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G';"   # still 2
```

403 vs 404 in one read, unlike §8 and §9. The membership is a column of the reply and not a condition of the `WHERE`: no row means only "no group owns that id", and a row with a false `isMember` means "not a member". The other two put the membership inside the write, so they cannot tell the two apart without help.

A member that does not exist is a `404` and not a `500`. The handler reads `UsersExist` first, as §7 does: without it an unknown id would only break the foreign key of `chat_members`, and a bad request would answer with a server error.

---

## 11. `leaveGroup` — `DELETE /groups/{groupId}/members/me`
`me` is the caller and not a parameter: a member removes itself, and removing anybody else is not an operation this API has. A fourth fixture `D` is used below as the outsider.

```shell
D=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"dave\"}" | id)
G=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Leave \",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)

curl -i -X DELETE localhost:3000/groups/$G/members/me -H "Authorization: Bearer $A"
```

| case | reply |
|---|---|
| a member of three leaves | `204` and an empty body |
| the same caller leaves again | `403 {"code":403,"message":"not a member of the group"}` — it is no longer one of them |
| a caller who was never a member | `403 not a member of the group` |
| the last member leaves | `204`, and the group is gone (see below) |
| any call on a group already emptied | `404 {"code":404,"message":"group not found"}` — the row went with the last member |
| a valid UUID owned by nobody | `404 group not found` |
| the id of a private chat | `404 group not found` — its two members are its pair, and neither leaves it |
| `/groups/not-a-uuid/members/me`, or the uppercase UUID | `400 {"code":400,"message":"invalid group id"}` |
| `GET` / `POST` / `PATCH` / `PUT` on the same path | `405 Method Not Allowed` + `Allow: DELETE, OPTIONS` (§16) |
| `DELETE /groups/{groupId}/members` (without `/me`) | `405` — that path is `POST` only, and `POST` on it still answers `200`: the two routes do not collide |
| bad/absent token | §0, and no membership is removed |

The reply has no body, unlike every other endpoint of this API. What this call removes is one membership, whose whole content is the group of the path and the caller of the token: there is no value the caller does not already hold, so a `204` says everything a `200` could. Failures still carry the `Error` schema — it is success that is empty. 403 vs 404 still come from one read, for the same reason as §10.

The group is never observably empty, which is why no reply has to say it is. Removing the last membership and dropping the group row are one transaction, so a concurrent reader sees the group with at least one member or does not see it at all. `LeaveGroup` only asks `SELECT 1 ... LIMIT 1` after the `DELETE` — whether anybody is left, never who, and never how many — and that answer decides whether the row goes. The method gives back the group photo id and nothing else: the handler needs it for `releasePhoto` and sends none of it to the client.

The last member out drops the group: nobody can reach it again, so it goes with the caller, and its messages follow through the `ON DELETE CASCADE` of the `messages` table. No membership is left to cascade, the one this call removed being the last.
```shell
G2=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Drop \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
GP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G2';")

curl -s -X DELETE localhost:3000/groups/$G2/members/me -H "Authorization: Bearer $A"   # -> 204, empty body
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G2';"         # -> 1, B is still inside
ls db/photos/$GP                                                                       # -> still there
curl -s -X DELETE localhost:3000/groups/$G2/members/me -H "Authorization: Bearer $B"   # -> 204, the last one out
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats WHERE id='$G2';"                    # -> 0
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G2';"         # -> 0
ls db/photos/$GP                                                                       # -> No such file
```

Garbage collection, as in §4 and §9. `LeaveGroup` gives back a photo id only when the caller was the last one out: while the group stands its own row still points at that photo, so nothing could release it and `releasePhoto` is never reached. On the last leave it is reached and asks `PhotoIsReferenced`, so a photo another row still shows survives the group, and the default one is never collected.

Nothing is written when a call is refused: after a `403`, `SELECT COUNT(*) FROM chat_members WHERE chatId='$G'` and `ls db/photos | wc -l` are both what they were.

Concurrency. The read of the membership, its removal and the drop of the group are one transaction: without it the last two members leaving at once would each still find the other inside, and neither would drop the group. Run the two leaves with `&` and check no row is stranded:
```shell
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats c WHERE c.id='$G2'
  AND NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId=c.id);"       # -> 0
```

---

## 12. `getMyConversations` — `GET /me/chats`
The homepage list: every private chat and every group the caller belongs to, most recently active first, each with the preview of its last message. It is under `/me/` because the list is the caller's own and nobody else can address it.

Three fixtures: a group `G` holding A, B and C, a private chat `P` between A and B, and a group `G2` nobody has written in.
```shell
G=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Study \",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
P=$(curl -s -X POST localhost:3000/private-chats -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"id\":\"$B\"}" | id)
G2=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Silent \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)

curl -i localhost:3000/me/chats -H "Authorization: Bearer $A"
```

| case | reply |
|---|---|
| a caller in a group and a private chat | `200 {"chats":[{"id":..,"chatType":..,"name":..,"photo":..,"snippet":{..}},..]}` |
| a group | `name`/`photo` are the ones the group owns |
| a private chat | `name`/`photo` are the other member's, so one row reads differently for each of the two |
| a chat nobody has written in | no `snippet` field, and the chat sorts last |
| a caller in nothing, or who left every chat | `404 {"code":404,"message":"no conversations found"}` — an empty list is never a `200`, as in §3 |
| `POST` / `PATCH` / `DELETE` on the path | `405 Method Not Allowed` + `Allow: GET, OPTIONS` (§16) |
| `GET /me/chats/`, or `GET /chats` | `404` — no trailing slash, and the bare collection is not a route: only `/chats/{chatId}` is (§13) |
| bad/absent token | §0 |

The borrow is live, and a group is never multiplied. A private chat holds `NULL` in `name` and `photoId` (the `CHECK` requires it) and reads both from the other member, so no copy is left to go stale on a `setMyUserName`; the `chatType` inside that same join is what keeps a group of three from coming back three times:
```shell
curl -s localhost:3000/me/chats -H "Authorization: Bearer $B" | grep -o '"name":"[^"]*"'   # -> alice, the chat A reads as bob
curl -s -X PATCH localhost:3000/me/username -H "Authorization: Bearer $B" \
  -H 'Content-Type: application/merge-patch+json' -d "{\"username\":\"bob_renamed\"}" > /dev/null
curl -s localhost:3000/me/chats -H "Authorization: Bearer $A" | grep -o '"name":"[^"]*"'   # -> bob_renamed, with no write to chats
curl -s localhost:3000/me/chats -H "Authorization: Bearer $A" | grep -o "\"id\":\"$G\"" | wc -l   # -> 1
```

The snippet. The messages are the ones `sendMessage` writes (§14); only what the endpoint refuses is still put in by hand. Besides the content below, a snippet carries the `id`, `user`, `date` and `state` of the message it previews.
```shell
curl -s -X POST localhost:3000/chats/$G/messages -H "Authorization: Bearer $B" -F "text=$(printf 'abcdefghij%.0s' $(seq 1 12))"
curl -s -X POST localhost:3000/chats/$P/messages -H "Authorization: Bearer $B" -F "photoFile=@$PNG"
```

| the last message is | `content` |
|---|---|
| text only | `{"text":"<first 50 chars>"}` |
| photo only | `{"emoji":"📷"}` — a symbol where the opened chat carries the picture |
| text and photo | both fields |
| 120 chars of text | `text` is exactly 50 — `schemas.TruncateChars` |
| 60 `👨‍👩‍👧‍👦` | 50 of them and never 50 runes: the cut falls between two chars, so no half emoji comes back |
| an empty text, or spaces alone, with no photo | no `snippet` at all, and the chat stays in the list |
| spaces alone and a photo | `{"emoji":"📷"}` — the guard fires only when nothing is left |

Nothing to preview means no snippet. The `CHECK` keeps a message from holding neither text nor photo, but it counts an empty string as text and `TruncateChars` trims spaces alone down to nothing, so without the guard the reply would carry `"content":{}`, against `minProperties: 1` and `SnippetContent.IsValid()`. `sendMessage` cannot write such a row — it answers `400` to a text that counts zero and stores `NULL` rather than `''` — so the fixture for this one is inserted by hand, and the guard is what keeps a row already in a database from breaking a reply. Ordering is the query's and the guard is the handler's, so such a chat still sorts by that message: it did see activity, there is just nothing to show for it.

The state is computed and never stored. A message is `read` once no member of the chat is left behind it. A member falls behind when a message arrives, and catches up by opening the chat (§13) or by writing in it (§14), since writing means having seen what is above — which is why the sender needs no exception of its own. The state is a fact about the members and not about the caller, so B reads the same value as A:
```shell
# the group message above was sent by B; the other members are A and C
curl -s localhost:3000/chats/$G -H "Authorization: Bearer $A"   # A opens it -> still "state":"received", C has not
curl -s localhost:3000/chats/$G -H "Authorization: Bearer $C"   # C opens it
curl -s localhost:3000/me/chats -H "Authorization: Bearer $A"   # -> "state":"read"
```
Every read answers with the states taken before it marks the chat, so a call shows what was true when it was asked and the value it wrote shows on the next one.

Newest first, and the same order on every call. The sort key is the date of the last message; a chat with none has no date and SQLite sorts `NULL` last in `DESC`, so it lands at the bottom, and the chat id breaks that tie. Two messages can share a millisecond, and `rowid` breaks that one by insertion order. This pair stays written by hand so the tie is deterministic. They are dated ahead of every sent message so they stay the newest of that chat:
```shell
sqlite3 db/wasatext.db "INSERT INTO messages (id,chatId,userId,text,photoId,date) VALUES
 ('33333333-3333-4333-8333-333333333333','$P','$B','FIRST in that millisecond',NULL,'2026-08-24T15:00:00.000Z'),
 ('44444444-4444-4444-8444-444444444444','$P','$B','SECOND in that millisecond',NULL,'2026-08-24T15:00:00.000Z');"
curl -s localhost:3000/me/chats -H "Authorization: Bearer $A" | grep -o '"chatType":"[^"]*"'
# -> private (its snippet reads SECOND in that millisecond), group, then G2 which has no message at all
```

Nothing is written by this endpoint. It is the only read of a chat, so a call must leave `chats`, `chat_members`, `messages` and `db/photos` exactly as they were.

---

## 13. `getConversation` — `GET /chats/{chatId}`
The opened chat: the chat, its members and a page of its messages, newest first. It is the read §12 leads to, and the one route where a private chat id is not a `404`.

The fixtures are the ones of §12: the group `G` holding A, B and C, the private chat `P` between A and B, the silent group `GS`, and `D` as the outsider.
```shell
curl -i localhost:3000/chats/$G -H "Authorization: Bearer $A"
```

| case | reply |
|---|---|
| a member opens a group | `200 {"id":..,"chatType":"group","name":..,"photo":..,"members":[..],"messages":[..]}` |
| a member opens a private chat | `200`, and `name`/`photo` are the other member's, as in §12 |
| the other member opens the same chat | `200`, and the name flips |
| a chat nobody has written in | `200` and `"messages":[]` |
| a message with text only | `"content":{"text":".."}`, no `photo` key |
| a message with a photo only | `"content":{"photo":"/photos/<uuid>"}`, no `text` key |
| a message with both | both fields |
| a message nobody reacted to | `"comments":[]`, never `null` |
| the id of a private chat | `200` — the one route where it is not a `404` |
| a caller who is not a member | `403 {"code":403,"message":"not a member of the chat"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"chat not found"}` |
| `/chats/not-a-uuid`, or the uppercase UUID | `400 {"code":400,"message":"invalid chat id"}` |
| `POST` / `PATCH` / `DELETE` on the path | `405 Method Not Allowed` (§16) |
| `GET /chats/{chatId}/` (trailing slash) | `404` (§16) |
| bad/absent token | §0 |

The reply is a `GroupDetail` or a `PrivateChatDetail`, the only endpoint that answers with either.

- No `snippet`, and no truncation: the list holds the last message itself, so the 120-character message of §12 reads 50 there and 120 here.
- One page of `ChatMessagesPageSize` (500), a `LIMIT` on the query and not a cut made afterwards, so a chat of any length is read in the same bounded time. Pagination is still missing (§19).
- Ordering and state are the ones of §12, same rules written once per read: `ORDER BY date DESC, rowid DESC`, and `read` once no member of the chat is left behind the message.
- A message photo is a URL, and a message without one keeps the field absent: `withMessagePhotoURL` returns early on the empty field, or `"/photos/"` — the prefix alone — would go out on every text-only message.
- The comments of the whole page are one query, bucketed by `messageId`, so a chat of 500 messages is one query and not 500. A reaction on a message older than the page is never read.
- 403 vs 404 in one read, as in §10: membership is an `EXISTS` column and not a condition of the `WHERE`, so the query finds the row whether or not the caller belongs to it — that is what tells "no chat owns this id" from "not yours". The row is read by the handler and never sent: a non-member is answered with the `403` body alone.
- Three reads in one transaction: a message arriving between the first and the last would otherwise land in the list with its sender missing from the members read before it.
- Opening a chat marks it read for the caller, and this is the only read of the project that writes. It adds no row and removes none: it moves one `chat_members.lastReadDate` forward, for the caller alone, and never backwards. The write is the last thing the transaction does, so the states in the reply are the ones taken before it — a message arrives in the state it was in when the chat was asked for, and reads `read` on the next call. A caller answered `403` writes nothing, the write sitting after the membership check.
- It is also what makes the state reachable at all: without it only writing in a chat would ever catch a member up, and a member who reads but never answers would hold every message at `received` forever.

---

## 14. `sendMessage` — `POST /chats/{chatId}/messages` (multipart)
The one write of a chat, and the first writer of the `messages` table. Both kinds take a message the same way, as both are read under `/chats/{chatId}`.

```shell
curl -i -X POST localhost:3000/chats/$G/messages -H "Authorization: Bearer $A" -F "text=hello everybody"
```

| case | reply |
|---|---|
| text only | `201` and the `Message`: `{"id":..,"user":..,"date":..,"state":..,"content":{"text":".."},"comments":[]}` |
| photo only | `201`, `content` is `{"photo":"/photos/<uuid>"}` — a new id, never the one uploaded before |
| text and photo | `201`, both fields |
| a private chat | `201` — the same call, the id in the URL is the only difference |
| neither, or a `text` of spaces alone | `400 {"code":400,"message":"empty message content: text or photo is required"}` |
| `text` of exactly 5000 chars | `201` |
| `text` of 5001 | `400 {"code":400,"message":"invalid message text"}` |
| 100 `👨‍👩‍👧‍👦` | `201` — grapheme clusters, so 100 and not 1100 runes, as in §8 |
| `photoFile` that is not an image, or empty | `400 {"code":400,"message":"invalid photo"}` — the type comes from the bytes |
| over `MaxPhotoBytes` / over the body limit | `400 invalid photo` / `400 invalid multipart body`, exactly as §4 |
| not multipart | `400 invalid multipart body` |
| `-F "file=@$PNG"` (wrong field) with a text | `201` — a part under another name is no photo at all |
| the same without a text | `400` — nothing is left for the message to carry |
| a caller who is not a member | `403 {"code":403,"message":"not a member of the chat"}` |
| a valid UUID owned by nobody | `404 {"code":404,"message":"chat not found"}` |
| `/chats/not-a-uuid/messages`, or the uppercase UUID | `400 {"code":400,"message":"invalid chat id"}` |
| a chat already holding 10000 messages | `400 {"code":400,"message":"the chat is full"}` |
| `GET` on the same path | `405 Method Not Allowed` (§16) |
| bad/absent token | §0, and nothing is written to `./db/photos` |

The photo is optional, and this is the only upload of the project where it is. `r.FormFile` answering `http.ErrMissingFile` is a message without a photo and not a bad request, while any other error still is one. A `text` that counts zero characters is not stored as `''` but as `NULL`: the endpoint can never write the row whose snippet guard §12 describes.

Order of the checks, as in §7. The text is validated before a single byte is written, so a message refused for its text leaves `ls db/photos | wc -l` unchanged. What is saved after that is dropped on every failing branch and not only on `500` — a `403` and a `404` are only known once the bytes are on disk, exactly as in §9:
```shell
ls db/photos | wc -l                                                     # before
curl -s -X POST localhost:3000/chats/$G/messages -H "Authorization: Bearer $D" -F "photoFile=@$PNG"    # -> 403
curl -s -X POST localhost:3000/chats/00000000-0000-4000-8000-000000000001/messages \
  -H "Authorization: Bearer $A" -F "photoFile=@$PNG"                     # -> 404
ls db/photos | wc -l                                                     # same number
```

Dates are ordinary `time.Time` values with millisecond precision. The database uses fixed-width UTC (`2026-08-28T11:34:19.450Z`) so string order is chronological; Go's standard JSON encoder may return the same instant as `...19.45Z` because trailing zeroes carry no information. No custom marshaler is needed. Every writer truncates to milliseconds before storing and returning the value, so a `201` and every later read represent the same instant.

The state of a new message is computed and never assumed. Sending catches the sender up to its own message — the `lastReadDate` written is the very string in `messages.date`, so `lastReadDate < date` is false for it and it never answers for what it wrote. That is why the sender needs no exception in the query, and why the reply is not always `received`:

| the chat the message is sent to | `state` in the `201` |
|---|---|
| holds somebody else who has not seen it | `received` |
| holds the sender alone | `read` — there is nobody to be behind it |

The three cases the rule was written for:
```shell
# e.g.1 a group the sender is alone in: the message is born read
curl -s -X POST localhost:3000/chats/$G1/messages -H "Authorization: Bearer $A" -F 'text=alone'   # -> "state":"read"

# e.g.2 a member that never opened it leaves, and the message it was holding becomes read
curl -s -X POST localhost:3000/chats/$G2/messages -H "Authorization: Bearer $A" -F 'text=hi all' # -> received
curl -s localhost:3000/chats/$G2 -H "Authorization: Bearer $B"                                   # B opens it
curl -s localhost:3000/chats/$G2 -H "Authorization: Bearer $A"                                   # -> still received, C has not
curl -s -X DELETE localhost:3000/groups/$G2/members/me -H "Authorization: Bearer $C"             # C leaves
curl -s localhost:3000/chats/$G2 -H "Authorization: Bearer $A"                                   # -> "state":"read"

# e.g.3 read is a black hole: a member joining answers for nothing sent before it
curl -s -X POST localhost:3000/groups/$G3/members -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"members\":[\"$D\"]}"                               # D joins
curl -s localhost:3000/chats/$G3 -H "Authorization: Bearer $A"                                   # -> still "state":"read"
```
`read` is absorbing, and two things make it so. A member joins with `lastReadDate` set to the moment it joined, which is later than every message already there, so it is behind none of them — a join can never turn a message back. And every write of the column is guarded with `lastReadDate < ?`, so a value only ever moves forward. Leaving works the other way with no code at all: `leaveGroup` removes the row, and a member that is gone holds nothing back.

The cap is a bad request and not a conflict. A chat holds `schemas.ChatMaxMessages` (10000); the count is taken inside the same transaction as the insert, so the row that would pass it is never written:
```shell
sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$GF';"   # -> 10000, and the 10001st was a 400
```

---

## 15. `forwardMessage` — `POST /chats/{chatId}/messages/forwards` (JSON)
Copies one existing message into a destination chat. The id in the body names the source message; the id in the URL names the destination chat. The source message stays untouched, while the copy gets a new id, date and sender and starts with no comments.

The verification used two private chats: `PS` between A and B as the source, and `PD` between A and C as the destination. That makes A a member of both, B a source-only member, C a destination-only member, and D an outsider to both.

```shell
PS=$(curl -s -X POST localhost:3000/private-chats -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"id\":\"$B\"}" | id)
PD=$(curl -s -X POST localhost:3000/private-chats -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/json' -d "{\"id\":\"$C\"}" | id)
MS=$(curl -s -X POST localhost:3000/chats/$PS/messages -H "Authorization: Bearer $A" \
  -F 'text=forward this text' | id)

curl -i -X POST localhost:3000/chats/$PD/messages/forwards \
  -H "Authorization: Bearer $A" -H 'Content-Type: application/json' \
  -d "{\"messageId\":\"$MS\"}"
```

| case | verified reply |
|---|---|
| text-only source | `201`; the copy has the same text and no `photo` key |
| photo-only source | `201`; the copy has the same photo URL and no `text` key |
| source with text and photo | `201`; both fields are copied |
| identity of the copy | a new message id and date; `user` is the caller, regardless of who sent the source |
| source has comments | the copy has `"comments":[]`; source comments are not copied |
| destination is the source chat itself | `201`; forwarding inside the same chat is allowed |
| destination is a private chat or a group | `201`; both chat kinds use the same route |
| caller belongs to source and destination | `201` |
| caller belongs to the source only | `403 {"code":403,"message":"not a member of the chat"}` |
| caller belongs to the destination only | the same `403` |
| caller belongs to neither | the same `403` |
| no message owns `messageId` | `404 {"code":404,"message":"message not found"}` |
| no chat owns `chatId` | `404 {"code":404,"message":"chat not found"}` |
| malformed, empty, or missing `messageId` body | `400 {"code":400,"message":"invalid request body"}` |
| non-canonical/uppercase `messageId` | the same `400` |
| non-canonical/uppercase `chatId` | `400 {"code":400,"message":"invalid chat id"}` |
| destination already holds 10000 messages | `400 {"code":400,"message":"the chat is full"}` and the count stays 10000 |
| `GET` on the path | `405 Method Not Allowed` (§16) |
| trailing slash | `404`, plain text (§16) |
| bad/absent token | §0, before the body or either chat is read |

The copied fields are `text` and `photoId`, including their absence as SQL `NULL`; comments belong to the source row and are deliberately not selected. A forwarded photo is not uploaded or duplicated: both message rows point at the same photo id, and the API turns that id into the same `/photos/<uuid>` URL. Consequently this endpoint writes no file to `db/photos`.

The source read, both membership checks, the destination count, insert, sender catch-up and state read are one transaction. Every client-refused branch rolls it back, so a `400`, `403` or `404` adds no message and changes no `lastReadDate`. An internal failure before commit rolls back too. A success writes exactly one message row and may move only the caller's destination `lastReadDate` forward.

The new message follows the same state rule as §14. Forwarding means writing, so the caller is caught up to the copy using its exact date. The `201` is `received` while any other destination member is behind and `read` when nobody is; the automated endpoint test verifies both a two-member destination (`received`) and a group in which the caller is the only remaining member (`read`). Its date in the response names the same instant stored in `messages.date` and in the sender's `lastReadDate`.

Checks have a deliberate order:

1. Authentication runs first.
2. The destination `chatId` syntax is validated before the JSON body.
3. The source message must exist and the caller must belong to its chat.
4. The destination chat must exist and the caller must belong to it.
5. The destination capacity is checked before the insert.

Thus an invalid destination UUID wins over a broken body; once both ids are well formed, an absent source answers `message not found` before an absent destination is considered. An unknown JSON property is currently ignored, as for every other JSON request; that is the global `additionalProperties: false` mismatch in §18, not special behavior of forwarding.

The endpoint originally failed before any request because it was registered on the same `POST /chats/:chatId/messages` route as `sendMessage`, and `httprouter` rejects a duplicate method/path. The verified registration is `POST /chats/:chatId/messages/forwards`, matching `doc/api.yaml`.

---

## 16. Router level — the answers that are not JSON
`httprouter` replies before any handler, so these carry a plain text body and not the `Error` schema.

| request | reply |
|---|---|
| `GET /nope` | `404`, plain text |
| `GET /session` (registered as POST) | `405 Method Not Allowed` + `Allow: OPTIONS, POST` |
| `GET /users/` (trailing slash) | `404` — `RedirectTrailingSlash` is off, so no redirect |
| `OPTIONS /users` | `200` and an empty body, with no `Allow` header: the CORS middleware of `cmd/webapi/cors.go` answers every preflight before the router sees it |

CORS is worth one check, since the frontend depends on it:
```shell
curl -i -X OPTIONS localhost:3000/me/username -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: PATCH' -H 'Access-Control-Request-Headers: authorization,content-type'
# -> 200, Access-Control-Allow-Origin: *, Access-Control-Allow-Methods: PATCH,
#    Access-Control-Allow-Headers: Authorization,Content-Type, Access-Control-Max-Age: 1
# The headers appear only when the request carries an Origin: a bare OPTIONS gets a plain 200
```

---

## 17. Scenarios — a sequence, not a single call
S1 — the token of `doLogin` is accepted by `authenticate`. `doLogin "s1"` → `201` id; `GET /users?username=s1` with that id → `200`. Nothing else checks that the two endpoints agree.

S2 — the token survives a rename. `doLogin "s2"` → id; `setMyUserName "s2b"` → `200`; `getUsers` with the same id → `200`. The token is the user id and the id never changes: a rename must not log anybody out.

S3 — a freed username is a new account. `doLogin "s3"` → id A; `setMyUserName "s3b"` → `200`; `doLogin "s3"` → `201` and a new id B ≠ A; `doLogin "s3b"` → `200` and A. The identity is the id, never the username.

S4 — a taken username is released. u1 `doLogin "s4a"`, u2 `doLogin "s4b"`; u1 `setMyUserName "s4b"` → `400 username already taken`; u2 `setMyUserName "s4c"` → `200`; u1 `setMyUserName "s4b"` → `200`.

S5 — a photo outlives the user that replaced it only while something points at it. A `setMyPhoto` → URL P1; A `setMyPhoto` again → URL P2; `GET P1` → `404` (file collected), `GET P2` → `200`. Then `createGroup` with a photo, and check the group photo is never collected while the group exists.

S6 — chat and group do not collide. A+B `createPrivateChat` → chat C (`201`, then `200` from both sides); A `createGroup` with B → group G ≠ C; `createPrivateChat` A+B again → still C. A group never satisfies the `pairKey` UNIQUE.

S7 — nothing survives a refused write. Count `db/photos` and the `chats` rows, send a `createGroup` with a nonexistent member and a valid photo, count again: both unchanged. The photo is saved after the check, and the chat plus its memberships are one transaction.

S8 — a rename touches the name and nothing else. `createGroup` → G with photo URL P; `setGroupName` twice → `200` each time and the `photo` in both replies is still P; `getPhoto P` → `200`. A rename writes one column, so it can neither orphan the group photo nor collect it.

S9 — the two kinds of chat stay apart under `/groups/`. A+B `createPrivateChat` → C; `setGroupName` on C → `404 group not found`, and the row of C still has `name` and `photoId` `NULL`. A `createGroup` with B → G; `setGroupName` on G → `200`. The same id space, two answers, and the `chats` CHECK is never reached because the `WHERE` filters on `chatType` first.

S10 — a member added is a member for every other call. A `createGroup` with B → G; A `addToGroup` C → `200`; then C `setGroupName` on G → `200`, C `setGroupPhoto` → `200`, C `addToGroup` D → `200`. One row in `chat_members` is what all of them read, so joining grants the whole group surface and not only the list C appeared in.

S11 — leaving takes the whole group surface away, the inverse of S10. A `createGroup` with B and C → G; C `leaveGroup` → `204`; then C `setGroupName` → `403`, C `setGroupPhoto` → `403`, C `addToGroup` → `403`, C `leaveGroup` again → `403`. The same row that granted everything is the one just removed.

S12 — a group outlives every member but the last. A `createGroup` with B → G with photo P; A `leaveGroup` → `204` and `chat_members` still holds B, `getPhoto P` → `200`; B `leaveGroup` → `204` and the `chats` row is gone, `getPhoto P` → `404`, and G is a `404` for both of them. The group is dropped exactly once, by the member that empties it, and its photo goes with it. The replies say none of this: a departed member is told only that it left, and the rest is read from the tables.

S13 — the list is the membership, seen from the other side. A fresh user `getMyConversations` → `404`; A `createPrivateChat` with it → the chat appears for both, named after the other one each time; A `createGroup` with it → the group appears too; it `leaveGroup` → the group is gone from its list and still in A's; it is the only member left of nothing, so once the private chat is its last chat the list holds exactly one row. Every row of §12 is one row of `chat_members`, which is why §10 and §11 change the list without touching it.

S14 — a rename reaches the chats list of somebody else. B `setMyUserName` → `200`; A `getMyConversations` → the private chat with B reads the new username in the same call, with no write to `chats`. The mirror of S8: a group rename writes a column, a private chat rename writes none and is read through the other member.

S15 — the homepage preview is the last message of the opened chat. A+B `createPrivateChat` → P; two messages sent through `sendMessage`, the newer one 120 characters long; `getMyConversations` → the `snippet.id` is the id of the first message `getConversation` answers with, and its `text` is the first 50 characters of the same text the opened chat carries whole. The list holds no `messages` and the opened chat holds no `snippet`. Opening the chat changes only the snippet state from `received` to `read`; its id, content and ordering stay the same. The two endpoints read the same rows through different queries — one takes the last message of every chat, the other a page of one chat — so this is the only check that they agree on which message that is.

S16 — forwarding changes the destination without changing the source. A+B `createPrivateChat` → source P; A+C `createPrivateChat` → destination D; B sends a text-and-photo message M in P; A `forwardMessage` M into D → copy F. F has a new id, A as its sender and the exact text/photo of M. M remains in P with B as its sender, while F becomes both the first message of D and C's preview of D. The preview starts `received` and becomes `read` after C opens D.

---

## 18. Known mismatches with `doc/api.yaml` (implemented endpoints only)
Open:
- `additionalProperties: false` is not enforced. `encoding/json` ignores unknown fields, so `{"username":"x","admin":true}` is accepted. Fix: `dec.DisallowUnknownFields()` in `decodeAndValidate`.
- The request `Content-Type` is never read. `application/json` works where the spec says `application/merge-patch+json`. Permissive, not wrong.
- A name is trimmed to be counted and stored untrimmed. `CountChars` measures `strings.TrimSpace(s)`, so `"  <100 chars>  "` passes the 1–100 rule and 104 characters reach the column and the reply, over `GroupName`'s `maxLength: 100`. This affects `ChatName`; `Username` is safe because its pattern refuses whitespace, and `MessageTextRequest` normalizes message text before validation. Fix: trim the name before storing, or count the untrimmed string.
```shell
curl -s -X PATCH localhost:3000/groups/$G/name -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/merge-patch+json' -d "{\"name\":\"  $(printf 'x%.0s' $(seq 100))  \"}"
sqlite3 db/wasatext.db "SELECT length(name) FROM chats WHERE id='$G';"   # -> 104
```

## 19. Open decisions
- A page is a cap and not yet pagination. `getMyConversations` stops at `UserChatsPageSize` (500) and `getConversation` at `ChatMessagesPageSize` (500), both matching the `maxItems` the spec declares, so neither is a mismatch any more. What neither has is a way to ask for the next page: over the limit the rows are simply cut, which hides chats from the homepage and history from a chat instead of shortening either. The spec already reserves the place for the fix — `# EXTRA: add cursor based pagination` under `components/parameters`.
- The search returns the caller. Nothing filters the caller out of `getUsers`, and the frontend uses that list to open a private chat — where picking yourself is a `400`. Either the query adds `AND id != <caller>`, or the frontend hides the row.
