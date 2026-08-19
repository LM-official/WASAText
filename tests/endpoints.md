# WASAText — endpoint tests
Manual tests for the endpoints registered in `service/api/api-handler.go`:
`doLogin`, `setMyUserName`, `setMyPhoto`, `getUsers`, `getPhoto`, `createPrivateChat`, `createGroup`, `setGroupName`, `setGroupPhoto`, `addToGroup`.

Last full run: 2026-08-19, every row below re-checked against a live server. §10 (`addToGroup`) was added on the same date and checked the same way.

## Run
```shell
go run ./cmd/webapi/          # another terminal
```

`service/database/database.go` creates `./db/wasatext.db` and every table on startup (`CREATE TABLE IF NOT EXISTS`), `service/photos/photos.go` creates `./db/photos`. There is nothing to set up by hand and nothing to delete afterwards: the database is kept between runs, so every test below uses a run timestamp suffix `$R` and never depends on a database.

The file appears at the first run of the server, never at build time: `go build` writes no database, and an empty `db/` before the first `go run ./cmd/webapi/` is normal, not a failure.

A schema change is the one exception: `CREATE TABLE IF NOT EXISTS` does not alter an existing table, so a new column or constraint is only visible after `rm ./db/wasatext.db`.

## Fixtures
```shell
R=$(date +%s)                                   # run suffix: fresh usernames on every run
id() { sed 's/.*"id":"\([^"]*\)".*/\1/'; }      # extracts the id from a reply (or use jq -r .id)

A=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"alice$R\"}" | id)
B=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"bob$R\"}"   | id)
echo "A=$A B=$B"                                # the id is both the user id and the bearer token

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
`service/api/api-authenticate.go` runs before the handler, so these answers are the same on `/me/username`, `/me/photo`, `/users`, `/photos/:photoId`, `/private-chats`, `/groups`, `/groups/:groupId/name`, `/groups/:groupId/photo` and `/groups/:groupId/members`, and nothing is read or written when they fire.

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
curl -i -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"carl$R\"}"
```

| body | reply |
|---|---|
| `{"username":"carl$R"}` first time | `201 {"id":"<uuid>"}` — registered |
| `{"username":"carl$R"}` again | `200` and the same id — logged in |
| `{"usernam":"carl"}` (typo) | `400 invalid request body` — the field stays empty and breaks the length rule |
| `{"username":""}` / `{}` / `{"username":null}` | `400 invalid request body` |
| `{"username":"a b"}` / `" carl "` / `"cà rl"` | `400` — the pattern is `^[a-zA-Z0-9_.-]+$` |
| `{"username":"<30 chars>"}` | `201` |
| `{"username":"<31 chars>"}` | `400` |
| `{"username":` (truncated) or empty body | `400 invalid request body` |
| `{"username":"carl$R","admin":true}` | `200`/`201` — unknown fields are ignored, see §13 |

The returned id is the bearer token of every other test: this is the only point where `doLogin` and `authenticate` have to agree.

---

## 2. `setMyUserName` — `PATCH /me/username`
```shell
curl -i -X PATCH localhost:3000/me/username \
  -H "Authorization: Bearer $A" -H 'Content-Type: application/merge-patch+json' \
  -d "{\"username\":\"alice_new$R\"}"
```

| case | reply |
|---|---|
| free username | `200 {"id":"$A","username":"alice_new$R","photo":"/photos/00000000-0000-4000-8000-000000000000"}` |
| the username it already has | `200`, unchanged — updating a row to its own value is not a UNIQUE conflict |
| `"bob$R"` (owned by B) | `400 {"code":400,"message":"username already taken"}`, nothing written |
| `{"username":""}`, `{}`, `{"username":"a b"}`, 31 chars | `400 invalid request body` |
| `Content-Type: application/json` | identical result — the handler never reads the header (§13) |
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
| own username with own token | `200`, and the caller is in the list — see §14 |

LIKE escaping. `_` and `%` are LIKE wildcards and `_` is a legal username character, so `service/database/get-users.go` escapes the prefix:
```shell
curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"a_b$R\"}"
curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"axb$R\"}"
curl -s "localhost:3000/users?username=a_" -H "Authorization: Bearer $A"
```

`a_b$R` must be in the list and `axb$R` must not.

