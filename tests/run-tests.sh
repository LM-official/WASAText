#!/usr/bin/env bash
# tests/endpoints.md, sections 0-20, for the endpoints registered in api-handler.go
cd /Users/lorenzo/WASAText
H=localhost:3000
PASS=0; FAIL=0; FAILED=()

id() { sed 's/.*"id":"\([^"]*\)".*/\1/'; }
ok()  { PASS=$((PASS+1)); }
no()  { FAIL=$((FAIL+1)); FAILED+=("$1 -- got: $2"); }
# eq <label> <expected> <actual>
eq()  { if [ "$2" = "$3" ]; then ok; else no "$1 (want '$2')" "$3"; fi; }
# has <label> <needle> <haystack>
has() { case "$3" in *"$2"*) ok;; *) no "$1 (want substring '$2')" "$3";; esac; }
hasnt() { case "$3" in *"$2"*) no "$1 (unwanted '$2')" "$3";; *) ok;; esac; }
# code <method> <path> [curl args...]  -> prints status
code() { local m=$1 p=$2; shift 2; curl -s -o /dev/null -w '%{http_code}' -X "$m" "$H$p" "$@"; }
body() { local m=$1 p=$2; shift 2; curl -s -X "$m" "$H$p" "$@"; }

PNG=db/photos/00000000-0000-4000-8000-000000000000
printf 'not an image' > /tmp/text.txt
: > /tmp/empty.png

# Empty the database and rebuild the cast, so every run starts from the same rows and a fixture
# is just a name: nothing below depends on what a previous run happened to leave behind
bash tests/seed.sh || { echo "seed failed"; exit 1; }

A=$(body POST /session -H 'Content-Type: application/json' -d "{\"username\":\"alice\"}" | id)
B=$(body POST /session -H 'Content-Type: application/json' -d "{\"username\":\"bob\"}" | id)
C=$(body POST /session -H 'Content-Type: application/json' -d "{\"username\":\"carl\"}" | id)
D=$(body POST /session -H 'Content-Type: application/json' -d "{\"username\":\"dave\"}" | id)
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"
CU="Authorization: Bearer $C"; DU="Authorization: Bearer $D"
JS='Content-Type: application/json'

echo "### 0. authentication"
eq "0 no header"        401 "$(code GET /users?username=alice)"
has "0 no header body"  '"message":"missing bearer token"' "$(body GET /users?username=alice)"
eq "0 lowercase bearer" 401 "$(code GET /users?username=alice -H "Authorization: bearer $A")"
eq "0 no token"         401 "$(code GET /users?username=alice -H 'Authorization: Bearer')"
eq "0 not a uuid"       401 "$(code GET /users?username=alice -H 'Authorization: Bearer nope')"
has "0 not a uuid body" '"message":"invalid token format"' "$(body GET /users?username=alice -H 'Authorization: Bearer nope')"
eq "0 uppercase uuid"   401 "$(code GET /users?username=alice -H "Authorization: Bearer $(echo $A | tr a-f A-F)")"
eq "0 unknown token"    401 "$(code GET /users?username=alice -H 'Authorization: Bearer 11111111-2222-4333-8444-555555555555')"
has "0 unknown body"    '"message":"unknown token"' "$(body GET /users?username=alice -H 'Authorization: Bearer 11111111-2222-4333-8444-555555555555')"

echo "### 1. doLogin"
eq "1 register"      201 "$(code POST /session -H "$JS" -d "{\"username\":\"new\"}")"
eq "1 login again"   200 "$(code POST /session -H "$JS" -d "{\"username\":\"new\"}")"
eq "1 same id"       "$(body POST /session -H "$JS" -d "{\"username\":\"new\"}" | id)" "$(body POST /session -H "$JS" -d "{\"username\":\"new\"}" | id)"
eq "1 typo field"    400 "$(code POST /session -H "$JS" -d '{"usernam":"carl"}')"
eq "1 empty string"  400 "$(code POST /session -H "$JS" -d '{"username":""}')"
eq "1 empty object"  400 "$(code POST /session -H "$JS" -d '{}')"
eq "1 null"          400 "$(code POST /session -H "$JS" -d '{"username":null}')"
eq "1 space"         400 "$(code POST /session -H "$JS" -d '{"username":"a b"}')"
eq "1 accent"        400 "$(code POST /session -H "$JS" -d '{"username":"cà rl"}')"
N30="$(printf 'x%.0s' $(seq 30))"
N31="$(printf 'y%.0s' $(seq 31))"
P30="{\"username\":\"$N30\"}"; P31="{\"username\":\"$N31\"}"
eq "1 30 chars ($(printf %s "$N30" | wc -c | tr -d ' ') chars)" 201 "$(code POST /session -H "$JS" -d "$P30")"
eq "1 31 chars ($(printf %s "$N31" | wc -c | tr -d ' ') chars)" 400 "$(code POST /session -H "$JS" -d "$P31")"
eq "1 truncated"     400 "$(code POST /session -H "$JS" -d '{"username":')"
eq "1 empty body"    400 "$(code POST /session -H "$JS" -d '')"
PUNK="{\"username\":\"new\",\"admin\":true}"
eq "1 unknown field" 200 "$(code POST /session -H "$JS" -d "$PUNK")"

echo "### 2. setMyUserName"
eq "2 free username" 200 "$(code PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"alice_new\"}")"
has "2 body" "\"id\":\"$A\",\"username\":\"alice_new\",\"photo\":\"/photos/00000000-0000-4000-8000-000000000000\"" \
     "$(body PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"alice_new\"}")"
eq "2 idempotent"    200 "$(code PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"alice_new\"}")"
eq "2 taken"         400 "$(code PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"bob\"}")"
has "2 taken body"   '"message":"username already taken"' "$(body PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"bob\"}")"
eq "2 invalid"       400 "$(code PUT /me/username -H "$AU" -H "$JS" -d '{"username":"a b"}')"
eq "2 json content type" 200 "$(code PUT /me/username -H "$AU" -H "$JS" -d "{\"username\":\"alice_new\"}")"
eq "2 no token"      401 "$(code PUT /me/username -H "$JS" -d '{"username":"x"}')"

echo "### 3. getUsers"
eq "3 match"      200 "$(code GET "/users?username=alice_new" -H "$AU")"
eq "3 no match"   404 "$(code GET "/users?username=zzzznope" -H "$AU")"
has "3 404 body"  '"message":"no user matches the given username"' "$(body GET "/users?username=zzzznope" -H "$AU")"
eq "3 empty param" 400 "$(code GET "/users?username=" -H "$AU")"
eq "3 no param"   400 "$(code GET "/users" -H "$AU")"
eq "3 bad chars"  400 "$(code GET "/users?username=a%20b" -H "$AU")"
# a_b and axb come from the seed: '_' is a LIKE wildcard and a legal username character,
# so an unescaped prefix of a_b would find axb too
LIKE=$(body GET "/users?username=a_b" -H "$AU")
has   "3 LIKE escape keeps a_b"  '"username":"a_b"' "$LIKE"
hasnt "3 LIKE escape drops axb"  '"username":"axb"' "$LIKE"
# the 99 fill users of the seed all match this prefix, and the query answers 20 of them
eq "3 LIMIT 20" 20 "$(body GET "/users?username=fill" -H "$AU" | grep -o '"id"' | wc -l | tr -d ' ')"

echo "### 4. getUser"
# §3 searches by username prefix; this one goes the other way, from the id every other
# reply hands out (the user of a message, the author of a comment, a member of a chat)
eq  "4 other user"   200 "$(code GET /users/$B -H "$AU")"
has "4 other body"   "\"id\":\"$B\",\"username\":\"bob\",\"photo\":\"/photos/00000000-0000-4000-8000-000000000000\"" "$(body GET /users/$B -H "$AU")"
# nothing is special about asking for yourself: alice was renamed in §2 and answers under the new name
eq  "4 own id"       200 "$(code GET /users/$A -H "$AU")"
has "4 own body"     "\"id\":\"$A\",\"username\":\"alice_new\"" "$(body GET /users/$A -H "$AU")"
# the photo is a URL and never the stored id
has "4 photo is a URL" '"photo":"/photos/' "$(body GET /users/$A -H "$AU")"
eq  "4 unknown uuid" 404 "$(code GET /users/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "4 unknown body" '"message":"user not found"' "$(body GET /users/11111111-2222-4333-8444-555555555555 -H "$AU")"
eq  "4 not a uuid"   400 "$(code GET /users/not-a-uuid -H "$AU")"
has "4 not a uuid body" '"message":"invalid user id"' "$(body GET /users/not-a-uuid -H "$AU")"
eq  "4 uppercase uuid" 400 "$(code GET /users/$(echo $A | tr a-f A-F) -H "$AU")"
eq  "4 no token"     401 "$(code GET /users/$A)"
eq  "4 no token wins over a malformed id" 401 "$(code GET /users/not-a-uuid)"
eq  "4 POST"         405 "$(code POST /users/$A -H "$AU")"
eq  "4 PUT"          405 "$(code PUT /users/$A -H "$AU")"
eq  "4 DELETE"       405 "$(code DELETE /users/$A -H "$AU")"
eq  "4 trailing slash" 404 "$(code GET /users/$A/ -H "$AU")"
# the item does not shadow the collection: /users is static, /users/:userId one segment deeper
eq  "4 collection still answers" 200 "$(code GET "/users?username=alice_new" -H "$AU")"
eq  "4 item is one user, not a list" 1 "$(body GET /users/$A -H "$AU" | grep -o '"id"' | wc -l | tr -d ' ')"
# a read of one row of users: it belongs to no chat, so unlike §14 and §19 it writes nothing
ROWS_BEFORE=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM users)||'/'||(SELECT COUNT(*) FROM chat_members);")
body GET /users/$B -H "$AU" > /dev/null
eq  "4 writes nothing" "$ROWS_BEFORE" "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM users)||'/'||(SELECT COUNT(*) FROM chat_members);")"

