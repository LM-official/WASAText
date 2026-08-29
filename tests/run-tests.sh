#!/usr/bin/env bash
# tests/endpoints.md, sections 0-16, for the endpoints registered in api-handler.go
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
JS='Content-Type: application/json'; MP='Content-Type: application/merge-patch+json'

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
eq "2 free username" 200 "$(code PATCH /me/username -H "$AU" -H "$MP" -d "{\"username\":\"alice_new\"}")"
has "2 body" "\"id\":\"$A\",\"username\":\"alice_new\",\"photo\":\"/photos/00000000-0000-4000-8000-000000000000\"" \
     "$(body PATCH /me/username -H "$AU" -H "$MP" -d "{\"username\":\"alice_new\"}")"
eq "2 idempotent"    200 "$(code PATCH /me/username -H "$AU" -H "$MP" -d "{\"username\":\"alice_new\"}")"
eq "2 taken"         400 "$(code PATCH /me/username -H "$AU" -H "$MP" -d "{\"username\":\"bob\"}")"
has "2 taken body"   '"message":"username already taken"' "$(body PATCH /me/username -H "$AU" -H "$MP" -d "{\"username\":\"bob\"}")"
eq "2 invalid"       400 "$(code PATCH /me/username -H "$AU" -H "$MP" -d '{"username":"a b"}')"
eq "2 plain json ct" 200 "$(code PATCH /me/username -H "$AU" -H "$JS" -d "{\"username\":\"alice_new\"}")"
eq "2 no token"      401 "$(code PATCH /me/username -H "$MP" -d '{"username":"x"}')"

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

echo "### 4. setMyPhoto"
BEFORE=$(ls db/photos | wc -l | tr -d ' ')
eq "4 png"          200 "$(code PATCH /me/photo -H "$AU" -F "photoFile=@$PNG")"
P1=$(body PATCH /me/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
P2=$(body PATCH /me/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
if [ "$P1" != "$P2" ]; then ok; else no "4 new upload new id" "$P1 == $P2"; fi
if [ ! -f "db/photos/$P1" ]; then ok; else no "4 replaced photo collected" "db/photos/$P1 still there"; fi
if [ -f "db/photos/00000000-0000-4000-8000-000000000000" ]; then ok; else no "4 default photo kept" "gone"; fi
N=$(ls db/photos | wc -l | tr -d ' ')
eq "4 wrong field"   400 "$(code PATCH /me/photo -H "$AU" -F "file=@$PNG")"
has "4 wrong field body" '"message":"missing photoFile field"' "$(body PATCH /me/photo -H "$AU" -F "file=@$PNG")"
eq "4 not an image"  400 "$(code PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
has "4 not an image body" '"message":"invalid photo"' "$(body PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
eq "4 empty file"    400 "$(code PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/empty.png")"
eq "4 not multipart" 400 "$(code PATCH /me/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
has "4 not multipart body" '"message":"invalid multipart body"' "$(body PATCH /me/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
eq "4 no leaked files" "$N" "$(ls db/photos | wc -l | tr -d ' ')"
{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=512k count=61 2>/dev/null; } > /tmp/big.png
eq "4 over MaxPhotoBytes" 400 "$(code PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/big.png")"
has "4 over MaxPhotoBytes body" '"message":"invalid photo"' "$(body PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/big.png")"
{ printf '\x89PNG\r\n\x1a\n'; dd if=/dev/zero bs=1m count=33 2>/dev/null; } > /tmp/huge.png
eq "4 over body limit" 400 "$(code PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/huge.png")"
has "4 over body limit body" '"message":"invalid multipart body"' "$(body PATCH /me/photo -H "$AU" -F "photoFile=@/tmp/huge.png")"
eq "4 still no leaks" "$N" "$(ls db/photos | wc -l | tr -d ' ')"
eq "4 no token" 401 "$(code PATCH /me/photo -F "photoFile=@$PNG")"

echo "### 5. getPhoto"
DEF=00000000-0000-4000-8000-000000000000
eq "5 existing"   200 "$(code GET /photos/$DEF -H "$AU")"
HDRS=$(curl -s -D- -o /dev/null "$H/photos/$DEF" -H "$AU")
has "5 content-type"  'image/png' "$HDRS"
has "5 nosniff"       'nosniff' "$HDRS"
has "5 cache-control" 'private, max-age=31536000, immutable' "$HDRS"
eq "5 not a uuid" 400 "$(code GET /photos/not-a-uuid -H "$AU")"
has "5 not a uuid body" '"message":"invalid photo id"' "$(body GET /photos/not-a-uuid -H "$AU")"
eq "5 dotted"     400 "$(code GET /photos/a.b -H "$AU")"
eq "5 traversal"  404 "$(code GET /photos/../../etc/passwd -H "$AU")"
eq "5 missing"    404 "$(code GET /photos/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "5 missing body" '"message":"photo not found"' "$(body GET /photos/11111111-2222-4333-8444-555555555555 -H "$AU")"
eq "5 no token"   401 "$(code GET /photos/$DEF)"
eq "5 range"      206 "$(code GET /photos/$DEF -H "$AU" -H 'Range: bytes=0-9')"

echo "### 6. createPrivateChat"
eq "6 first time"  201 "$(code POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
P=$(body POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
eq "6 again"       200 "$(code POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
eq "6 other side"  200 "$(code POST /private-chats -H "$BU" -H "$JS" -d "{\"id\":\"$A\"}")"
eq "6 same chat"   "$P" "$(body POST /private-chats -H "$BU" -H "$JS" -d "{\"id\":\"$A\"}" | id)"
eq "6 with self"   400 "$(code POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$A\"}")"
has "6 with self body" '"message":"cannot open a private chat with yourself"' "$(body POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$A\"}")"
eq "6 unknown user" 404 "$(code POST /private-chats -H "$AU" -H "$JS" -d '{"id":"11111111-2222-4333-8444-555555555555"}')"
has "6 unknown body" '"message":"the other user does not exist"' "$(body POST /private-chats -H "$AU" -H "$JS" -d '{"id":"11111111-2222-4333-8444-555555555555"}')"
eq "6 not a uuid"  400 "$(code POST /private-chats -H "$AU" -H "$JS" -d '{"id":"not-a-uuid"}')"
eq "6 empty body"  400 "$(code POST /private-chats -H "$AU" -H "$JS" -d '{}')"

