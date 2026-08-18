# WASAText — endpoint tests
Manual tests for the endpoints registered in `service/api/api-handler.go`:
`doLogin`, `setMyUserName`, `setMyPhoto`, `getUsers`, `getPhoto`, `createPrivateChat`, `createGroup`.

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
`service/api/api-authenticate.go` runs before the handler, so these answers are the same on `/me/username`, `/me/photo`, `/users`, `/photos/:photoId`, `/private_chats` and `/groups`, and nothing is read or written when they fire.

| Authorization header | reply |
|---|---|
| *(absent)* | `401 {"code":401,"message":"missing bearer token"}` |
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
| `{"username":"carl$R","admin":true}` | `200`/`201` — unknown fields are ignored, see §10 |

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
| `Content-Type: application/json` | identical result — the handler never reads the header (§10) |
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
| own username with own token | `200`, and the caller is in the list — see §11 |

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
| `/photos/../../etc/passwd` | `400 invalid photo id` — an id is a canonical UUID, so it can hold no `/` and no `.` |
| a valid UUID with no file | `404 {"code":404,"message":"photo not found"}` |
| no token | `401` — which is why the frontend cannot use a plain `<img src>` and has to fetch the bytes |

`http.ServeContent` adds the rest:
```shell
curl -i -H "Range: bytes=0-9" -H "Authorization: Bearer $A" localhost:3000/photos/000...000 -o /dev/null
```

The type is the one detected from the bytes: a file renamed `.png` that is really a JPEG is served as `image/jpeg`, and nothing that is not an image can be stored in the first place (§4).

---

## 6. `createPrivateChat` — `POST /private_chats`
```shell
curl -i -X POST localhost:3000/private_chats -H "Authorization: Bearer $A" \
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
| valid name + members + photo | `201 {"id":"<chatId>"}`, members are `[$B, $A]`: the creator is added by the server |
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

## 8. Router level — the answers that are not JSON
`httprouter` replies before any handler, so these carry a plain text body and not the `Error` schema.

| request | reply |
|---|---|
| `GET /nope` | `404`, plain text |
| `GET /session` (registered as POST) | `405 Method Not Allowed` + `Allow: POST` |
| `GET /users/` (trailing slash) | `404` — `RedirectTrailingSlash` is off, so no redirect |
| `OPTIONS /users` | `204` + `Allow`, and the CORS headers from `cmd/webapi/cors.go` |

CORS is worth one check, since the frontend depends on it:
```shell
curl -i -X OPTIONS localhost:3000/me/username -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: PATCH' -H 'Access-Control-Request-Headers: authorization,content-type'
# -> Access-Control-Allow-Origin: *, Allow-Methods including PATCH, Allow-Headers with Authorization
```

---

## 9. Scenarios — a sequence, not a single call
S1 — the token of `doLogin` is accepted by `authenticate`. `doLogin "s1$R"` → `201` id; `GET /users?username=s1$R` with that id → `200`. Nothing else checks that the two endpoints agree.

S2 — the token survives a rename. `doLogin "s2$R"` → id; `setMyUserName "s2b$R"` → `200`; `getUsers` with the same id → `200`. The token is the user id and the id never changes: a rename must not log anybody out.

S3 — a freed username is a new account. `doLogin "s3$R"` → id A; `setMyUserName "s3b$R"` → `200`; `doLogin "s3$R"` → `201` and a new id B ≠ A; `doLogin "s3b$R"` → `200` and A. The identity is the id, never the username.

S4 — a taken username is released. u1 `doLogin "s4a$R"`, u2 `doLogin "s4b$R"`; u1 `setMyUserName "s4b$R"` → `400 username already taken`; u2 `setMyUserName "s4c$R"` → `200`; u1 `setMyUserName "s4b$R"` → `200`.

S5 — a photo outlives the user that replaced it only while something points at it. A `setMyPhoto` → URL P1; A `setMyPhoto` again → URL P2; `GET P1` → `404` (file collected), `GET P2` → `200`. Then `createGroup` with a photo, and check the group photo is never collected while the group exists.

S6 — chat and group do not collide. A+B `createPrivateChat` → chat C (`201`, then `200` from both sides); A `createGroup` with B → group G ≠ C; `createPrivateChat` A+B again → still C. A group never satisfies the `pairKey` UNIQUE.

S7 — nothing survives a refused write. Count `db/photos` and the `chats` rows, send a `createGroup` with a nonexistent member and a valid photo, count again: both unchanged. The photo is saved after the check, and the chat plus its memberships are one transaction.

---

## 10. Known mismatches with `doc/api.yaml` (implemented endpoints only)
Open:
- `additionalProperties: false` is not enforced. `encoding/json` ignores unknown fields, so `{"username":"x","admin":true}` is accepted. Fix: `dec.DisallowUnknownFields()` in `decodeAndValidate`.
- The request `Content-Type` is never read. `application/json` works where the spec says `application/merge-patch+json`. Permissive, not wrong.

## 11. Open decisions
- The search returns the caller. Nothing filters the caller out of `getUsers`, and the frontend uses that list to open a private chat — where picking yourself is a `400`. Either the query adds `AND id != <caller>`, or the frontend hides the row.