echo "### 5. setMyPhoto"
BEFORE=$(ls db/photos | wc -l | tr -d ' ')
eq "5 png"          200 "$(code PUT /me/photo -H "$AU" -F "photoFile=@$PNG")"
P1=$(body PUT /me/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
P2=$(body PUT /me/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
if [ "$P1" != "$P2" ]; then ok; else no "4 new upload new id" "$P1 == $P2"; fi
if [ ! -f "db/photos/$P1" ]; then ok; else no "4 replaced photo collected" "db/photos/$P1 still there"; fi
if [ -f "db/photos/00000000-0000-4000-8000-000000000000" ]; then ok; else no "4 default photo kept" "gone"; fi
N=$(ls db/photos | wc -l | tr -d ' ')
eq "5 wrong field"   400 "$(code PUT /me/photo -H "$AU" -F "file=@$PNG")"
has "5 wrong field body" '"message":"missing photoFile field"' "$(body PUT /me/photo -H "$AU" -F "file=@$PNG")"
eq "5 not an image"  400 "$(code PUT /me/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
has "5 not an image body" '"message":"invalid photo"' "$(body PUT /me/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
eq "5 empty file"    400 "$(code PUT /me/photo -H "$AU" -F "photoFile=@/tmp/empty.png")"
eq "5 not multipart" 400 "$(code PUT /me/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
has "5 not multipart body" '"message":"invalid multipart body"' "$(body PUT /me/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
eq "5 no leaked files" "$N" "$(ls db/photos | wc -l | tr -d ' ')"
{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=512k count=61 2>/dev/null; } > /tmp/big.png
eq "5 over MaxPhotoBytes" 400 "$(code PUT /me/photo -H "$AU" -F "photoFile=@/tmp/big.png")"
has "5 over MaxPhotoBytes body" '"message":"invalid photo"' "$(body PUT /me/photo -H "$AU" -F "photoFile=@/tmp/big.png")"
{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=1m count=33 2>/dev/null; } > /tmp/huge.png
eq "5 over body limit" 400 "$(code PUT /me/photo -H "$AU" -F "photoFile=@/tmp/huge.png")"
has "5 over body limit body" '"message":"invalid multipart body"' "$(body PUT /me/photo -H "$AU" -F "photoFile=@/tmp/huge.png")"
eq "5 still no leaks" "$N" "$(ls db/photos | wc -l | tr -d ' ')"
eq "5 no token" 401 "$(code PUT /me/photo -F "photoFile=@$PNG")"

echo "### 6. getPhoto"
DEF=00000000-0000-4000-8000-000000000000
eq "6 existing"   200 "$(code GET /photos/$DEF -H "$AU")"
HDRS=$(curl -s -D- -o /dev/null "$H/photos/$DEF" -H "$AU")
has "6 content-type"  'image/png' "$HDRS"
has "6 nosniff"       'nosniff' "$HDRS"
has "6 cache-control" 'private, max-age=31536000, immutable' "$HDRS"
eq "6 not a uuid" 400 "$(code GET /photos/not-a-uuid -H "$AU")"
has "6 not a uuid body" '"message":"invalid photo id"' "$(body GET /photos/not-a-uuid -H "$AU")"
eq "6 dotted"     400 "$(code GET /photos/a.b -H "$AU")"
eq "6 traversal"  404 "$(code GET /photos/../../etc/passwd -H "$AU")"
eq "6 missing"    404 "$(code GET /photos/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "6 missing body" '"message":"photo not found"' "$(body GET /photos/11111111-2222-4333-8444-555555555555 -H "$AU")"
eq "6 no token"   401 "$(code GET /photos/$DEF)"
eq "6 range"      206 "$(code GET /photos/$DEF -H "$AU" -H 'Range: bytes=0-9')"

echo "### 7. createPrivateChat"
eq "7 first time"  201 "$(code POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
P=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
eq "7 again"       200 "$(code POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
eq "7 other side"  200 "$(code POST /private_chats -H "$BU" -H "$JS" -d "{\"id\":\"$A\"}")"
eq "7 same chat"   "$P" "$(body POST /private_chats -H "$BU" -H "$JS" -d "{\"id\":\"$A\"}" | id)"
eq "7 with self"   400 "$(code POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$A\"}")"
has "7 with self body" '"message":"cannot open a private chat with yourself"' "$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$A\"}")"
eq "7 unknown user" 404 "$(code POST /private_chats -H "$AU" -H "$JS" -d '{"id":"11111111-2222-4333-8444-555555555555"}')"
has "7 unknown body" '"message":"the other user does not exist"' "$(body POST /private_chats -H "$AU" -H "$JS" -d '{"id":"11111111-2222-4333-8444-555555555555"}')"
eq "7 not a uuid"  400 "$(code POST /private_chats -H "$AU" -H "$JS" -d '{"id":"not-a-uuid"}')"
eq "7 empty body"  400 "$(code POST /private_chats -H "$AU" -H "$JS" -d '{}')"

echo "### 8. createGroup"
GBEFORE=$(ls db/photos | wc -l | tr -d ' ')
DOK="data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json"
DLEAVE="data={\"name\":\"Leave \",\"members\":[\"$B\",\"$C\"]};type=application/json"
DDROP="data={\"name\":\"Drop \",\"members\":[\"$B\"]};type=application/json"
DME="data={\"name\":\"X \",\"members\":[\"$A\"]};type=application/json"
DUNK="data={\"name\":\"X \",\"members\":[\"11111111-2222-4333-8444-555555555555\"]};type=application/json"
DNOPHOTO="data={\"name\":\"X \",\"members\":[\"$B\"]};type=application/json"
eq "8 valid" 201 "$(code POST /groups -H "$AU" -F "$DOK" -F "photoFile=@$PNG")"
G1=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
G2=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
if [ "$G1" != "$G2" ]; then ok; else no "7 same people many groups" "$G1 == $G2"; fi
eq "8 creator inside" 400 "$(code POST /groups -H "$AU" -F "$DME" -F "photoFile=@$PNG")"
has "8 creator body" '"message":"the creator is already a member of the group"' "$(body POST /groups -H "$AU" -F "$DME" -F "photoFile=@$PNG")"
eq "8 unknown member" 404 "$(code POST /groups -H "$AU" -F "$DUNK" -F "photoFile=@$PNG")"
has "8 unknown member body" '"message":"one or more of the members does not exist"' "$(body POST /groups -H "$AU" -F "$DUNK" -F "photoFile=@$PNG")"
eq "8 empty members" 400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[]};type=application/json" -F "photoFile=@$PNG")"
eq "8 duplicates"    400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[\"$B\",\"$B\"]};type=application/json" -F "photoFile=@$PNG")"
eq "8 empty name"    400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG")"
eq "8 no data part"  400 "$(code POST /groups -H "$AU" -F "photoFile=@$PNG")"
# The photo is optional: a group without one starts from the same default a new user starts from
eq "8 no photo part" 201 "$(code POST /groups -H "$AU" -F "$DNOPHOTO")"
GDEF=$(body POST /groups -H "$AU" -F "$DNOPHOTO" | id)
has "8 no photo default" "\"photo\":\"/photos/00000000-0000-4000-8000-000000000000\"" "$(body GET /chats/$GDEF -H "$AU")"
# The default is one shared id and never a file the group owns, so nothing is written for it
PHOTOS_BEFORE=$(ls db/photos | wc -l | tr -d ' ')
body POST /groups -H "$AU" -F "$DNOPHOTO" > /dev/null
eq "8 no photo writes no file" "$PHOTOS_BEFORE" "$(ls db/photos | wc -l | tr -d ' ')"
eq "8 bad photo"     400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@/tmp/text.txt")"
eq "8 no token"      401 "$(code POST /groups -F "data={\"name\":\"X\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG")"

echo "### 9. setGroupName  (reply: GroupChat)"
G=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
GPHOTO=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
NB=$(body PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"CS Study Group"}')
eq  "9 member renames"   200 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"CS Study Group"}')"
eq  "9 exact body"       "{\"id\":\"$G\",\"chatType\":\"group\",\"name\":\"CS Study Group\",\"photo\":\"/photos/$GPHOTO\"}" "$NB"
hasnt "9 no snippet key" 'snippet' "$NB"
hasnt "9 no members key" 'members' "$NB"
eq  "9 other member"     200 "$(code PUT /groups/$G/name -H "$BU" -H "$JS" -d '{"name":"CS Study Group"}')"
eq  "9 100 chars"        200 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d "{\"name\":\"$(printf 'z%.0s' $(seq 100))\"}")"
eq  "9 emoji name"       200 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"👨‍👩‍👧‍👦"}')"
eq  "9 not a member"     403 "$(code PUT /groups/$G/name -H "$CU" -H "$JS" -d '{"name":"Hijacked"}')"
has "9 403 body"         '"message":"not a member of the group"' "$(body PUT /groups/$G/name -H "$CU" -H "$JS" -d '{"name":"Hijacked"}')"
eq  "9 unknown group"    404 "$(code PUT /groups/11111111-2222-4333-8444-555555555555/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
has "9 404 body"         '"message":"group not found"' "$(body PUT /groups/11111111-2222-4333-8444-555555555555/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
eq  "9 private chat id"  404 "$(code PUT /groups/$P/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
eq  "9 not a uuid"       400 "$(code PUT /groups/not-a-uuid/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
has "9 bad id body"      '"message":"invalid group id"' "$(body PUT /groups/not-a-uuid/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
eq  "9 empty name"       400 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":""}')"
eq  "9 spaces name"      400 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"   "}')"
eq  "9 101 chars"        400 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d "{\"name\":\"$(printf 'z%.0s' $(seq 101))\"}")"
eq  "9 GET on path"      405 "$(code GET /groups/$G/name -H "$AU")"
eq  "9 no token"         401 "$(code PUT /groups/$G/name -H "$JS" -d '{"name":"x"}')"
body PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"Kept Name"}' > /dev/null
body PUT /groups/$G/name -H "$CU" -H "$JS" -d '{"name":"Hijacked"}' > /dev/null
eq  "9 403 writes nothing" "Kept Name" "$(sqlite3 db/wasatext.db "SELECT name FROM chats WHERE id='$G';")"
eq  "9 private chat untouched" "NULL|NULL" "$(sqlite3 db/wasatext.db "SELECT quote(name), quote(photoId) FROM chats WHERE id='$P';" | tr -d ' ')"