echo "### 7. createGroup"
GBEFORE=$(ls db/photos | wc -l | tr -d ' ')
DOK="data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json"
DLEAVE="data={\"name\":\"Leave \",\"members\":[\"$B\",\"$C\"]};type=application/json"
DDROP="data={\"name\":\"Drop \",\"members\":[\"$B\"]};type=application/json"
DME="data={\"name\":\"X \",\"members\":[\"$A\"]};type=application/json"
DUNK="data={\"name\":\"X \",\"members\":[\"11111111-2222-4333-8444-555555555555\"]};type=application/json"
DNOPHOTO="data={\"name\":\"X \",\"members\":[\"$B\"]};type=application/json"
eq "7 valid" 201 "$(code POST /groups -H "$AU" -F "$DOK" -F "photoFile=@$PNG")"
G1=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
G2=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
if [ "$G1" != "$G2" ]; then ok; else no "7 same people many groups" "$G1 == $G2"; fi
eq "7 creator inside" 400 "$(code POST /groups -H "$AU" -F "$DME" -F "photoFile=@$PNG")"
has "7 creator body" '"message":"the creator is already a member of the group"' "$(body POST /groups -H "$AU" -F "$DME" -F "photoFile=@$PNG")"
eq "7 unknown member" 404 "$(code POST /groups -H "$AU" -F "$DUNK" -F "photoFile=@$PNG")"
has "7 unknown member body" '"message":"one or more of the members does not exist"' "$(body POST /groups -H "$AU" -F "$DUNK" -F "photoFile=@$PNG")"
eq "7 empty members" 400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[]};type=application/json" -F "photoFile=@$PNG")"
eq "7 duplicates"    400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[\"$B\",\"$B\"]};type=application/json" -F "photoFile=@$PNG")"
eq "7 empty name"    400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG")"
eq "7 no data part"  400 "$(code POST /groups -H "$AU" -F "photoFile=@$PNG")"
eq "7 no photo part" 400 "$(code POST /groups -H "$AU" -F "$DNOPHOTO")"
has "7 no photo body" '"message":"missing photoFile field"' "$(body POST /groups -H "$AU" -F "$DNOPHOTO")"
eq "7 bad photo"     400 "$(code POST /groups -H "$AU" -F "data={\"name\":\"X \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@/tmp/text.txt")"
eq "7 no token"      401 "$(code POST /groups -F "data={\"name\":\"X\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG")"