The cap:
```shell
for i in $(seq 1 25); do curl -s -X POST localhost:3000/session \
  -H 'Content-Type: application/json' -d "{\"username\":\"lim${R}_$i\"}" > /dev/null; done
curl -s "localhost:3000/users?username=lim$R" -H "Authorization: Bearer $A" | grep -o '"id"' | wc -l
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
| `/photos/../../etc/passwd`, raw or percent-encoded | `404`, plain text — the extra segments match no route, so the router answers before the handler (§11) |
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
  -F "data={\"name\":\"Study $R\",\"members\":[\"$B\"]};type=application/json" \
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
C=$(curl -s -X POST localhost:3000/session -H 'Content-Type: application/json' -d "{\"username\":\"carl$R\"}" | id)
G=$(curl -s -X POST localhost:3000/groups -H "Authorization: Bearer $A" \
  -F "data={\"name\":\"Study $R\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)

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
| `Content-Type: application/json` | identical result — the handler never reads the header (§13) |
| `{"name":"x","admin":true}` | `200` — unknown fields are ignored (§13) |
| `GET` / `POST` on the same path | `405 Method Not Allowed` (§11) |
| bad/absent token | §0, and the name is not touched |

The reply is a `GroupSummary`: renaming never reads the messages of the group, so the `snippet` derived from them.

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
| `GET` / `POST` on the same path | `405 Method Not Allowed` (§11) |
| bad/absent token | §0, and nothing is written to `./db/photos` |

The reply is a `GroupSummary`, like §8: replacing a photo never reads the messages, so no `snippet` is derived.

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
| `GET` / `PATCH` / `DELETE` on the same path | `405 Method Not Allowed` + `Allow: OPTIONS, POST` (§11) |
| bad/absent token | §0, and no membership is written |

The reply is a `GroupWithMembers`: the summary plus the whole member list, and no messages, so no `snippet` is derived. The list is read after the `INSERT`, so its length is what the table holds and never what the request asked — adding 2 people to a group of 50 answers with 52.

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

## 11. Router level — the answers that are not JSON
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

## 12. Scenarios — a sequence, not a single call
S1 — the token of `doLogin` is accepted by `authenticate`. `doLogin "s1$R"` → `201` id; `GET /users?username=s1$R` with that id → `200`. Nothing else checks that the two endpoints agree.

S2 — the token survives a rename. `doLogin "s2$R"` → id; `setMyUserName "s2b$R"` → `200`; `getUsers` with the same id → `200`. The token is the user id and the id never changes: a rename must not log anybody out.

S3 — a freed username is a new account. `doLogin "s3$R"` → id A; `setMyUserName "s3b$R"` → `200`; `doLogin "s3$R"` → `201` and a new id B ≠ A; `doLogin "s3b$R"` → `200` and A. The identity is the id, never the username.

S4 — a taken username is released. u1 `doLogin "s4a$R"`, u2 `doLogin "s4b$R"`; u1 `setMyUserName "s4b$R"` → `400 username already taken`; u2 `setMyUserName "s4c$R"` → `200`; u1 `setMyUserName "s4b$R"` → `200`.

S5 — a photo outlives the user that replaced it only while something points at it. A `setMyPhoto` → URL P1; A `setMyPhoto` again → URL P2; `GET P1` → `404` (file collected), `GET P2` → `200`. Then `createGroup` with a photo, and check the group photo is never collected while the group exists.

S6 — chat and group do not collide. A+B `createPrivateChat` → chat C (`201`, then `200` from both sides); A `createGroup` with B → group G ≠ C; `createPrivateChat` A+B again → still C. A group never satisfies the `pairKey` UNIQUE.

S7 — nothing survives a refused write. Count `db/photos` and the `chats` rows, send a `createGroup` with a nonexistent member and a valid photo, count again: both unchanged. The photo is saved after the check, and the chat plus its memberships are one transaction.

S8 — a rename touches the name and nothing else. `createGroup` → G with photo URL P; `setGroupName` twice → `200` each time and the `photo` in both replies is still P; `getPhoto P` → `200`. A rename writes one column, so it can neither orphan the group photo nor collect it.

S9 — the two kinds of chat stay apart under `/groups/`. A+B `createPrivateChat` → C; `setGroupName` on C → `404 group not found`, and the row of C still has `name` and `photoId` `NULL`. A `createGroup` with B → G; `setGroupName` on G → `200`. The same id space, two answers, and the `chats` CHECK is never reached because the `WHERE` filters on `chatType` first.

S10 — a member added is a member for every other call. A `createGroup` with B → G; A `addToGroup` C → `200`; then C `setGroupName` on G → `200`, C `setGroupPhoto` → `200`, C `addToGroup` D → `200`. One row in `chat_members` is what all of them read, so joining grants the whole group surface and not only the list C appeared in.

---

## 13. Known mismatches with `doc/api.yaml` (implemented endpoints only)
Open:
- `additionalProperties: false` is not enforced. `encoding/json` ignores unknown fields, so `{"username":"x","admin":true}` is accepted. Fix: `dec.DisallowUnknownFields()` in `decodeAndValidate`.
- The request `Content-Type` is never read. `application/json` works where the spec says `application/merge-patch+json`. Permissive, not wrong.
- A name is trimmed to be counted and stored untrimmed. `CountChars` measures `strings.TrimSpace(s)`, so `"  <100 chars>  "` passes the 1–100 rule and 104 characters reach the column and the reply, over `GroupName`'s `maxLength: 100`. Affects `ChatName` and, when it lands, `MessageText`; `Username` is safe because its pattern already refuses whitespace. Fix: trim before storing, or count the untrimmed string.
```shell
curl -s -X PATCH localhost:3000/groups/$G/name -H "Authorization: Bearer $A" \
  -H 'Content-Type: application/merge-patch+json' -d "{\"name\":\"  $(printf 'x%.0s' $(seq 100))  \"}"
sqlite3 db/wasatext.db "SELECT length(name) FROM chats WHERE id='$G';"   # -> 104
```

## 14. Open decisions
- The search returns the caller. Nothing filters the caller out of `getUsers`, and the frontend uses that list to open a private chat — where picking yourself is a `400`. Either the query adds `AND id != <caller>`, or the frontend hides the row.