echo "### 10. setGroupPhoto  (reply: GroupChat)"
PB=$(body PUT /groups/$G/photo -H "$AU" -F "photoFile=@$PNG")
eq  "10 member uploads"   200 "$(code PUT /groups/$G/photo -H "$AU" -F "photoFile=@$PNG")"
has "10 body id"          "\"id\":\"$G\",\"chatType\":\"group\",\"name\":\"Kept Name\",\"photo\":\"/photos/" "$PB"
hasnt "10 no snippet key" 'snippet' "$PB"
hasnt "10 no members key" 'members' "$PB"
Q1=$(body PUT /groups/$G/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
Q2=$(body PUT /groups/$G/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
if [ "$Q1" != "$Q2" ]; then ok; else no "9 new upload new id" "$Q1 == $Q2"; fi
if [ ! -f "db/photos/$Q1" ]; then ok; else no "9 replaced photo collected" "still there"; fi
eq  "10 other member"     200 "$(code PUT /groups/$G/photo -H "$BU" -F "photoFile=@$PNG")"
eq  "10 wrong field"      400 "$(code PUT /groups/$G/photo -H "$AU" -F "file=@$PNG")"
eq  "10 not an image"     400 "$(code PUT /groups/$G/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
eq  "10 empty file"       400 "$(code PUT /groups/$G/photo -H "$AU" -F "photoFile=@/tmp/empty.png")"
eq  "10 not multipart"    400 "$(code PUT /groups/$G/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
PN=$(ls db/photos | wc -l | tr -d ' ')
CUR=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
eq  "10 not a member"     403 "$(code PUT /groups/$G/photo -H "$CU" -F "photoFile=@$PNG")"
eq  "10 unknown group"    404 "$(code PUT /groups/11111111-2222-4333-8444-555555555555/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "10 private chat id"  404 "$(code PUT /groups/$P/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "10 no leaked files"  "$PN" "$(ls db/photos | wc -l | tr -d ' ')"
eq  "10 photo untouched"  "$CUR" "$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")"
eq  "10 not a uuid"       400 "$(code PUT /groups/not-a-uuid/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "10 GET on path"      405 "$(code GET /groups/$G/photo -H "$AU")"
eq  "10 no token"         401 "$(code PUT /groups/$G/photo -F "photoFile=@$PNG")"

echo "### 11. addToGroup  (reply: GroupWithMembers)"
AB=$(body POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")
eq  "11 adds"             200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")"
has "11 has members"      '"members":[' "$AB"
has "11 has chatType"     '"chatType":"group"' "$AB"
hasnt "11 no snippet key" 'snippet' "$AB"
ABM=$(echo "$AB" | grep -o '"members":\[[^]]*\]')
eq  "11 member count"     3 "$(echo "$ABM" | grep -o '"username":' | wc -l | tr -d ' ')"
has "11 member carries a username" '"username":"' "$ABM"
has "11 member photo is a URL"     '"photo":"/photos/' "$ABM"
eq  "11 already inside"   200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")"
eq  "11 no duplicate row" 3 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G';")"
eq  "11 caller itself"    200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$A\"]}")"
eq  "11 empty list"       200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":[]}')"
eq  "11 absent field"     200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{}')"
eq  "11 null field"       200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":null}')"
eq  "11 not a member"     403 "$(code POST /groups/$G/members -H "$DU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "11 empty list 403"   403 "$(code POST /groups/$G/members -H "$DU" -H "$JS" -d '{"members":[]}')"
eq  "11 unknown group"    404 "$(code POST /groups/11111111-2222-4333-8444-555555555555/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "11 private chat id"  404 "$(code POST /groups/$P/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "11 unknown member"   404 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["11111111-2222-4333-8444-555555555555"]}')"
has "11 unknown member body" '"message":"one or more of the members does not exist"' "$(body POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["11111111-2222-4333-8444-555555555555"]}')"
eq  "11 bad member id"    400 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["nope"]}')"
eq  "11 duplicates"       400 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\",\"$D\"]}")"
eq  "11 bad group id"     400 "$(code POST /groups/not-a-uuid/members -H "$AU" -H "$JS" -d '{"members":[]}')"
eq  "11 GET on path"      405 "$(code GET /groups/$G/members -H "$AU")"
eq  "11 no token"         401 "$(code POST /groups/$G/members -H "$JS" -d '{"members":[]}')"
# the cap: fill a fresh group past 100
GF=$(body POST /groups -H "$AU" -F "data={\"name\":\"Full \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
# the 99 fill users of the seed: addToGroup checks UsersExist before it counts, so they must be real
# doLogin is idempotent, so this reads their ids and writes nobody
FILL=""; for i in $(seq -w 1 99); do U=$(body POST /session -H "$JS" -d "{\"username\":\"fill$i\"}" | id); FILL="$FILL\"$U\","; done
eq "11 group full"    400 "$(code POST /groups/$GF/members -H "$AU" -H "$JS" -d "{\"members\":[${FILL%,}]}")"
has "11 group full body" '"message":"the group is full"' "$(body POST /groups/$GF/members -H "$AU" -H "$JS" -d "{\"members\":[${FILL%,}]}")"
eq "11 rollback whole" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GF';")"

echo "### 12. leaveGroup  (reply changed: 204, no body)"
# GL holds A, B and C: only B and C leave below, so the group stands for the rest of the section
GL=$(body POST /groups -H "$AU" -F "$DLEAVE" -F "photoFile=@$PNG" | id)
HDR=$(curl -s -D- -o /dev/null -X DELETE "$H/groups/$GL/members/me" -H "$BU")
has "12 status is 204"    '204' "$HDR"
hasnt "12 no content-type" 'Content-Type' "$HDR"
LB=$(body DELETE /groups/$GL/members/me -H "$CU")
eq  "12 empty body"       "" "$LB"
eq  "12 caller removed"   0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GL' AND userId='$C';")"
eq  "12 A still inside"   1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GL';")"
eq  "12 leave again"      403 "$(code DELETE /groups/$GL/members/me -H "$BU")"
eq  "12 never a member"   403 "$(code DELETE /groups/$GL/members/me -H "$DU")"
eq  "12 unknown group"    404 "$(code DELETE /groups/11111111-2222-4333-8444-555555555555/members/me -H "$AU")"
eq  "12 private chat id"  404 "$(code DELETE /groups/$P/members/me -H "$AU")"
eq  "12 bad id"           400 "$(code DELETE /groups/not-a-uuid/members/me -H "$AU")"
eq  "12 GET on path"      405 "$(code GET /groups/$GL/members/me -H "$AU")"
eq  "12 no token"         401 "$(code DELETE /groups/$GL/members/me)"
# the last member out drops the group and its photo
GD=$(body POST /groups -H "$AU" -F "$DDROP" -F "photoFile=@$PNG" | id)
GDP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$GD';")
eq  "12 first leaver 204" 204 "$(code DELETE /groups/$GD/members/me -H "$AU")"
eq  "12 B still inside" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GD';")"
if [ -f "db/photos/$GDP" ]; then ok; else no "11 photo kept while group stands" "gone"; fi
eq  "12 last leaver 204"  204 "$(code DELETE /groups/$GD/members/me -H "$BU")"
eq  "12 chat row gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats WHERE id='$GD';")"
eq  "12 memberships gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GD';")"
if [ ! -f "db/photos/$GDP" ]; then ok; else no "11 photo collected with group" "still there"; fi
eq  "12 gone is 404"   404 "$(code DELETE /groups/$GD/members/me -H "$AU")"
eq  "12 no stranded chat" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats c WHERE NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId=c.id);")"