echo "### 8. setGroupName  (reply: GroupChat)"
G=$(body POST /groups -H "$AU" -F "data={\"name\":\"Study \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
GPHOTO=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
NB=$(body PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":"CS Study Group"}')
eq  "8 member renames"   200 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":"CS Study Group"}')"
eq  "8 exact body"       "{\"id\":\"$G\",\"chatType\":\"group\",\"name\":\"CS Study Group\",\"photo\":\"/photos/$GPHOTO\"}" "$NB"
hasnt "8 no snippet key" 'snippet' "$NB"
hasnt "8 no members key" 'members' "$NB"
eq  "8 other member"     200 "$(code PATCH /groups/$G/name -H "$BU" -H "$MP" -d '{"name":"CS Study Group"}')"
eq  "8 100 chars"        200 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d "{\"name\":\"$(printf 'z%.0s' $(seq 100))\"}")"
eq  "8 emoji name"       200 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":"👨‍👩‍👧‍👦"}')"
eq  "8 not a member"     403 "$(code PATCH /groups/$G/name -H "$CU" -H "$MP" -d '{"name":"Hijacked"}')"
has "8 403 body"         '"message":"not a member of the group"' "$(body PATCH /groups/$G/name -H "$CU" -H "$MP" -d '{"name":"Hijacked"}')"
eq  "8 unknown group"    404 "$(code PATCH /groups/11111111-2222-4333-8444-555555555555/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
has "8 404 body"         '"message":"group not found"' "$(body PATCH /groups/11111111-2222-4333-8444-555555555555/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
eq  "8 private chat id"  404 "$(code PATCH /groups/$P/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
eq  "8 not a uuid"       400 "$(code PATCH /groups/not-a-uuid/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
has "8 bad id body"      '"message":"invalid group id"' "$(body PATCH /groups/not-a-uuid/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
eq  "8 empty name"       400 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":""}')"
eq  "8 spaces name"      400 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":"   "}')"
eq  "8 101 chars"        400 "$(code PATCH /groups/$G/name -H "$AU" -H "$MP" -d "{\"name\":\"$(printf 'z%.0s' $(seq 101))\"}")"
eq  "8 GET on path"      405 "$(code GET /groups/$G/name -H "$AU")"
eq  "8 no token"         401 "$(code PATCH /groups/$G/name -H "$MP" -d '{"name":"x"}')"
body PATCH /groups/$G/name -H "$AU" -H "$MP" -d '{"name":"Kept Name"}' > /dev/null
body PATCH /groups/$G/name -H "$CU" -H "$MP" -d '{"name":"Hijacked"}' > /dev/null
eq  "8 403 writes nothing" "Kept Name" "$(sqlite3 db/wasatext.db "SELECT name FROM chats WHERE id='$G';")"
eq  "8 private chat untouched" "NULL|NULL" "$(sqlite3 db/wasatext.db "SELECT quote(name), quote(photoId) FROM chats WHERE id='$P';" | tr -d ' ')"

echo "### 9. setGroupPhoto  (reply: GroupChat)"
PB=$(body PATCH /groups/$G/photo -H "$AU" -F "photoFile=@$PNG")
eq  "9 member uploads"   200 "$(code PATCH /groups/$G/photo -H "$AU" -F "photoFile=@$PNG")"
has "9 body id"          "\"id\":\"$G\",\"chatType\":\"group\",\"name\":\"Kept Name\",\"photo\":\"/photos/" "$PB"
hasnt "9 no snippet key" 'snippet' "$PB"
hasnt "9 no members key" 'members' "$PB"
Q1=$(body PATCH /groups/$G/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
Q2=$(body PATCH /groups/$G/photo -H "$AU" -F "photoFile=@$PNG" | sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/')
if [ "$Q1" != "$Q2" ]; then ok; else no "9 new upload new id" "$Q1 == $Q2"; fi
if [ ! -f "db/photos/$Q1" ]; then ok; else no "9 replaced photo collected" "still there"; fi
eq  "9 other member"     200 "$(code PATCH /groups/$G/photo -H "$BU" -F "photoFile=@$PNG")"
eq  "9 wrong field"      400 "$(code PATCH /groups/$G/photo -H "$AU" -F "file=@$PNG")"
eq  "9 not an image"     400 "$(code PATCH /groups/$G/photo -H "$AU" -F "photoFile=@/tmp/text.txt")"
eq  "9 empty file"       400 "$(code PATCH /groups/$G/photo -H "$AU" -F "photoFile=@/tmp/empty.png")"
eq  "9 not multipart"    400 "$(code PATCH /groups/$G/photo -H "$AU" -H "$JS" -d '{"photoFile":"x"}')"
PN=$(ls db/photos | wc -l | tr -d ' ')
CUR=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
eq  "9 not a member"     403 "$(code PATCH /groups/$G/photo -H "$CU" -F "photoFile=@$PNG")"
eq  "9 unknown group"    404 "$(code PATCH /groups/11111111-2222-4333-8444-555555555555/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "9 private chat id"  404 "$(code PATCH /groups/$P/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "9 no leaked files"  "$PN" "$(ls db/photos | wc -l | tr -d ' ')"
eq  "9 photo untouched"  "$CUR" "$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")"
eq  "9 not a uuid"       400 "$(code PATCH /groups/not-a-uuid/photo -H "$AU" -F "photoFile=@$PNG")"
eq  "9 GET on path"      405 "$(code GET /groups/$G/photo -H "$AU")"
eq  "9 no token"         401 "$(code PATCH /groups/$G/photo -F "photoFile=@$PNG")"

echo "### 10. addToGroup  (reply: GroupWithMembers)"
AB=$(body POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")
eq  "10 adds"             200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")"
has "10 has members"      '"members":[' "$AB"
has "10 has chatType"     '"chatType":"group"' "$AB"
hasnt "10 no snippet key" 'snippet' "$AB"
eq  "10 member count"     3 "$(echo "$AB" | grep -o '"members":\[[^]]*\]' | grep -o '-' | wc -l | tr -d ' ' | awk '{print $1/4}')"
eq  "10 already inside"   200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")"
eq  "10 no duplicate row" 3 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G';")"
eq  "10 caller itself"    200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$A\"]}")"
eq  "10 empty list"       200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":[]}')"
eq  "10 absent field"     200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{}')"
eq  "10 null field"       200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":null}')"
eq  "10 not a member"     403 "$(code POST /groups/$G/members -H "$DU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "10 empty list 403"   403 "$(code POST /groups/$G/members -H "$DU" -H "$JS" -d '{"members":[]}')"
eq  "10 unknown group"    404 "$(code POST /groups/11111111-2222-4333-8444-555555555555/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "10 private chat id"  404 "$(code POST /groups/$P/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}")"
eq  "10 unknown member"   404 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["11111111-2222-4333-8444-555555555555"]}')"
has "10 unknown member body" '"message":"one or more of the members does not exist"' "$(body POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["11111111-2222-4333-8444-555555555555"]}')"
eq  "10 bad member id"    400 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d '{"members":["nope"]}')"
eq  "10 duplicates"       400 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\",\"$D\"]}")"
eq  "10 bad group id"     400 "$(code POST /groups/not-a-uuid/members -H "$AU" -H "$JS" -d '{"members":[]}')"
eq  "10 GET on path"      405 "$(code GET /groups/$G/members -H "$AU")"
eq  "10 no token"         401 "$(code POST /groups/$G/members -H "$JS" -d '{"members":[]}')"
# the cap: fill a fresh group past 100
GF=$(body POST /groups -H "$AU" -F "data={\"name\":\"Full \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
# the 99 fill users of the seed: addToGroup checks UsersExist before it counts, so they must be real
# doLogin is idempotent, so this reads their ids and writes nobody
FILL=""; for i in $(seq -w 1 99); do U=$(body POST /session -H "$JS" -d "{\"username\":\"fill$i\"}" | id); FILL="$FILL\"$U\","; done
eq "10 group full"    400 "$(code POST /groups/$GF/members -H "$AU" -H "$JS" -d "{\"members\":[${FILL%,}]}")"
has "10 group full body" '"message":"the group is full"' "$(body POST /groups/$GF/members -H "$AU" -H "$JS" -d "{\"members\":[${FILL%,}]}")"
eq "10 rollback whole" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GF';")"

echo "### 11. leaveGroup  (reply changed: 204, no body)"
# GL holds A, B and C: only B and C leave below, so the group stands for the rest of the section
GL=$(body POST /groups -H "$AU" -F "$DLEAVE" -F "photoFile=@$PNG" | id)
HDR=$(curl -s -D- -o /dev/null -X DELETE "$H/groups/$GL/members/me" -H "$BU")
has "11 status is 204"    '204' "$HDR"
hasnt "11 no content-type" 'Content-Type' "$HDR"
LB=$(body DELETE /groups/$GL/members/me -H "$CU")
eq  "11 empty body"       "" "$LB"
eq  "11 caller removed"   0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GL' AND userId='$C';")"
eq  "11 A still inside"   1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GL';")"
eq  "11 leave again"      403 "$(code DELETE /groups/$GL/members/me -H "$BU")"
eq  "11 never a member"   403 "$(code DELETE /groups/$GL/members/me -H "$DU")"
eq  "11 unknown group"    404 "$(code DELETE /groups/11111111-2222-4333-8444-555555555555/members/me -H "$AU")"
eq  "11 private chat id"  404 "$(code DELETE /groups/$P/members/me -H "$AU")"
eq  "11 bad id"           400 "$(code DELETE /groups/not-a-uuid/members/me -H "$AU")"
eq  "11 GET on path"      405 "$(code GET /groups/$GL/members/me -H "$AU")"
eq  "11 no token"         401 "$(code DELETE /groups/$GL/members/me)"
# the last member out drops the group and its photo
GD=$(body POST /groups -H "$AU" -F "$DDROP" -F "photoFile=@$PNG" | id)
GDP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$GD';")
eq  "11 first leaver 204" 204 "$(code DELETE /groups/$GD/members/me -H "$AU")"
eq  "11 B still inside" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GD';")"
if [ -f "db/photos/$GDP" ]; then ok; else no "11 photo kept while group stands" "gone"; fi
eq  "11 last leaver 204"  204 "$(code DELETE /groups/$GD/members/me -H "$BU")"
eq  "11 chat row gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats WHERE id='$GD';")"
eq  "11 memberships gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$GD';")"
if [ ! -f "db/photos/$GDP" ]; then ok; else no "11 photo collected with group" "still there"; fi
eq  "11 gone is 404"   404 "$(code DELETE /groups/$GD/members/me -H "$AU")"
eq  "11 no stranded chat" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats c WHERE NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId=c.id);")"