echo "### 13. getMyConversations  (reply: GroupSummary | PrivateChatSummary, with the snippet)"
GS=$(body POST /groups -H "$AU" -F "data={\"name\":\"Silent \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
CL=$(body GET /me/chats -H "$AU")
eq  "13 list"          200 "$(code GET /me/chats -H "$AU")"
has "13 has chats"     '"chats":[' "$CL"
has "13 has chatType"  '"chatType":' "$CL"
has "13 silent group has no snippet" "\"id\":\"$GS\",\"chatType\":\"group\",\"name\":\"Silent \"" "$CL"
NEW=$(body POST /session -H "$JS" -d "{\"username\":\"lonely\"}" | id)
eq  "13 no chats"      404 "$(code GET /me/chats -H "Authorization: Bearer $NEW")"
has "13 404 body"      '"message":"no conversations found"' "$(body GET /me/chats -H "Authorization: Bearer $NEW")"
eq  "13 POST on path"  405 "$(code POST /me/chats -H "$AU")"
eq  "13 trailing slash" 404 "$(code GET /me/chats/ -H "$AU")"
eq  "13 no token"      401 "$(code GET /me/chats)"
# the borrow is live, and a group is never multiplied
has "13 private chat named after the other" "alice_new" "$(body GET /me/chats -H "$BU")"
body PUT /me/username -H "$BU" -H "$JS" -d "{\"username\":\"bob_renamed\"}" > /dev/null
has "13 rename reaches the other list" "bob_renamed" "$(body GET /me/chats -H "$AU")"
eq  "13 group appears once" 1 "$(body GET /me/chats -H "$AU" | grep -o "\"id\":\"$G\"" | wc -l | tr -d ' ')"
# the snippet: the messages are the ones sendMessage writes (§15), not rows put in by hand
LONGTEXT=$(printf 'abcdefghij%.0s' $(seq 1 12))
body POST /chats/$G/messages -H "$BU" -F "text=$LONGTEXT" > /dev/null
body POST /chats/$P/messages -H "$BU" -F "photoFile=@$PNG" > /dev/null
SN=$(body GET /me/chats -H "$AU")
CUT50=$(printf 'abcdefghij%.0s' $(seq 1 5))
has "13 text snippet truncated to 50" "\"text\":\"$CUT50\"" "$SN"
has "13 photo snippet emoji" '"emoji":"📷"' "$SN"
has "13 snippet state received" '"state":"received"' "$SN"
# the state moves as the members see the chat, and reads the same for everybody
# every read answers with the states taken before it marks the chat, so each call shows what was
# true when it was asked: B is caught up by having sent, and A and C by opening
has "13 received while C has not opened" '"state":"received"' "$(body GET /chats/$G -H "$AU")"
body GET /chats/$G -H "$CU" > /dev/null
has "13 read once every member has seen it" '"state":"read"' "$(body GET /chats/$G -H "$AU")"
has "13 the snippet carries that state too" '"state":"read"' "$(body GET /me/chats -H "$AU")"
# a message holding an empty text and no photo: sendMessage answers 400 to it, so the row can only be
# written here, and it is what keeps the guard that drops a snippet with nothing to show covered
GE=$(body POST /groups -H "$AU" -F "data={\"name\":\"Empty\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "INSERT INTO messages (id,chatId,userId,text,photoId,date) VALUES
 ('55555555-5555-4555-8555-555555555555','$GE','$B','',NULL,'2099-01-01T00:00:00.000Z');"
EM=$(body GET /me/chats -H "$AU")
has   "13 a chat whose last message shows nothing is still listed" "\"id\":\"$GE\"" "$EM"
hasnt "13 and carries an empty content instead of a snippet" '"content":{}' "$EM"
# ordering: newest first, silent chat last
# written by hand and not sent so the two messages deterministically share the same millisecond.
# Dated ahead of every sent message so they stay the newest of P
sqlite3 db/wasatext.db "INSERT INTO messages (id,chatId,userId,text,photoId,date) VALUES
 ('33333333-3333-4333-8333-333333333333','$P','$B','FIRST in that millisecond',NULL,'2099-01-01T00:00:00.000Z'),
 ('44444444-4444-4444-8444-444444444444','$P','$B','SECOND in that millisecond',NULL,'2099-01-01T00:00:00.000Z');"
ORD=$(body GET /me/chats -H "$AU")
has "13 rowid breaks the tie" 'SECOND in that millisecond' "$ORD"
POS() { echo "$ORD" | grep -o '"id":"[0-9a-f-]*"' | grep -n "$1" | head -1 | cut -d: -f1; }
PG=$(POS "$G"); PP=$(POS "$P"); PS=$(POS "$GS")
if [ -n "$PG" ] && [ -n "$PP" ] && [ -n "$PS" ] && [ "$PS" -gt "$PG" ] && [ "$PS" -gt "$PP" ]; then ok
else no "12 message-less chat sorts after the ones with messages" "P=$PP G=$PG silent=$PS"; fi

echo "### 14. getConversation  (reply: GroupDetail | PrivateChatDetail, with the messages)"
GC=$(body GET /chats/$G -H "$AU")
eq  "14 group 200"        200 "$(code GET /chats/$G -H "$AU")"
has "14 group id"         "\"id\":\"$G\"" "$GC"
has "14 group chatType"   '"chatType":"group"' "$GC"
eq  "14 group answers with the name it owns" "$(sqlite3 db/wasatext.db "SELECT name FROM chats WHERE id='$G';")" "$(echo "$GC" | sed 's/.*"chatType":"group","name":"\([^"]*\)".*/\1/')"
has "14 photo is a URL"   '"photo":"/photos/' "$GC"
has "14 has members"      '"members":[' "$GC"
has "14 has messages"     '"messages":[' "$GC"
hasnt "14 no snippet in the opened chat" '"snippet"' "$GC"
GCM=$(echo "$GC" | grep -o '"members":\[[^]]*\]')
eq  "14 group holds three members" 3 "$(echo "$GCM" | grep -o '"username":' | wc -l | tr -d ' ')"
has "14 member carries a username" '"username":"' "$GCM"
has "14 member photo is a URL"     '"photo":"/photos/' "$GCM"
# a private chat reads the name of the other member, so one row answers each of the two differently
PC=$(body GET /chats/$P -H "$AU")
eq  "14 private 200"      200 "$(code GET /chats/$P -H "$AU")"
has "14 private chatType" '"chatType":"private"' "$PC"
has "14 private named after the other" "\"name\":\"bob_renamed\"" "$PC"
has "14 private name flips" "\"name\":\"alice_new\"" "$(body GET /chats/$P -H "$BU")"
eq  "14 private holds two members" 2 "$(echo "$PC" | grep -o '"members":\[[^]]*\]' | grep -o '"username":' | wc -l | tr -d ' ')"
# newest first, rowid breaking the tie inside the same millisecond, exactly as the snippet of section 13
eq  "14 messages newest first" "SECOND in that millisecond FIRST in that millisecond " "$(echo "$PC" | grep -o 'SECOND in that millisecond\|FIRST in that millisecond' | tr '\n' ' ')"
has "14 text only message"  '"content":{"text":"SECOND in that millisecond"}' "$PC"
has "14 photo only message" '"content":{"photo":"/photos/' "$PC"
hasnt "14 no bare prefix on a message without a photo" '"photo":"/photos/"' "$PC"
has "14 the opened chat carries the whole text, where the snippet cuts it" "\"text\":\"$LONGTEXT\"" "$GC"
# comments inserted by hand here keep this read test independent from the writer exercised in §17
CMIDS="'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'"
sqlite3 db/wasatext.db "DELETE FROM comments WHERE id IN ($CMIDS);"
sqlite3 db/wasatext.db "INSERT INTO comments (id,messageId,userId,emoji) VALUES
 ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','44444444-4444-4444-8444-444444444444','$A','👍'),
 ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','33333333-3333-4333-8333-333333333333','$A','😂');"
PC2=$(body GET /chats/$P -H "$AU")
has "14 a comment lands on its own message" '"content":{"text":"SECOND in that millisecond"},"comments":[{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"' "$PC2"
has "14 no comment is an empty list and never null" '"comments":[]' "$PC2"
# the state is the one of section 13: computed from lastReadDate, the same value for every member
has "14 read on the group"           '"state":"read"' "$GC"
has "14 received on the private chat" '"state":"received"' "$PC"
has "14 a chat with no message answers an empty list" '"messages":[]' "$(body GET /chats/$GS -H "$AU")"
# the one id that answers here and not under /groups/: both kinds are read through /chats/
eq  "14 private chat id under /groups/" 404 "$(code PUT /groups/$P/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
eq  "14 private chat id under /chats/"  200 "$(code GET /chats/$P -H "$AU")"
# one page: a chat answers at most schemas.ChatMessagesPageSize messages, whatever it holds
GP=$(body POST /groups -H "$AU" -F "data={\"name\":\"Page \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "DELETE FROM messages WHERE id LIKE 'page____-0000-4000-8000-000000000000';"
sqlite3 db/wasatext.db "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 501)
 INSERT INTO messages (id,chatId,userId,text,photoId,date)
 SELECT printf('page%04d-0000-4000-8000-000000000000', i), '$GP', '$A', 'm'||i, NULL,
        strftime('%Y-%m-%dT%H:%M:%fZ', datetime('2026-08-24T00:00:00Z', '+'||i||' seconds')) FROM n;"
eq  "14 501 messages answer one page of 500" 500 "$(body GET /chats/$GP -H "$AU" | grep -o '"state":"' | wc -l | tr -d ' ')"
eq  "14 not a member"     403 "$(code GET /chats/$G -H "$DU")"
has "14 403 body"         '"message":"not a member of the chat"' "$(body GET /chats/$G -H "$DU")"
eq  "14 unknown chat"     404 "$(code GET /chats/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "14 404 body"         '"message":"chat not found"' "$(body GET /chats/11111111-2222-4333-8444-555555555555 -H "$AU")"
eq  "14 bad id"           400 "$(code GET /chats/not-a-uuid -H "$AU")"
has "14 400 body"         '"message":"invalid chat id"' "$(body GET /chats/not-a-uuid -H "$AU")"
eq  "14 uppercase uuid"   400 "$(code GET /chats/$(echo $G | tr a-f A-F) -H "$AU")"
eq  "14 POST on path"     405 "$(code POST /chats/$G -H "$AU")"
eq  "14 trailing slash"   404 "$(code GET /chats/$G/ -H "$AU")"
eq  "14 no token"         401 "$(code GET /chats/$G)"
# no row is added or removed: the one thing this read writes is the lastReadDate of its caller
CNT() { sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM chats)||'/'||(SELECT COUNT(*) FROM chat_members)||'/'||(SELECT COUNT(*) FROM messages)||'/'||(SELECT COUNT(*) FROM comments);"; }
LRD() { sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$P' AND userId='$A';"; }
OTH() { sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$P' AND userId='$B';"; }
BEF13=$(CNT); LR13=$(LRD); OT13=$(OTH)
body GET /chats/$G -H "$AU" > /dev/null; body GET /chats/$P -H "$AU" > /dev/null
eq  "14 no row is written" "$BEF13" "$(CNT)"
# opening a chat is what marks it read, and it marks it for the caller alone
if [ "$(LRD)" \> "$LR13" ]; then ok; else no "13 opening a chat marks it read for the caller" "$LR13 -> $(LRD)"; fi
eq  "14 and leaves every other member alone" "$OT13" "$(OTH)"
# a caller that is refused writes nothing at all
LRD_D() { sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G' AND userId='$D';"; }
body GET /chats/$G -H "$DU" > /dev/null
eq  "14 a 403 marks nothing" 0 "$(LRD_D)"

echo "### 15. sendMessage  (reply: Message, 201)"
SM=$(body POST /chats/$G/messages -H "$AU" -F "text=hello everybody")
eq  "15 text only"        201 "$(code POST /chats/$G/messages -H "$AU" -F 'text=another one')"
has "15 reply is a Message" '"content":{"text":"hello everybody"}' "$SM"
has "15 born with no reaction" '"comments":[]' "$SM"
has "15 carries its own date" '"date":"20' "$SM"
eq  "15 photo only"       201 "$(code POST /chats/$G/messages -H "$AU" -F "photoFile=@$PNG")"
BOTH=$(body POST /chats/$G/messages -H "$AU" -F "text=look at this" -F "photoFile=@$PNG")
has "15 text and photo together" '"text":"look at this"' "$BOTH"
has "15 the photo is a URL and never an id" '"photo":"/photos/' "$BOTH"
eq  "15 a private chat takes one too" 201 "$(code POST /chats/$P/messages -H "$AU" -F 'text=hi bob')"
# what a message may not be
eq  "15 neither text nor photo" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'x=1')"
has "15 400 body"        '"message":"empty message content: text or photo is required"' "$(body POST /chats/$G/messages -H "$AU" -F 'x=1')"
eq  "15 spaces alone"    400 "$(code POST /chats/$G/messages -H "$AU" -F 'text=   ')"
eq  "15 empty text"      400 "$(code POST /chats/$G/messages -H "$AU" -F 'text=')"
T5000=$(printf 'z%.0s' $(seq 5000)); T5001=$(printf 'z%.0s' $(seq 5001))
eq  "15 5000 chars"      201 "$(code POST /chats/$G/messages -H "$AU" -F "text=$T5000")"
eq  "15 5001 chars"      400 "$(code POST /chats/$G/messages -H "$AU" -F "text=$T5001")"
has "15 too long body"   '"message":"invalid message text"' "$(body POST /chats/$G/messages -H "$AU" -F "text=$T5001")"
# one grapheme cluster is one char, as everywhere else
eq  "15 an emoji counts one" 201 "$(code POST /chats/$G/messages -H "$AU" -F "text=$(printf '\U0001f468‍\U0001f469‍\U0001f467‍\U0001f466%.0s' $(seq 1 100))")"
# the photo, refused exactly as section 5 refuses it
eq  "15 not an image"    400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt')"
has "15 not an image body" '"message":"invalid photo"' "$(body POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt')"
eq  "15 empty photo"     400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/empty.png')"
eq  "15 over MaxPhotoBytes" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/big.png')"
eq  "15 over the body limit" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/huge.png')"
eq  "15 not multipart"   400 "$(code POST /chats/$G/messages -H "$AU" -H "$JS" -d '{"text":"x"}')"
# a file part under another name is no photo at all, so the text alone decides
eq  "15 wrong field with a text"    201 "$(code POST /chats/$G/messages -H "$AU" -F 'text=ok' -F "file=@$PNG")"
eq  "15 wrong field without a text" 400 "$(code POST /chats/$G/messages -H "$AU" -F "file=@$PNG")"
# who may write, and where
eq  "15 not a member"    403 "$(code POST /chats/$G/messages -H "$DU" -F 'text=x')"
has "15 403 body"        '"message":"not a member of the chat"' "$(body POST /chats/$G/messages -H "$DU" -F 'text=x')"
eq  "15 unknown chat"    404 "$(code POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F 'text=x')"
has "15 404 body"        '"message":"chat not found"' "$(body POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F 'text=x')"
eq  "15 bad chat id"     400 "$(code POST /chats/not-a-uuid/messages -H "$AU" -F 'text=x')"
has "15 bad id body"     '"message":"invalid chat id"' "$(body POST /chats/not-a-uuid/messages -H "$AU" -F 'text=x')"
eq  "15 uppercase uuid"  400 "$(code POST /chats/$(echo $G | tr a-f A-F)/messages -H "$AU" -F 'text=x')"
eq  "15 GET on the path" 405 "$(code GET /chats/$G/messages -H "$AU")"
eq  "15 trailing slash"  404 "$(code POST /chats/$G/messages/ -H "$AU" -F 'text=x')"
eq  "15 no token"        401 "$(code POST /chats/$G/messages -F 'text=x')"
# nothing is left behind by a refusal, whatever the reason and however late it fires
PB=$(ls db/photos | wc -l | tr -d ' ')
code POST /chats/$G/messages -H "$DU" -F "photoFile=@$PNG" > /dev/null
code POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F "photoFile=@$PNG" > /dev/null
code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt' > /dev/null
eq  "15 a refused send leaks no photo" "$PB" "$(ls db/photos | wc -l | tr -d ' ')"
eq  "15 every stored photo is on disk" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE photoId IS NOT NULL AND photoId NOT IN ($(ls db/photos | sed "s/.*/'&'/" | paste -sd, -));")"
# the sender is caught up to its own message, which is what keeps it from answering for it
GX=$(body POST /groups -H "$AU" -F "data={\"name\":\"Stamp\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
MD=$(body POST /chats/$GX/messages -H "$AU" -F 'text=stamp' | sed 's/.*"date":"\([^"]*\)".*/\1/')
# Plain time.Time keeps the same instant; JSON may trim trailing zeroes from the fixed-width DB value
eq  "15 the reply date is the stored instant" 1 \
    "$(sqlite3 db/wasatext.db "SELECT julianday('$MD') = julianday((SELECT date FROM messages WHERE chatId='$GX' ORDER BY rowid DESC LIMIT 1));")"
eq  "15 and it is a valid date" 1 "$(sqlite3 db/wasatext.db "SELECT julianday('$MD') IS NOT NULL;")"
# read before the chat is opened below: opening it moves the caller lastReadDate forward
eq  "15 the sender lastReadDate is the date of its message" \
    "$(sqlite3 db/wasatext.db "SELECT date FROM messages WHERE chatId='$GX' ORDER BY rowid DESC LIMIT 1;")" \
    "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$GX' AND userId='$A';")"
eq  "15 the same date comes back from getConversation" "$MD" \
    "$(body GET /chats/$GX -H "$AU" | sed 's/.*"date":"\([^"]*\)".*/\1/')"
# the state of a new message is computed and not assumed
has "15 received while somebody else is behind" '"state":"received"' "$(body POST /chats/$GX/messages -H "$AU" -F 'text=still received')"
# e.g.1 a chat the sender is alone in: there is nobody to be behind, so the message is born read
G1=$(body POST /groups -H "$AU" -F "data={\"name\":\"Solo\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
code DELETE /groups/$G1/members/me -H "$BU" > /dev/null
has "15 e.g.1 alone in the chat, born read" '"state":"read"' "$(body POST /chats/$G1/messages -H "$AU" -F 'text=alone')"
# e.g.2 the member that never opened it leaves, and the message it was holding becomes read
G2=$(body POST /groups -H "$AU" -F "data={\"name\":\"Three\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$G2/messages -H "$AU" -F 'text=hi all' > /dev/null
body GET /chats/$G2 -H "$BU" > /dev/null
has "15 e.g.2 received while C has not opened" '"state":"received"' "$(body GET /chats/$G2 -H "$AU")"
code DELETE /groups/$G2/members/me -H "$CU" > /dev/null
has "15 e.g.2 read once the one behind it leaves" '"state":"read"' "$(body GET /chats/$G2 -H "$AU")"
# e.g.3 read is a black hole: a member joining answers for nothing sent before it
G3=$(body POST /groups -H "$AU" -F "data={\"name\":\"Hole\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$G3/messages -H "$AU" -F 'text=read me' > /dev/null
body GET /chats/$G3 -H "$BU" > /dev/null; body GET /chats/$G3 -H "$CU" > /dev/null
has "15 e.g.3 read once every member has seen it" '"state":"read"' "$(body GET /chats/$G3 -H "$AU")"
code POST /groups/$G3/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}" > /dev/null
has "15 e.g.3 and still read after somebody joins" '"state":"read"' "$(body GET /chats/$G3 -H "$AU")"
hasnt "15 e.g.3 a join never turns a message back" '"state":"received"' "$(body GET /chats/$G3 -H "$AU")"
# a chat holds at most schemas.ChatMaxMessages: seeded by hand, 9999 calls would be absurd
GF=$(body POST /groups -H "$AU" -F "data={\"name\":\"Full\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 9999)
 INSERT INTO messages (id,chatId,userId,text,photoId,date)
 SELECT printf('%08d-0000-4000-8000-000000000000', i), '$GF', '$A', 'm'||i, NULL,
        strftime('%Y-%m-%dT%H:%M:%fZ', datetime('2020-01-01T00:00:00Z', '+'||i||' seconds')) FROM n;"
eq  "15 the 10000th message fits"  201 "$(code POST /chats/$GF/messages -H "$AU" -F 'text=the last one')"
eq  "15 the 10001st does not"      400 "$(code POST /chats/$GF/messages -H "$AU" -F 'text=one too many')"
has "15 full body"       '"message":"the chat is full"' "$(body POST /chats/$GF/messages -H "$AU" -F 'text=one too many')"
eq  "15 and the refused one wrote no row" 10000 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$GF';")"
# a message reaches the two reads it belongs to
GN=$(body POST /groups -H "$AU" -F "data={\"name\":\"Newest\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
GO=$(body POST /groups -H "$AU" -F "data={\"name\":\"Older\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$GO/messages -H "$AU" -F 'text=the older thing' > /dev/null
body POST /chats/$GN/messages -H "$AU" -F 'text=the newest thing' > /dev/null
has "15 it reaches the opened chat"  'the newest thing' "$(body GET /chats/$GN -H "$AU")"
has "15 and the snippet of the list" 'the newest thing' "$(body GET /me/chats -H "$AU")"
# sending moves a chat up the list, which is the whole sort key of §13
# the two are compared against each other and not against the head of the list, which §13 pins ahead of both
RANK() { body GET /me/chats -H "$AU" | grep -o '"id":"[0-9a-f-]*"' | grep -n "$1" | head -1 | cut -d: -f1; }
if [ "$(RANK $GN)" -lt "$(RANK $GO)" ]; then ok; else no "14 the chat written last comes first" "GN=$(RANK $GN) GO=$(RANK $GO)"; fi
body POST /chats/$GO/messages -H "$AU" -F 'text=and now this one' > /dev/null
if [ "$(RANK $GO)" -lt "$(RANK $GN)" ]; then ok; else no "14 and writing again moves it back up" "GO=$(RANK $GO) GN=$(RANK $GN)"; fi

# replying — the optional replyTo form field, which is why there is no reply endpoint:
# an answer is this same request carrying one more field
# Its own chats, so the lifecycle below disturbs no other section
RG=$(body POST /groups -H "$AU" -F "data={\"name\":\"Quotes\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
RG2=$(body POST /groups -H "$AU" -F "data={\"name\":\"Elsewhere\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
RQ=$(body POST /chats/$RG/messages -H "$BU" -F 'text=quote me' | id)
RELSE=$(body POST /chats/$RG2/messages -H "$AU" -F 'text=another chat' | id)
RANS=$(body POST /chats/$RG/messages -H "$AU" -F 'text=quoting you' -F "replyTo=$RQ")
eq  "15 reply accepted"       201 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=quoting you' -F "replyTo=$RQ")"
has "15 reply carries replyTo" "\"replyTo\":\"$RQ\"" "$RANS"
# omitempty, never null: a message answering none simply has no field
hasnt "15 no replyTo means no field" '"replyTo"' "$(body POST /chats/$RG/messages -H "$AU" -F 'text=plain')"
# an empty field is the same as no field at all, and not a malformed id
eq    "15 empty replyTo is no reply" 201 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=plain' -F 'replyTo=')"
hasnt "15 empty replyTo writes no field" '"replyTo"' "$(body POST /chats/$RG/messages -H "$AU" -F 'text=plain' -F 'replyTo=')"
# an answer needs no text of its own
has "15 a photo may answer too" "\"replyTo\":\"$RQ\"" "$(body POST /chats/$RG/messages -H "$AU" -F "photoFile=@$PNG" -F "replyTo=$RQ")"
# a quote never crosses a chat: chatId is part of the condition, so from here the two are the same nothing
eq  "15 replyTo from another chat" 404 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F "replyTo=$RELSE")"
has "15 cross-chat body"      '"message":"replied message not found"' "$(body POST /chats/$RG/messages -H "$AU" -F 'text=x' -F "replyTo=$RELSE")"
eq  "15 replyTo owned by nothing" 404 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F 'replyTo=11111111-2222-4333-8444-555555555555')"
eq  "15 replyTo not a uuid"   400 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F 'replyTo=not-a-uuid')"
has "15 bad reply id body"    '"message":"invalid reply id"' "$(body POST /chats/$RG/messages -H "$AU" -F 'text=x' -F 'replyTo=not-a-uuid')"
eq  "15 replyTo uppercase uuid" 400 "$(code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F "replyTo=$(echo $RQ | tr a-f A-F)")"
# a refused reply is refused before the row: the chat holds what it held
RCOUNT=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$RG';")
code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F "replyTo=$RELSE" > /dev/null
code POST /chats/$RG/messages -H "$AU" -F 'text=x' -F 'replyTo=not-a-uuid' > /dev/null
eq  "15 a refused reply writes no row" "$RCOUNT" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$RG';")"
# an answer is a message of its own, so it may be answered in turn
RCHAIN=$(body POST /chats/$RG/messages -H "$BU" -F 'text=answering the answer' -F "replyTo=$(printf %s "$RANS" | id)")
has "15 a reply may be replied to" "\"replyTo\":\"$(printf %s "$RANS" | id)\"" "$RCHAIN"
# §16: the copy leaves the quote behind — the message it named is not in the destination
hasnt "15 a forwarded reply drops its quote" '"replyTo"' "$(body POST /chats/$RG2/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$(printf %s "$RANS" | id)\"}")"
# §17: reacting gives the message back whole
has "15 commenting keeps replyTo" "\"replyTo\":\"$RQ\"" "$(body PUT /chats/$RG/messages/$(printf %s "$RANS" | id)/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
# §20: ON DELETE SET NULL — retracting one message must not retract the answer somebody else wrote to it
eq "15 the quoted message is deleted" 204 "$(code DELETE /chats/$RG/messages/$RQ -H "$BU")"
RAFTER=$(body GET /chats/$RG -H "$AU")
has   "15 the answer survives the delete" "$(printf %s "$RANS" | id)" "$RAFTER"
hasnt "15 and its quote is gone"   "\"replyTo\":\"$RQ\"" "$RAFTER"
eq "15 no row points at the deleted quote" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE replyTo='$RQ';")"

echo "### 16. forwardMessage  (reply: Message, 201)"
# A member of both chats, B source-only, C destination-only, D neither — same fixture shape as §16 of endpoints.md
PS=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
PD=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$C\"}" | id)
SRC_TEXT=$(body POST /chats/$PS/messages -H "$BU" -F 'text=forward this text')
MS=$(echo "$SRC_TEXT" | id)
SRC_PHOTO=$(body POST /chats/$PS/messages -H "$BU" -F "photoFile=@$PNG")
MPH=$(echo "$SRC_PHOTO" | id)
MPHOTO=$(echo "$SRC_PHOTO" | sed 's/.*"photo":"\([^"]*\)".*/\1/')
SRC_BOTH=$(body POST /chats/$PS/messages -H "$BU" -F 'text=both fields' -F "photoFile=@$PNG")
MB=$(echo "$SRC_BOTH" | id)
MBPHOTO=$(echo "$SRC_BOTH" | sed 's/.*"photo":"\([^"]*\)".*/\1/')
SRC_ROW=$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||quote(photoId)||'|'||date FROM messages WHERE id='$MS';")
PHOTOS15=$(ls db/photos | wc -l | tr -d ' ')

FW_FILE=/tmp/wasatext-forward-response-$$.json
FW_CODE=$(curl -s -o "$FW_FILE" -w '%{http_code}' -X POST "$H/chats/$PD/forwards" \
  -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")
FW=$(tr -d '\n' < "$FW_FILE")
eq  "16 text-only source"        201 "$FW_CODE"
has "16 text copied"             '"content":{"text":"forward this text"}' "$FW"
hasnt "16 no photo key on text-only" '"photo"' "$FW"
has "16 born with no reaction"   '"comments":[]' "$FW"
has "16 carries its own date"    '"date":"20' "$FW"
has "16 user is the caller"      "\"user\":\"$A\"" "$FW"
hasnt "16 user is not the source sender" "\"user\":\"$B\"" "$FW"
if [ "$(echo "$FW" | id)" != "$MS" ]; then ok; else no "15 new id, not the source id" "$MS"; fi
FWID=$(echo "$FW" | id)
FWDATE=$(echo "$FW" | sed 's/.*"date":"\([^"]*\)".*/\1/')
has "16 received while C is behind" '"state":"received"' "$FW"
eq "16 reply date is the stored instant" 1 \
   "$(sqlite3 db/wasatext.db "SELECT julianday('$FWDATE') = julianday((SELECT date FROM messages WHERE id='$FWID'));")"
eq "16 sender caught up to the exact copy date" \
   "$(sqlite3 db/wasatext.db "SELECT date FROM messages WHERE id='$FWID';")" \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$PD' AND userId='$A';")"

FWP=$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MPH\"}")
has   "16 exact photo URL copied"     "\"photo\":\"$MPHOTO\"" "$FWP"
hasnt "16 no text key on photo-only" '"text"' "$FWP"

FWB=$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MB\"}")
has "16 both fields copied, text"  '"text":"both fields"' "$FWB"
has "16 both fields copied, exact photo" "\"photo\":\"$MBPHOTO\"" "$FWB"
eq  "16 forwarding creates no photo file" "$PHOTOS15" "$(ls db/photos | wc -l | tr -d ' ')"

# comments belong to the source row and are not copied
sqlite3 db/wasatext.db "INSERT INTO comments (id, messageId, userId, emoji) VALUES ('99999999-0000-4000-8000-000000000000', '$MS', '$B', '👍');"
FWC=$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")
FWCID=$(echo "$FWC" | id)
has "16 source comments are not copied" '"comments":[]' "$FWC"
eq  "16 copied row owns no comment" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$FWCID';")"
eq  "16 source keeps its comment"   1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$MS';")"
eq  "16 source message stays untouched" "$SRC_ROW" \
    "$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||quote(photoId)||'|'||date FROM messages WHERE id='$MS';")"

# forwarding into the source chat itself is allowed, and any chat kind is a valid destination
eq "16 same-chat forward" 201 "$(code POST /chats/$PS/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq "16 group destination"  201 "$(code POST /chats/$G/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
# a destination with no other member has nobody to hold the copy at received
GFS=$(body POST /groups -H "$AU" -F "data={\"name\":\"Forward Solo\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
code DELETE /groups/$GFS/members/me -H "$BU" > /dev/null
has "16 alone in destination, born read" '"state":"read"' \
    "$(body POST /chats/$GFS/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"

# who may forward, and between which chats
eq  "16 belongs to source and destination" 201 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
REFUSED_BEFORE=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE chatId='$PD')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$PD' ORDER BY userId));")
eq  "16 source-only member"      403 "$(code POST /chats/$PD/forwards -H "$BU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "16 destination-only member" 403 "$(code POST /chats/$PD/forwards -H "$CU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "16 belongs to neither"      403 "$(code POST /chats/$PD/forwards -H "$DU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 403 body" '"message":"not a member of the chat"' "$(body POST /chats/$PD/forwards -H "$DU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "16 every 403 writes nothing" "$REFUSED_BEFORE" \
    "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE chatId='$PD')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$PD' ORDER BY userId));")"

# not found: message before chat
NOTFOUND_BEFORE=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")
eq  "16 unknown message"  404 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{"messageId":"11111111-2222-4333-8444-555555555555"}')"
has "16 message 404 body" '"message":"message not found"' "$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{"messageId":"11111111-2222-4333-8444-555555555555"}')"
eq  "16 unknown chat"     404 "$(code POST /chats/11111111-2222-4333-8444-555555555555/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 chat 404 body"    '"message":"chat not found"' "$(body POST /chats/11111111-2222-4333-8444-555555555555/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 missing source wins over missing destination" '"message":"message not found"' \
    "$(body POST /chats/11111111-2222-4333-8444-555555555555/forwards -H "$AU" -H "$JS" -d '{"messageId":"22222222-2222-4222-8222-222222222222"}')"
eq  "16 every 404 writes no message" "$NOTFOUND_BEFORE" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")"

# the body and the ids
eq  "16 empty messageId"    400 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{"messageId":""}')"
eq  "16 missing messageId"  400 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{}')"
has "16 400 body"           '"message":"invalid request body"' "$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{}')"
eq  "16 malformed body"     400 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d '{')"
eq  "16 uppercase messageId" 400 "$(code POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$(echo $MS | tr a-f A-F)\"}")"
eq  "16 bad chat id"        400 "$(code POST /chats/not-a-uuid/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 bad chat id body"   '"message":"invalid chat id"' "$(body POST /chats/not-a-uuid/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "16 uppercase chat id"  400 "$(code POST /chats/$(echo $PD | tr a-f A-F)/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 bad destination id wins over malformed body" '"message":"invalid chat id"' \
    "$(body POST /chats/not-a-uuid/forwards -H "$AU" -H "$JS" -d '{')"
EXTRA_CODE=$(curl -s -o "$FW_FILE" -w '%{http_code}' -X POST "$H/chats/$PD/forwards" \
  -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\",\"admin\":true}")
EXTRA=$(tr -d '\n' < "$FW_FILE")
eq  "16 unknown JSON field is ignored" 201 "$EXTRA_CODE"
has "16 extra field still returns the copy" '"text":"forward this text"' "$EXTRA"

# the destination cap is checked here too — GF is already full from §15
eq  "16 destination full"  400 "$(code POST /chats/$GF/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "16 full body"         '"message":"the chat is full"' "$(body POST /chats/$GF/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "16 and the refused one wrote no row" 10000 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$GF';")"

# router-level answers on this route
eq "16 GET on the path" 405 "$(code GET /chats/$PD/forwards -H "$AU")"
eq "16 trailing slash"  404 "$(code POST /chats/$PD/forwards/ -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq "16 no token wins over malformed body" 401 "$(code POST /chats/$PD/forwards -H "$JS" -d '{')"

echo "### 17. commentMessage  (reply: updated Message, 201 create / 200 update)"
# A, B and C share the target group; B writes the message and D is the outsider
CG=$(body POST /groups -H "$AU" -F "data={\"name\":\"Comments\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
CM_SRC=$(body POST /chats/$CG/messages -H "$BU" -F 'text=react to this' -F "photoFile=@$PNG")
CMID=$(echo "$CM_SRC" | id)
CMPHOTO=$(echo "$CM_SRC" | sed 's/.*"photo":"\([^"]*\)".*/\1/')
CMROW=$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||photoId||'|'||date FROM messages WHERE id='$CMID';")
CMCOUNT=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")
PHOTOS16=$(ls db/photos | wc -l | tr -d ' ')
CP=/chats/$CG/messages/$CMID/comments/me

# Put A visibly behind, then commenting must catch A up while C still holds the message at received
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2000-01-01T00:00:00.000Z' WHERE chatId='$CG' AND userId='$A';"
CM_FILE=/tmp/wasatext-comment-response-$$.json
CM_CODE=$(curl -s -o "$CM_FILE" -w '%{http_code}' -X PUT "$H$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍"}')
CMR=$(tr -d '\n' < "$CM_FILE")
eq  "17 first reaction" 201 "$CM_CODE"
has "17 reply is the original Message" "\"id\":\"$CMID\",\"user\":\"$B\"" "$CMR"
has "17 text stays on the message" '"text":"react to this"' "$CMR"
has "17 exact photo stays on the message" "\"photo\":\"$CMPHOTO\"" "$CMR"
has "17 caller's reaction is in the list" "\"user\":\"$A\",\"emoji\":\"👍\"" "$CMR"
has "17 state remains received while C is behind" '"state":"received"' "$CMR"
CAID=$(echo "$CMR" | sed 's/.*"comments":\[{"id":"\([^"]*\)".*/\1/')
eq  "17 one row for A" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$CMID' AND userId='$A';")"
eq  "17 A is caught up by commenting" 1 \
    "$(sqlite3 db/wasatext.db "SELECT lastReadDate >= (SELECT date FROM messages WHERE id='$CMID') FROM chat_members WHERE chatId='$CG' AND userId='$A';")"
eq  "17 message row is untouched" "$CMROW" \
    "$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||photoId||'|'||date FROM messages WHERE id='$CMID';")"
eq  "17 commenting adds no message" "$CMCOUNT" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")"
eq  "17 commenting writes no photo" "$PHOTOS16" "$(ls db/photos | wc -l | tr -d ' ')"

# A second PUT updates A's one row: the reaction id stays stable and no second row is added
UP_CODE=$(curl -s -o "$CM_FILE" -w '%{http_code}' -X PUT "$H$CP" -H "$AU" -H "$JS" -d '{"emoji":"😂"}')
UPR=$(tr -d '\n' < "$CM_FILE")
eq  "17 existing reaction update" 200 "$UP_CODE"
NEW_CAID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$A';")
eq  "17 update preserves the comment id" "$CAID" "$NEW_CAID"
eq  "17 update keeps one row" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$CMID' AND userId='$A';")"
eq  "17 update stores the new emoji" '😂' "$(sqlite3 db/wasatext.db "SELECT emoji FROM comments WHERE messageId='$CMID' AND userId='$A';")"
has "17 update reply has the new emoji" '"emoji":"😂"' "$UPR"
hasnt "17 update reply drops the old emoji" '"emoji":"👍"' "$UPR"

# lastReadDate is monotonic even if the local clock is behind a value already stored
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2099-01-01T00:00:00.000Z' WHERE chatId='$CG' AND userId='$A';"
body PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"🔥"}' > /dev/null
eq "17 commenting never moves lastReadDate backwards" '2099-01-01T00:00:00.000Z' \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$CG' AND userId='$A';")"

# Other members get their own rows; when C reacts, every member has seen the message and its state is read
has "17 B adds a second user's reaction" "\"user\":\"$B\",\"emoji\":\"🎉\"" \
    "$(body PUT "$CP" -H "$BU" -H "$JS" -d '{"emoji":"🎉"}')"
CM_C=$(body PUT "$CP" -H "$CU" -H "$JS" -d '{"emoji":"❤️"}')
eq  "17 one reaction per each of three members" 3 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$CMID';")"
has "17 C's reaction is returned" "\"user\":\"$C\",\"emoji\":\"❤️\"" "$CM_C"
has "17 state becomes read when C catches up" '"state":"read"' "$CM_C"

# Both chat kinds use the same endpoint: FWID is a message in the A+C private destination from §16
eq "17 private chat message" 201 "$(code PUT /chats/$PD/messages/$FWID/comments/me -H "$CU" -H "$JS" -d '{"emoji":"👍"}')"

# Membership and ownership failures are transactional
REF16=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM comments)||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$CG' ORDER BY userId));")
eq  "17 outsider" 403 "$(code PUT "$CP" -H "$DU" -H "$JS" -d '{"emoji":"👍"}')"
has "17 outsider body" '"message":"not a member of the chat"' "$(body PUT "$CP" -H "$DU" -H "$JS" -d '{"emoji":"👍"}')"
eq  "17 message under the wrong chat" 404 "$(code PUT /chats/$PD/messages/$CMID/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
has "17 wrong chat body" '"message":"chat not found"' "$(body PUT /chats/$PD/messages/$CMID/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
eq  "17 unknown message" 404 "$(code PUT /chats/$CG/messages/11111111-2222-4333-8444-555555555555/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
has "17 unknown message body" '"message":"message not found"' "$(body PUT /chats/$CG/messages/11111111-2222-4333-8444-555555555555/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
has "17 missing message wins over missing chat" '"message":"message not found"' \
    "$(body PUT /chats/11111111-2222-4333-8444-555555555555/messages/22222222-2222-4222-8222-222222222222/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
eq "17 refused calls write nothing" "$REF16" \
   "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM comments)||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$CG' ORDER BY userId));")"

# URL ids are validated before the JSON body, chat first and then message
eq  "17 bad chat id" 400 "$(code PUT /chats/not-a-uuid/messages/$CMID/comments/me -H "$AU" -H "$JS" -d '{')"
has "17 bad chat id body" '"message":"invalid chat id"' "$(body PUT /chats/not-a-uuid/messages/$CMID/comments/me -H "$AU" -H "$JS" -d '{')"
eq  "17 uppercase chat id" 400 "$(code PUT /chats/$(echo $CG | tr a-f A-F)/messages/$CMID/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
eq  "17 bad message id" 400 "$(code PUT /chats/$CG/messages/not-a-uuid/comments/me -H "$AU" -H "$JS" -d '{')"
has "17 bad message id body" '"message":"invalid message id"' "$(body PUT /chats/$CG/messages/not-a-uuid/comments/me -H "$AU" -H "$JS" -d '{')"
eq  "17 uppercase message id" 400 "$(code PUT /chats/$CG/messages/$(echo $CMID | tr a-f A-F)/comments/me -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"

# Emoji is one grapheme cluster capped at 16 code points; it is not restricted to Unicode's emoji list
eq  "17 missing emoji" 400 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{}')"
eq  "17 empty emoji" 400 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":""}')"
eq  "17 two symbols" 400 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍😂"}')"
eq  "17 malformed body" 400 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{')"
E16="a$(printf '\u0301%.0s' $(seq 1 15))"
E17="a$(printf '\u0301%.0s' $(seq 1 16))"
eq  "17 one grapheme of 16 code points" 200 "$(code PUT "$CP" -H "$AU" -H "$JS" -d "{\"emoji\":\"$E16\"}")"
eq  "17 one grapheme of 17 code points" 400 "$(code PUT "$CP" -H "$AU" -H "$JS" -d "{\"emoji\":\"$E17\"}")"
eq  "17 a non-emoji symbol is accepted" 200 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"a"}')"
eq  "17 family emoji is one symbol" 200 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👨‍👩‍👧‍👦"}')"
eq  "17 unknown JSON field is ignored" 200 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍","admin":true}')"

# Authentication and router-level answers
eq "17 no token wins over malformed ids/body" 401 "$(code PUT /chats/nope/messages/nope/comments/me -H "$JS" -d '{')"
eq "17 POST on the path" 405 "$(code POST "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
eq "17 GET on the path" 405 "$(code GET "$CP" -H "$AU")"
eq "17 trailing slash" 404 "$(code PUT "$CP/" -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"

echo "### 18. uncommentMessage  (reply: 204, no body)"
# Section 17 left one reaction from A, B and C on CMID. Removing A's must leave B and C untouched.
UBID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$B';")
UCID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$C';")
UCMCOUNT=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")
UPHOTOS=$(ls db/photos | wc -l | tr -d ' ')
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2000-01-01T00:00:00.000Z' WHERE chatId='$CG' AND userId='$A';"
UC_FILE=/tmp/wasatext-uncomment-response-$$.txt
UC_CODE=$(curl -s -o "$UC_FILE" -w '%{http_code}' -X DELETE "$H$CP" -H "$AU")
eq "18 own reaction removed" 204 "$UC_CODE"
eq "18 success body is empty" 0 "$(wc -c < "$UC_FILE" | tr -d ' ')"
eq "18 caller's row is gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$CMID' AND userId='$A';")"
eq "18 the other two rows remain" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$CMID';")"
eq "18 B's comment id is untouched" "$UBID" "$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$B';")"
eq "18 C's comment id is untouched" "$UCID" "$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$C';")"
eq "18 successful uncomment catches A up" 1 \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate >= (SELECT date FROM messages WHERE id='$CMID') FROM chat_members WHERE chatId='$CG' AND userId='$A';")"
eq "18 message row is untouched" "$CMROW" \
   "$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||photoId||'|'||date FROM messages WHERE id='$CMID';")"
eq "18 uncommenting removes no message" "$UCMCOUNT" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")"
eq "18 uncommenting removes no photo" "$UPHOTOS" "$(ls db/photos | wc -l | tr -d ' ')"
UC_CHAT=$(body GET /chats/$CG -H "$CU")
has "18 opened chat keeps B's reaction" "\"user\":\"$B\",\"emoji\":\"🎉\"" "$UC_CHAT"
has "18 opened chat keeps C's reaction" "\"user\":\"$C\",\"emoji\":\"❤️\"" "$UC_CHAT"
hasnt "18 opened chat drops A's reaction" "\"user\":\"$A\"" "$UC_CHAT"

# Once the resource is deleted, PUT creates a new one with a new identity. A later DELETE still cannot move time backwards.
RECREATE_CODE=$(curl -s -o "$CM_FILE" -w '%{http_code}' -X PUT "$H$CP" -H "$AU" -H "$JS" -d '{"emoji":"🔥"}')
eq "18 recreating the deleted reaction" 201 "$RECREATE_CODE"
RECREATED_AID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$CMID' AND userId='$A';")
if [ "$RECREATED_AID" != "$CAID" ]; then ok; else no "17 recreation gets a new id" "$CAID"; fi
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2099-01-01T00:00:00.000Z' WHERE chatId='$CG' AND userId='$A';"
eq "18 remove the recreated reaction" 204 "$(code DELETE "$CP" -H "$AU")"
eq "18 uncommenting never moves lastReadDate backwards" '2099-01-01T00:00:00.000Z' \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$CG' AND userId='$A';")"
eq "18 removing an absent own comment" 404 "$(code DELETE "$CP" -H "$AU")"
has "18 absent own comment body" '"message":"comment not found"' "$(body DELETE "$CP" -H "$AU")"

# Both chat kinds use the same operation: C owns the private-chat reaction created in section 17.
PRIVATE_CP=/chats/$PD/messages/$FWID/comments/me
eq "18 private chat reaction removed" 204 "$(code DELETE "$PRIVATE_CP" -H "$CU")"
eq "18 private comment row is gone" 0 \
   "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$FWID' AND userId='$C';")"

# Every refused request is read-only, including the 404 for a member who owns no comment.
REF17=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM comments)||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$CG' ORDER BY userId));")
eq "18 outsider" 403 "$(code DELETE "$CP" -H "$DU")"
has "18 outsider body" '"message":"not a member of the chat"' "$(body DELETE "$CP" -H "$DU")"
eq "18 message under the wrong chat" 404 "$(code DELETE /chats/$PD/messages/$CMID/comments/me -H "$AU")"
has "18 wrong chat body" '"message":"chat not found"' "$(body DELETE /chats/$PD/messages/$CMID/comments/me -H "$AU")"
eq "18 unknown message" 404 "$(code DELETE /chats/$CG/messages/11111111-2222-4333-8444-555555555555/comments/me -H "$AU")"
has "18 unknown message body" '"message":"message not found"' \
    "$(body DELETE /chats/$CG/messages/11111111-2222-4333-8444-555555555555/comments/me -H "$AU")"
has "18 missing message wins over missing chat" '"message":"message not found"' \
    "$(body DELETE /chats/11111111-2222-4333-8444-555555555555/messages/22222222-2222-4222-8222-222222222222/comments/me -H "$AU")"
eq "18 member without a comment" 404 "$(code DELETE "$CP" -H "$AU")"
has "18 member without a comment body" '"message":"comment not found"' "$(body DELETE "$CP" -H "$AU")"
eq "18 refused calls write nothing" "$REF17" \
   "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM comments)||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$CG' ORDER BY userId));")"

# URL ids are validated by the handler, chat first and then message; DELETE has no request body to validate.
eq "18 bad chat id" 400 "$(code DELETE /chats/not-a-uuid/messages/$CMID/comments/me -H "$AU")"
has "18 bad chat id body" '"message":"invalid chat id"' "$(body DELETE /chats/not-a-uuid/messages/$CMID/comments/me -H "$AU")"
eq "18 uppercase chat id" 400 "$(code DELETE /chats/$(echo $CG | tr a-f A-F)/messages/$CMID/comments/me -H "$AU")"
eq "18 bad message id" 400 "$(code DELETE /chats/$CG/messages/not-a-uuid/comments/me -H "$AU")"
has "18 bad message id body" '"message":"invalid message id"' "$(body DELETE /chats/$CG/messages/not-a-uuid/comments/me -H "$AU")"
eq "18 uppercase message id" 400 "$(code DELETE /chats/$CG/messages/$(echo $CMID | tr a-f A-F)/comments/me -H "$AU")"