echo "### 12. getMyConversations  (reply: GroupSummary | PrivateChatSummary, with the snippet)"
GS=$(body POST /groups -H "$AU" -F "data={\"name\":\"Silent \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
CL=$(body GET /me/chats -H "$AU")
eq  "12 list"          200 "$(code GET /me/chats -H "$AU")"
has "12 has chats"     '"chats":[' "$CL"
has "12 has chatType"  '"chatType":' "$CL"
has "12 silent group has no snippet" "\"id\":\"$GS\",\"chatType\":\"group\",\"name\":\"Silent \"" "$CL"
NEW=$(body POST /session -H "$JS" -d "{\"username\":\"lonely\"}" | id)
eq  "12 no chats"      404 "$(code GET /me/chats -H "Authorization: Bearer $NEW")"
has "12 404 body"      '"message":"no conversations found"' "$(body GET /me/chats -H "Authorization: Bearer $NEW")"
eq  "12 POST on path"  405 "$(code POST /me/chats -H "$AU")"
eq  "12 trailing slash" 404 "$(code GET /me/chats/ -H "$AU")"
eq  "12 no token"      401 "$(code GET /me/chats)"
# the borrow is live, and a group is never multiplied
has "12 private chat named after the other" "alice_new" "$(body GET /me/chats -H "$BU")"
body PATCH /me/username -H "$BU" -H "$MP" -d "{\"username\":\"bob_renamed\"}" > /dev/null
has "12 rename reaches the other list" "bob_renamed" "$(body GET /me/chats -H "$AU")"
eq  "12 group appears once" 1 "$(body GET /me/chats -H "$AU" | grep -o "\"id\":\"$G\"" | wc -l | tr -d ' ')"
# the snippet: the messages are the ones sendMessage writes (§14), not rows put in by hand
LONGTEXT=$(printf 'abcdefghij%.0s' $(seq 1 12))
body POST /chats/$G/messages -H "$BU" -F "text=$LONGTEXT" > /dev/null
body POST /chats/$P/messages -H "$BU" -F "photoFile=@$PNG" > /dev/null
SN=$(body GET /me/chats -H "$AU")
CUT50=$(printf 'abcdefghij%.0s' $(seq 1 5))
has "12 text snippet truncated to 50" "\"text\":\"$CUT50\"" "$SN"
has "12 photo snippet emoji" '"emoji":"📷"' "$SN"
has "12 snippet state received" '"state":"received"' "$SN"
# the state moves as the members see the chat, and reads the same for everybody
# every read answers with the states taken before it marks the chat, so each call shows what was
# true when it was asked: B is caught up by having sent, and A and C by opening
has "12 received while C has not opened" '"state":"received"' "$(body GET /chats/$G -H "$AU")"
body GET /chats/$G -H "$CU" > /dev/null
has "12 read once every member has seen it" '"state":"read"' "$(body GET /chats/$G -H "$AU")"
has "12 the snippet carries that state too" '"state":"read"' "$(body GET /me/chats -H "$AU")"
# a message holding an empty text and no photo: sendMessage answers 400 to it, so the row can only be
# written here, and it is what keeps the guard that drops a snippet with nothing to show covered
GE=$(body POST /groups -H "$AU" -F "data={\"name\":\"Empty\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "INSERT INTO messages (id,chatId,userId,text,photoId,date) VALUES
 ('55555555-5555-4555-8555-555555555555','$GE','$B','',NULL,'2099-01-01T00:00:00.000Z');"
EM=$(body GET /me/chats -H "$AU")
has   "12 a chat whose last message shows nothing is still listed" "\"id\":\"$GE\"" "$EM"
hasnt "12 and carries an empty content instead of a snippet" '"content":{}' "$EM"
# ordering: newest first, silent chat last
# written by hand and not sent so the two messages deterministically share the same millisecond.
# Dated ahead of every sent message so they stay the newest of P
sqlite3 db/wasatext.db "INSERT INTO messages (id,chatId,userId,text,photoId,date) VALUES
 ('33333333-3333-4333-8333-333333333333','$P','$B','FIRST in that millisecond',NULL,'2099-01-01T00:00:00.000Z'),
 ('44444444-4444-4444-8444-444444444444','$P','$B','SECOND in that millisecond',NULL,'2099-01-01T00:00:00.000Z');"
ORD=$(body GET /me/chats -H "$AU")
has "12 rowid breaks the tie" 'SECOND in that millisecond' "$ORD"
POS() { echo "$ORD" | grep -o '"id":"[0-9a-f-]*"' | grep -n "$1" | head -1 | cut -d: -f1; }
PG=$(POS "$G"); PP=$(POS "$P"); PS=$(POS "$GS")
if [ -n "$PG" ] && [ -n "$PP" ] && [ -n "$PS" ] && [ "$PS" -gt "$PG" ] && [ "$PS" -gt "$PP" ]; then ok
else no "12 message-less chat sorts after the ones with messages" "P=$PP G=$PG silent=$PS"; fi

echo "### 13. getConversation  (reply: GroupDetail | PrivateChatDetail, with the messages)"
GC=$(body GET /chats/$G -H "$AU")
eq  "13 group 200"        200 "$(code GET /chats/$G -H "$AU")"
has "13 group id"         "\"id\":\"$G\"" "$GC"
has "13 group chatType"   '"chatType":"group"' "$GC"
eq  "13 group answers with the name it owns" "$(sqlite3 db/wasatext.db "SELECT name FROM chats WHERE id='$G';")" "$(echo "$GC" | sed 's/.*"chatType":"group","name":"\([^"]*\)".*/\1/')"
has "13 photo is a URL"   '"photo":"/photos/' "$GC"
has "13 has members"      '"members":[' "$GC"
has "13 has messages"     '"messages":[' "$GC"
hasnt "13 no snippet in the opened chat" '"snippet"' "$GC"
eq  "13 group holds three members" 3 "$(echo "$GC" | grep -o '"members":\[[^]]*\]' | grep -o '[0-9a-f-]\{36\}' | wc -l | tr -d ' ')"
# a private chat reads the name of the other member, so one row answers each of the two differently
PC=$(body GET /chats/$P -H "$AU")
eq  "13 private 200"      200 "$(code GET /chats/$P -H "$AU")"
has "13 private chatType" '"chatType":"private"' "$PC"
has "13 private named after the other" "\"name\":\"bob_renamed\"" "$PC"
has "13 private name flips" "\"name\":\"alice_new\"" "$(body GET /chats/$P -H "$BU")"
eq  "13 private holds two members" 2 "$(echo "$PC" | grep -o '"members":\[[^]]*\]' | grep -o '[0-9a-f-]\{36\}' | wc -l | tr -d ' ')"
# newest first, rowid breaking the tie inside the same millisecond, exactly as the snippet of section 12
eq  "13 messages newest first" "SECOND in that millisecond FIRST in that millisecond " "$(echo "$PC" | grep -o 'SECOND in that millisecond\|FIRST in that millisecond' | tr '\n' ' ')"
has "13 text only message"  '"content":{"text":"SECOND in that millisecond"}' "$PC"
has "13 photo only message" '"content":{"photo":"/photos/' "$PC"
hasnt "13 no bare prefix on a message without a photo" '"photo":"/photos/"' "$PC"
has "13 the opened chat carries the whole text, where the snippet cuts it" "\"text\":\"$LONGTEXT\"" "$GC"
# the comments, written by hand: commentMessage does not exist yet
CMIDS="'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'"
sqlite3 db/wasatext.db "DELETE FROM comments WHERE id IN ($CMIDS);"
sqlite3 db/wasatext.db "INSERT INTO comments (id,messageId,userId,emoji) VALUES
 ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','44444444-4444-4444-8444-444444444444','$A','👍'),
 ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','33333333-3333-4333-8333-333333333333','$A','😂');"
PC2=$(body GET /chats/$P -H "$AU")
has "13 a comment lands on its own message" '"content":{"text":"SECOND in that millisecond"},"comments":[{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"' "$PC2"
has "13 no comment is an empty list and never null" '"comments":[]' "$PC2"
# the state is the one of section 12: computed from lastReadDate, the same value for every member
has "13 read on the group"           '"state":"read"' "$GC"
has "13 received on the private chat" '"state":"received"' "$PC"
has "13 a chat with no message answers an empty list" '"messages":[]' "$(body GET /chats/$GS -H "$AU")"
# the one id that answers here and not under /groups/: both kinds are read through /chats/
eq  "13 private chat id under /groups/" 404 "$(code PATCH /groups/$P/name -H "$AU" -H "$MP" -d '{"name":"x"}')"
eq  "13 private chat id under /chats/"  200 "$(code GET /chats/$P -H "$AU")"
# one page: a chat answers at most schemas.ChatMessagesPageSize messages, whatever it holds
GP=$(body POST /groups -H "$AU" -F "data={\"name\":\"Page \",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "DELETE FROM messages WHERE id LIKE 'page____-0000-4000-8000-000000000000';"
sqlite3 db/wasatext.db "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 501)
 INSERT INTO messages (id,chatId,userId,text,photoId,date)
 SELECT printf('page%04d-0000-4000-8000-000000000000', i), '$GP', '$A', 'm'||i, NULL,
        strftime('%Y-%m-%dT%H:%M:%fZ', datetime('2026-08-24T00:00:00Z', '+'||i||' seconds')) FROM n;"
eq  "13 501 messages answer one page of 500" 500 "$(body GET /chats/$GP -H "$AU" | grep -o '"state":"' | wc -l | tr -d ' ')"
eq  "13 not a member"     403 "$(code GET /chats/$G -H "$DU")"
has "13 403 body"         '"message":"not a member of the chat"' "$(body GET /chats/$G -H "$DU")"
eq  "13 unknown chat"     404 "$(code GET /chats/11111111-2222-4333-8444-555555555555 -H "$AU")"
has "13 404 body"         '"message":"chat not found"' "$(body GET /chats/11111111-2222-4333-8444-555555555555 -H "$AU")"
eq  "13 bad id"           400 "$(code GET /chats/not-a-uuid -H "$AU")"
has "13 400 body"         '"message":"invalid chat id"' "$(body GET /chats/not-a-uuid -H "$AU")"
eq  "13 uppercase uuid"   400 "$(code GET /chats/$(echo $G | tr a-f A-F) -H "$AU")"
eq  "13 POST on path"     405 "$(code POST /chats/$G -H "$AU")"
eq  "13 trailing slash"   404 "$(code GET /chats/$G/ -H "$AU")"
eq  "13 no token"         401 "$(code GET /chats/$G)"
# no row is added or removed: the one thing this read writes is the lastReadDate of its caller
CNT() { sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM chats)||'/'||(SELECT COUNT(*) FROM chat_members)||'/'||(SELECT COUNT(*) FROM messages)||'/'||(SELECT COUNT(*) FROM comments);"; }
LRD() { sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$P' AND userId='$A';"; }
OTH() { sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$P' AND userId='$B';"; }
BEF13=$(CNT); LR13=$(LRD); OT13=$(OTH)
body GET /chats/$G -H "$AU" > /dev/null; body GET /chats/$P -H "$AU" > /dev/null
eq  "13 no row is written" "$BEF13" "$(CNT)"
# opening a chat is what marks it read, and it marks it for the caller alone
if [ "$(LRD)" \> "$LR13" ]; then ok; else no "13 opening a chat marks it read for the caller" "$LR13 -> $(LRD)"; fi
eq  "13 and leaves every other member alone" "$OT13" "$(OTH)"
# a caller that is refused writes nothing at all
LRD_D() { sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G' AND userId='$D';"; }
body GET /chats/$G -H "$DU" > /dev/null
eq  "13 a 403 marks nothing" 0 "$(LRD_D)"

echo "### 14. sendMessage  (reply: Message, 201)"
SM=$(body POST /chats/$G/messages -H "$AU" -F "text=hello everybody")
eq  "14 text only"        201 "$(code POST /chats/$G/messages -H "$AU" -F 'text=another one')"
has "14 reply is a Message" '"content":{"text":"hello everybody"}' "$SM"
has "14 born with no reaction" '"comments":[]' "$SM"
has "14 carries its own date" '"date":"20' "$SM"
eq  "14 photo only"       201 "$(code POST /chats/$G/messages -H "$AU" -F "photoFile=@$PNG")"
BOTH=$(body POST /chats/$G/messages -H "$AU" -F "text=look at this" -F "photoFile=@$PNG")
has "14 text and photo together" '"text":"look at this"' "$BOTH"
has "14 the photo is a URL and never an id" '"photo":"/photos/' "$BOTH"
eq  "14 a private chat takes one too" 201 "$(code POST /chats/$P/messages -H "$AU" -F 'text=hi bob')"
# what a message may not be
eq  "14 neither text nor photo" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'x=1')"
has "14 400 body"        '"message":"empty message content: text or photo is required"' "$(body POST /chats/$G/messages -H "$AU" -F 'x=1')"
eq  "14 spaces alone"    400 "$(code POST /chats/$G/messages -H "$AU" -F 'text=   ')"
eq  "14 empty text"      400 "$(code POST /chats/$G/messages -H "$AU" -F 'text=')"
T5000=$(printf 'z%.0s' $(seq 5000)); T5001=$(printf 'z%.0s' $(seq 5001))
eq  "14 5000 chars"      201 "$(code POST /chats/$G/messages -H "$AU" -F "text=$T5000")"
eq  "14 5001 chars"      400 "$(code POST /chats/$G/messages -H "$AU" -F "text=$T5001")"
has "14 too long body"   '"message":"invalid message text"' "$(body POST /chats/$G/messages -H "$AU" -F "text=$T5001")"
# one grapheme cluster is one char, as everywhere else
eq  "14 an emoji counts one" 201 "$(code POST /chats/$G/messages -H "$AU" -F "text=$(printf '\U0001f468‍\U0001f469‍\U0001f467‍\U0001f466%.0s' $(seq 1 100))")"
# the photo, refused exactly as section 4 refuses it
eq  "14 not an image"    400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt')"
has "14 not an image body" '"message":"invalid photo"' "$(body POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt')"
eq  "14 empty photo"     400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/empty.png')"
eq  "14 over MaxPhotoBytes" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/big.png')"
eq  "14 over the body limit" 400 "$(code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/huge.png')"
eq  "14 not multipart"   400 "$(code POST /chats/$G/messages -H "$AU" -H "$JS" -d '{"text":"x"}')"
# a file part under another name is no photo at all, so the text alone decides
eq  "14 wrong field with a text"    201 "$(code POST /chats/$G/messages -H "$AU" -F 'text=ok' -F "file=@$PNG")"
eq  "14 wrong field without a text" 400 "$(code POST /chats/$G/messages -H "$AU" -F "file=@$PNG")"
# who may write, and where
eq  "14 not a member"    403 "$(code POST /chats/$G/messages -H "$DU" -F 'text=x')"
has "14 403 body"        '"message":"not a member of the chat"' "$(body POST /chats/$G/messages -H "$DU" -F 'text=x')"
eq  "14 unknown chat"    404 "$(code POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F 'text=x')"
has "14 404 body"        '"message":"chat not found"' "$(body POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F 'text=x')"
eq  "14 bad chat id"     400 "$(code POST /chats/not-a-uuid/messages -H "$AU" -F 'text=x')"
has "14 bad id body"     '"message":"invalid chat id"' "$(body POST /chats/not-a-uuid/messages -H "$AU" -F 'text=x')"
eq  "14 uppercase uuid"  400 "$(code POST /chats/$(echo $G | tr a-f A-F)/messages -H "$AU" -F 'text=x')"
eq  "14 GET on the path" 405 "$(code GET /chats/$G/messages -H "$AU")"
eq  "14 trailing slash"  404 "$(code POST /chats/$G/messages/ -H "$AU" -F 'text=x')"
eq  "14 no token"        401 "$(code POST /chats/$G/messages -F 'text=x')"
# nothing is left behind by a refusal, whatever the reason and however late it fires
PB=$(ls db/photos | wc -l | tr -d ' ')
code POST /chats/$G/messages -H "$DU" -F "photoFile=@$PNG" > /dev/null
code POST /chats/11111111-2222-4333-8444-555555555555/messages -H "$AU" -F "photoFile=@$PNG" > /dev/null
code POST /chats/$G/messages -H "$AU" -F 'photoFile=@/tmp/text.txt' > /dev/null
eq  "14 a refused send leaks no photo" "$PB" "$(ls db/photos | wc -l | tr -d ' ')"
eq  "14 every stored photo is on disk" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE photoId IS NOT NULL AND photoId NOT IN ($(ls db/photos | sed "s/.*/'&'/" | paste -sd, -));")"
# the sender is caught up to its own message, which is what keeps it from answering for it
GX=$(body POST /groups -H "$AU" -F "data={\"name\":\"Stamp\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
MD=$(body POST /chats/$GX/messages -H "$AU" -F 'text=stamp' | sed 's/.*"date":"\([^"]*\)".*/\1/')
# Plain time.Time keeps the same instant; JSON may trim trailing zeroes from the fixed-width DB value
eq  "14 the reply date is the stored instant" 1 \
    "$(sqlite3 db/wasatext.db "SELECT julianday('$MD') = julianday((SELECT date FROM messages WHERE chatId='$GX' ORDER BY rowid DESC LIMIT 1));")"
eq  "14 and it is a valid date" 1 "$(sqlite3 db/wasatext.db "SELECT julianday('$MD') IS NOT NULL;")"
# read before the chat is opened below: opening it moves the caller lastReadDate forward
eq  "14 the sender lastReadDate is the date of its message" \
    "$(sqlite3 db/wasatext.db "SELECT date FROM messages WHERE chatId='$GX' ORDER BY rowid DESC LIMIT 1;")" \
    "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$GX' AND userId='$A';")"
eq  "14 the same date comes back from getConversation" "$MD" \
    "$(body GET /chats/$GX -H "$AU" | sed 's/.*"date":"\([^"]*\)".*/\1/')"
# the state of a new message is computed and not assumed
has "14 received while somebody else is behind" '"state":"received"' "$(body POST /chats/$GX/messages -H "$AU" -F 'text=still received')"
# e.g.1 a chat the sender is alone in: there is nobody to be behind, so the message is born read
G1=$(body POST /groups -H "$AU" -F "data={\"name\":\"Solo\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
code DELETE /groups/$G1/members/me -H "$BU" > /dev/null
has "14 e.g.1 alone in the chat, born read" '"state":"read"' "$(body POST /chats/$G1/messages -H "$AU" -F 'text=alone')"
# e.g.2 the member that never opened it leaves, and the message it was holding becomes read
G2=$(body POST /groups -H "$AU" -F "data={\"name\":\"Three\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$G2/messages -H "$AU" -F 'text=hi all' > /dev/null
body GET /chats/$G2 -H "$BU" > /dev/null
has "14 e.g.2 received while C has not opened" '"state":"received"' "$(body GET /chats/$G2 -H "$AU")"
code DELETE /groups/$G2/members/me -H "$CU" > /dev/null
has "14 e.g.2 read once the one behind it leaves" '"state":"read"' "$(body GET /chats/$G2 -H "$AU")"
# e.g.3 read is a black hole: a member joining answers for nothing sent before it
G3=$(body POST /groups -H "$AU" -F "data={\"name\":\"Hole\",\"members\":[\"$B\",\"$C\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$G3/messages -H "$AU" -F 'text=read me' > /dev/null
body GET /chats/$G3 -H "$BU" > /dev/null; body GET /chats/$G3 -H "$CU" > /dev/null
has "14 e.g.3 read once every member has seen it" '"state":"read"' "$(body GET /chats/$G3 -H "$AU")"
code POST /groups/$G3/members -H "$AU" -H "$JS" -d "{\"members\":[\"$D\"]}" > /dev/null
has "14 e.g.3 and still read after somebody joins" '"state":"read"' "$(body GET /chats/$G3 -H "$AU")"
hasnt "14 e.g.3 a join never turns a message back" '"state":"received"' "$(body GET /chats/$G3 -H "$AU")"
# a chat holds at most schemas.ChatMaxMessages: seeded by hand, 9999 calls would be absurd
GF=$(body POST /groups -H "$AU" -F "data={\"name\":\"Full\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
sqlite3 db/wasatext.db "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 9999)
 INSERT INTO messages (id,chatId,userId,text,photoId,date)
 SELECT printf('%08d-0000-4000-8000-000000000000', i), '$GF', '$A', 'm'||i, NULL,
        strftime('%Y-%m-%dT%H:%M:%fZ', datetime('2020-01-01T00:00:00Z', '+'||i||' seconds')) FROM n;"
eq  "14 the 10000th message fits"  201 "$(code POST /chats/$GF/messages -H "$AU" -F 'text=the last one')"
eq  "14 the 10001st does not"      400 "$(code POST /chats/$GF/messages -H "$AU" -F 'text=one too many')"
has "14 full body"       '"message":"the chat is full"' "$(body POST /chats/$GF/messages -H "$AU" -F 'text=one too many')"
eq  "14 and the refused one wrote no row" 10000 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$GF';")"
# a message reaches the two reads it belongs to
GN=$(body POST /groups -H "$AU" -F "data={\"name\":\"Newest\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
GO=$(body POST /groups -H "$AU" -F "data={\"name\":\"Older\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
body POST /chats/$GO/messages -H "$AU" -F 'text=the older thing' > /dev/null
body POST /chats/$GN/messages -H "$AU" -F 'text=the newest thing' > /dev/null
has "14 it reaches the opened chat"  'the newest thing' "$(body GET /chats/$GN -H "$AU")"
has "14 and the snippet of the list" 'the newest thing' "$(body GET /me/chats -H "$AU")"
# sending moves a chat up the list, which is the whole sort key of §12
# the two are compared against each other and not against the head of the list, which §12 pins ahead of both
RANK() { body GET /me/chats -H "$AU" | grep -o '"id":"[0-9a-f-]*"' | grep -n "$1" | head -1 | cut -d: -f1; }
if [ "$(RANK $GN)" -lt "$(RANK $GO)" ]; then ok; else no "14 the chat written last comes first" "GN=$(RANK $GN) GO=$(RANK $GO)"; fi
body POST /chats/$GO/messages -H "$AU" -F 'text=and now this one' > /dev/null
if [ "$(RANK $GO)" -lt "$(RANK $GN)" ]; then ok; else no "14 and writing again moves it back up" "GO=$(RANK $GO) GN=$(RANK $GN)"; fi

echo "### 15. forwardMessage  (reply: Message, 201)"
# A member of both chats, B source-only, C destination-only, D neither — same fixture shape as §15 of endpoints.md
PS=$(body POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
PD=$(body POST /private-chats -H "$AU" -H "$JS" -d "{\"id\":\"$C\"}" | id)
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
FW_CODE=$(curl -s -o "$FW_FILE" -w '%{http_code}' -X POST "$H/chats/$PD/messages/forwards" \
  -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")
FW=$(tr -d '\n' < "$FW_FILE")
eq  "15 text-only source"        201 "$FW_CODE"
has "15 text copied"             '"content":{"text":"forward this text"}' "$FW"
hasnt "15 no photo key on text-only" '"photo"' "$FW"
has "15 born with no reaction"   '"comments":[]' "$FW"
has "15 carries its own date"    '"date":"20' "$FW"
has "15 user is the caller"      "\"user\":\"$A\"" "$FW"
hasnt "15 user is not the source sender" "\"user\":\"$B\"" "$FW"
if [ "$(echo "$FW" | id)" != "$MS" ]; then ok; else no "15 new id, not the source id" "$MS"; fi
FWID=$(echo "$FW" | id)
FWDATE=$(echo "$FW" | sed 's/.*"date":"\([^"]*\)".*/\1/')
has "15 received while C is behind" '"state":"received"' "$FW"
eq "15 reply date is the stored instant" 1 \
   "$(sqlite3 db/wasatext.db "SELECT julianday('$FWDATE') = julianday((SELECT date FROM messages WHERE id='$FWID'));")"
eq "15 sender caught up to the exact copy date" \
   "$(sqlite3 db/wasatext.db "SELECT date FROM messages WHERE id='$FWID';")" \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate FROM chat_members WHERE chatId='$PD' AND userId='$A';")"

FWP=$(body POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MPH\"}")
has   "15 exact photo URL copied"     "\"photo\":\"$MPHOTO\"" "$FWP"
hasnt "15 no text key on photo-only" '"text"' "$FWP"

FWB=$(body POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MB\"}")
has "15 both fields copied, text"  '"text":"both fields"' "$FWB"
has "15 both fields copied, exact photo" "\"photo\":\"$MBPHOTO\"" "$FWB"
eq  "15 forwarding creates no photo file" "$PHOTOS15" "$(ls db/photos | wc -l | tr -d ' ')"

# comments belong to the source row and are not copied
sqlite3 db/wasatext.db "INSERT INTO comments (id, messageId, userId, emoji) VALUES ('99999999-0000-4000-8000-000000000000', '$MS', '$B', '👍');"
FWC=$(body POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")
FWCID=$(echo "$FWC" | id)
has "15 source comments are not copied" '"comments":[]' "$FWC"
eq  "15 copied row owns no comment" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$FWCID';")"
eq  "15 source keeps its comment"   1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$MS';")"
eq  "15 source message stays untouched" "$SRC_ROW" \
    "$(sqlite3 db/wasatext.db "SELECT userId||'|'||text||'|'||quote(photoId)||'|'||date FROM messages WHERE id='$MS';")"

# forwarding into the source chat itself is allowed, and any chat kind is a valid destination
eq "15 same-chat forward" 201 "$(code POST /chats/$PS/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq "15 group destination"  201 "$(code POST /chats/$G/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
# a destination with no other member has nobody to hold the copy at received
GFS=$(body POST /groups -H "$AU" -F "data={\"name\":\"Forward Solo\",\"members\":[\"$B\"]};type=application/json" -F "photoFile=@$PNG" | id)
code DELETE /groups/$GFS/members/me -H "$BU" > /dev/null
has "15 alone in destination, born read" '"state":"read"' \
    "$(body POST /chats/$GFS/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"

# who may forward, and between which chats
eq  "15 belongs to source and destination" 201 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
REFUSED_BEFORE=$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE chatId='$PD')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$PD' ORDER BY userId));")
eq  "15 source-only member"      403 "$(code POST /chats/$PD/messages/forwards -H "$BU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "15 destination-only member" 403 "$(code POST /chats/$PD/messages/forwards -H "$CU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "15 belongs to neither"      403 "$(code POST /chats/$PD/messages/forwards -H "$DU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 403 body" '"message":"not a member of the chat"' "$(body POST /chats/$PD/messages/forwards -H "$DU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "15 every 403 writes nothing" "$REFUSED_BEFORE" \
    "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE chatId='$PD')||'|'||(SELECT group_concat(userId||'='||lastReadDate, ';') FROM (SELECT userId,lastReadDate FROM chat_members WHERE chatId='$PD' ORDER BY userId));")"

# not found: message before chat
NOTFOUND_BEFORE=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")
eq  "15 unknown message"  404 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{"messageId":"11111111-2222-4333-8444-555555555555"}')"
has "15 message 404 body" '"message":"message not found"' "$(body POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{"messageId":"11111111-2222-4333-8444-555555555555"}')"
eq  "15 unknown chat"     404 "$(code POST /chats/11111111-2222-4333-8444-555555555555/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 chat 404 body"    '"message":"chat not found"' "$(body POST /chats/11111111-2222-4333-8444-555555555555/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 missing source wins over missing destination" '"message":"message not found"' \
    "$(body POST /chats/11111111-2222-4333-8444-555555555555/messages/forwards -H "$AU" -H "$JS" -d '{"messageId":"22222222-2222-4222-8222-222222222222"}')"
eq  "15 every 404 writes no message" "$NOTFOUND_BEFORE" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages;")"

# the body and the ids
eq  "15 empty messageId"    400 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{"messageId":""}')"
eq  "15 missing messageId"  400 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{}')"
has "15 400 body"           '"message":"invalid request body"' "$(body POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{}')"
eq  "15 malformed body"     400 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d '{')"
eq  "15 uppercase messageId" 400 "$(code POST /chats/$PD/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$(echo $MS | tr a-f A-F)\"}")"
eq  "15 bad chat id"        400 "$(code POST /chats/not-a-uuid/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 bad chat id body"   '"message":"invalid chat id"' "$(body POST /chats/not-a-uuid/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "15 uppercase chat id"  400 "$(code POST /chats/$(echo $PD | tr a-f A-F)/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 bad destination id wins over malformed body" '"message":"invalid chat id"' \
    "$(body POST /chats/not-a-uuid/messages/forwards -H "$AU" -H "$JS" -d '{')"
EXTRA_CODE=$(curl -s -o "$FW_FILE" -w '%{http_code}' -X POST "$H/chats/$PD/messages/forwards" \
  -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\",\"admin\":true}")
EXTRA=$(tr -d '\n' < "$FW_FILE")
eq  "15 unknown JSON field is ignored" 201 "$EXTRA_CODE"
has "15 extra field still returns the copy" '"text":"forward this text"' "$EXTRA"

# the destination cap is checked here too — GF is already full from §14
eq  "15 destination full"  400 "$(code POST /chats/$GF/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
has "15 full body"         '"message":"the chat is full"' "$(body POST /chats/$GF/messages/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq  "15 and the refused one wrote no row" 10000 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$GF';")"

# router-level answers on this route
eq "15 GET on the path" 405 "$(code GET /chats/$PD/messages/forwards -H "$AU")"
eq "15 trailing slash"  404 "$(code POST /chats/$PD/messages/forwards/ -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")"
eq "15 no token wins over malformed body" 401 "$(code POST /chats/$PD/messages/forwards -H "$JS" -d '{')"

echo "### 16. router level"
eq "16 unknown path"   404 "$(code GET /nope)"
eq "16 wrong method"   405 "$(code GET /session)"
eq "16 trailing slash" 404 "$(code GET /users/ -H "$AU")"
CORS=$(curl -s -D- -o /dev/null -X OPTIONS "$H/me/username" -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: PATCH' -H 'Access-Control-Request-Headers: authorization,content-type')
has "16 CORS allow origin"  'Access-Control-Allow-Origin: *' "$CORS"
has "16 CORS allow method"  'Access-Control-Allow-Methods: PATCH' "$CORS"
has "16 CORS allow headers" 'Access-Control-Allow-Headers: Authorization,Content-Type' "$CORS"
has "16 CORS max age"       'Access-Control-Max-Age: 1' "$CORS"

echo
echo "==================================================="
echo "PASS: $PASS   FAIL: $FAIL"
if [ $FAIL -gt 0 ]; then printf '%s\n' "${FAILED[@]}"; fi
echo "==================================================="