# Authentication and route shape.
eq "18 no token wins over malformed ids" 401 "$(code DELETE /chats/nope/messages/nope/comments/me)"
eq "18 trailing slash" 404 "$(code DELETE "$CP/" -H "$AU")"

echo "### 19. deleteMessage  (reply: empty, 204)"
DMG=$(body POST /groups -H "$AU" -F "data={\"name\":\"Delete tests\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
DM_PREV=$(body POST /chats/$DMG/messages -H "$BU" -F 'text=previous message' | id)
DMID=$(body POST /chats/$DMG/messages -H "$AU" -F 'text=delete this message' | id)
DM_PATH=/chats/$DMG/messages/$DMID
eq "19 first comment fixture" 201 "$(code PUT "$DM_PATH/comments/me" -H "$BU" -H "$JS" -d '{"emoji":"👍"}')"
eq "19 second comment fixture" 201 "$(code PUT "$DM_PATH/comments/me" -H "$CU" -H "$JS" -d '{"emoji":"😂"}')"
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2000-01-01T00:00:00.000Z' WHERE chatId='$DMG' AND userId='$A';"
DM_FILE=/tmp/wasatext-delete-message-response-$$.txt
DM_CODE=$(curl -s -o "$DM_FILE" -w '%{http_code}' -X DELETE "$H$DM_PATH" -H "$AU")
eq "19 sender deletes group message" 204 "$DM_CODE"
eq "19 204 body is empty" 0 "$(wc -c < "$DM_FILE" | tr -d ' ')"
eq "19 message row is gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE id='$DMID';")"
eq "19 comments cascade" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$DMID';")"
eq "19 previous message remains" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE id='$DM_PREV';")"
eq "19 deletion catches sender up" 1 \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate > '2000-01-01T00:00:00.000Z' FROM chat_members WHERE chatId='$DMG' AND userId='$A';")"
DM_OPEN=$(body GET /chats/$DMG -H "$AU")
has "19 opened chat keeps previous message" "\"id\":\"$DM_PREV\"" "$DM_OPEN"
hasnt "19 opened chat drops deleted message" "\"id\":\"$DMID\"" "$DM_OPEN"
DM_LIST=$(body GET /me/chats -H "$AU")
has "19 preview falls back to previous message" "\"snippet\":{\"id\":\"$DM_PREV\"" "$DM_LIST"
hasnt "19 preview drops deleted message" "\"snippet\":{\"id\":\"$DMID\"" "$DM_LIST"
eq "19 deleting the same message again" 404 "$(code DELETE "$DM_PATH" -H "$AU")"
has "19 repeated delete body" '"message":"message not found"' "$(body DELETE "$DM_PATH" -H "$AU")"

# Refused deletes leave the message, its comments and every member read date untouched.
DM_OTHER=$(body POST /chats/$DMG/messages -H "$AU" -F 'text=sender only' | id)
DM_OTHER_PATH=/chats/$DMG/messages/$DM_OTHER
eq "19 comment on protected fixture" 201 "$(code PUT "$DM_OTHER_PATH/comments/me" -H "$CU" -H "$JS" -d '{"emoji":"❤️"}')"
REF19=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE id='$DM_OTHER')||'|'||(SELECT COUNT(*) FROM comments WHERE messageId='$DM_OTHER')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$DMG' ORDER BY userId));")
eq "19 member who is not sender" 403 "$(code DELETE "$DM_OTHER_PATH" -H "$BU")"
has "19 not-sender body" '"message":"not the sender of the message"' "$(body DELETE "$DM_OTHER_PATH" -H "$BU")"
eq "19 outsider" 403 "$(code DELETE "$DM_OTHER_PATH" -H "$DU")"
has "19 outsider body" '"message":"not a member of the chat"' "$(body DELETE "$DM_OTHER_PATH" -H "$DU")"
eq "19 message under the wrong chat" 404 "$(code DELETE /chats/$PD/messages/$DM_OTHER -H "$AU")"
has "19 wrong chat body" '"message":"chat not found"' "$(body DELETE /chats/$PD/messages/$DM_OTHER -H "$AU")"
eq "19 refused deletes write nothing" "$REF19" \
   "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE id='$DM_OTHER')||'|'||(SELECT COUNT(*) FROM comments WHERE messageId='$DM_OTHER')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$DMG' ORDER BY userId));")"
eq "19 unknown message" 404 "$(code DELETE /chats/$DMG/messages/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "19 unknown message body" '"message":"message not found"' \
    "$(body DELETE /chats/$DMG/messages/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "19 missing message wins over missing chat" '"message":"message not found"' \
    "$(body DELETE /chats/11111111-2222-4333-8444-555555555555/messages/22222222-2222-4222-8222-222222222222 -H "$AU")"

# The route works for private chats too, and a future read date is never moved backwards.
DM_PRIVATE=$(body POST /chats/$PD/messages -H "$AU" -F 'text=private deletion' | id)
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2099-01-01T00:00:00.000Z' WHERE chatId='$PD' AND userId='$A';"
eq "19 private message deleted" 204 "$(code DELETE /chats/$PD/messages/$DM_PRIVATE -H "$AU")"
eq "19 private message row is gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE id='$DM_PRIVATE';")"
eq "19 deletion never moves lastReadDate backwards" '2099-01-01T00:00:00.000Z' \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$PD' AND userId='$A';")"

# A forwarded photo is one shared file: deleting one message keeps it, deleting its last reference collects it.
DM_PHOTO_BODY=$(body POST /chats/$DMG/messages -H "$AU" -F "photoFile=@$PNG")
DM_PHOTO=$(printf '%s' "$DM_PHOTO_BODY" | id)
DM_PHOTO_ID=$(printf '%s' "$DM_PHOTO_BODY" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
DM_FORWARD=$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$DM_PHOTO\"}" | id)
eq "19 shared photo has two references" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE photoId='$DM_PHOTO_ID';")"
eq "19 delete original photo message" 204 "$(code DELETE /chats/$DMG/messages/$DM_PHOTO -H "$AU")"
eq "19 forwarded reference remains" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE photoId='$DM_PHOTO_ID';")"
eq "19 shared photo file remains" 200 "$(code GET /photos/$DM_PHOTO_ID -H "$AU")"
eq "19 delete final photo reference" 204 "$(code DELETE /chats/$PD/messages/$DM_FORWARD -H "$AU")"
if [ ! -f "db/photos/$DM_PHOTO_ID" ]; then ok; else no "19 unreferenced photo file is collected" "db/photos/$DM_PHOTO_ID still exists"; fi
eq "19 collected photo is not served" 404 "$(code GET /photos/$DM_PHOTO_ID -H "$AU")"

# Authentication wins before URL parsing; otherwise chatId is validated before messageId.
eq "19 bad chat id" 400 "$(code DELETE /chats/not-a-uuid/messages/$DM_OTHER -H "$AU")"
has "19 bad chat id body" '"message":"invalid chat id"' "$(body DELETE /chats/not-a-uuid/messages/$DM_OTHER -H "$AU")"
eq "19 uppercase chat id" 400 "$(code DELETE /chats/$(echo $DMG | tr a-f A-F)/messages/$DM_OTHER -H "$AU")"
eq "19 bad message id" 400 "$(code DELETE /chats/$DMG/messages/not-a-uuid -H "$AU")"
has "19 bad message id body" '"message":"invalid message id"' "$(body DELETE /chats/$DMG/messages/not-a-uuid -H "$AU")"
eq "19 uppercase message id" 400 "$(code DELETE /chats/$DMG/messages/$(echo $DM_OTHER | tr a-f A-F) -H "$AU")"
eq "19 no token wins over malformed ids" 401 "$(code DELETE /chats/nope/messages/nope)"
eq "19 GET on message item" 405 "$(code GET "$DM_OTHER_PATH" -H "$AU")"
eq "19 POST on message item" 405 "$(code POST "$DM_OTHER_PATH" -H "$AU")"
eq "19 PUT on message item" 405 "$(code PUT "$DM_OTHER_PATH" -H "$AU")"
eq "19 trailing slash" 404 "$(code DELETE "$DM_OTHER_PATH/" -H "$AU")"

echo "### 20. router level"
eq "20 unknown path"   404 "$(code GET /nope)"
eq "20 wrong method"   405 "$(code GET /session)"
eq "20 trailing slash" 404 "$(code GET /users/ -H "$AU")"
CORS=$(curl -s -D- -o /dev/null -X OPTIONS "$H/me/username" -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: PUT' -H 'Access-Control-Request-Headers: authorization,content-type')
has "20 CORS allow origin"  'Access-Control-Allow-Origin: *' "$CORS"
has "20 CORS allow method"  'Access-Control-Allow-Methods: PUT' "$CORS"
has "20 CORS allow headers" 'Access-Control-Allow-Headers: Authorization,Content-Type' "$CORS"
has "20 CORS max age"       'Access-Control-Max-Age: 1' "$CORS"

echo
echo "==================================================="
echo "PASS: $PASS   FAIL: $FAIL"
if [ $FAIL -gt 0 ]; then printf '%s\n' "${FAILED[@]}"; fi
echo "==================================================="
