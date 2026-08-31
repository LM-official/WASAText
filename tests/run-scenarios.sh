#!/usr/bin/env bash
# tests/endpoints.md section 21, S1-S20 — the scenarios: a sequence, not a single call
cd /Users/lorenzo/WASAText
H=localhost:3000
R=$(date +%s)
PASS=0; FAIL=0; FAILED=()
id()  { sed 's/.*"id":"\([^"]*\)".*/\1/'; }
ok()  { PASS=$((PASS+1)); }
no()  { FAIL=$((FAIL+1)); FAILED+=("$1 -- got: $2"); }
eq()  { if [ "$2" = "$3" ]; then ok; else no "$1 (want '$2')" "$3"; fi; }
ne()  { if [ "$2" != "$3" ]; then ok; else no "$1 (want different)" "$3"; fi; }
has() { case "$3" in *"$2"*) ok;; *) no "$1 (want '$2')" "$3";; esac; }
hasnt(){ case "$3" in *"$2"*) no "$1 (unwanted '$2')" "$3";; *) ok;; esac; }
code(){ local m=$1 p=$2; shift 2; curl -s -o /dev/null -w '%{http_code}' -X "$m" "$H$p" "$@"; }
body(){ local m=$1 p=$2; shift 2; curl -s -X "$m" "$H$p" "$@"; }
mk()  { body POST /session -H "$JS" -d "{\"username\":\"$1\"}" | id; }
JS='Content-Type: application/json'
PNG=db/photos/00000000-0000-4000-8000-000000000000
photo() { sed 's/.*"photo":"\/photos\/\([^"]*\)".*/\1/'; }
DATA()  { printf 'data={"name":"%s","members":[%s]};type=application/json' "$1" "$2"; }

echo "S1 — the token of doLogin is accepted by authenticate"
U=$(mk "s1$R")
eq "S1 getUsers with that token" 200 "$(code GET "/users?username=s1$R" -H "Authorization: Bearer $U")"

echo "S2 — the token survives a rename"
U=$(mk "s2$R")
eq "S2 rename"       200 "$(code PUT /me/username -H "Authorization: Bearer $U" -H "$JS" -d "{\"username\":\"s2b$R\"}")"
eq "S2 same token ok" 200 "$(code GET "/users?username=s2b$R" -H "Authorization: Bearer $U")"

echo "S3 — a freed username is a new account"
U1=$(mk "s3$R")
eq "S3 rename away" 200 "$(code PUT /me/username -H "Authorization: Bearer $U1" -H "$JS" -d "{\"username\":\"s3b$R\"}")"
eq "S3 old name is new account" 201 "$(code POST /session -H "$JS" -d "{\"username\":\"s3$R\"}")"
U2=$(mk "s3$R")
ne "S3 different id"  "$U1" "$U2"
eq "S3 new name is the old account" "$U1" "$(mk "s3b$R")"

echo "S4 — a taken username is released"
V1=$(mk "s4a$R"); V2=$(mk "s4b$R")
eq "S4 taken"    400 "$(code PUT /me/username -H "Authorization: Bearer $V1" -H "$JS" -d "{\"username\":\"s4b$R\"}")"
eq "S4 owner moves" 200 "$(code PUT /me/username -H "Authorization: Bearer $V2" -H "$JS" -d "{\"username\":\"s4c$R\"}")"
eq "S4 now free" 200 "$(code PUT /me/username -H "Authorization: Bearer $V1" -H "$JS" -d "{\"username\":\"s4b$R\"}")"

echo "S5 — a photo outlives its replacement only while a row points at it"
A=$(mk "s5a$R"); B=$(mk "s5b$R"); AU="Authorization: Bearer $A"
P1=$(body PUT /me/photo -H "$AU" -F "photoFile=@$PNG" | photo)
P2=$(body PUT /me/photo -H "$AU" -F "photoFile=@$PNG" | photo)
ne "S5 new upload new id" "$P1" "$P2"
eq "S5 replaced photo collected" 404 "$(code GET /photos/$P1 -H "$AU")"
eq "S5 current photo served"     200 "$(code GET /photos/$P2 -H "$AU")"
G=$(body POST /groups -H "$AU" -F "$(DATA "S5 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
GP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
eq "S5 group photo kept while the group exists" 200 "$(code GET /photos/$GP -H "$AU")"

echo "S6 — chat and group do not collide"
A=$(mk "s6a$R"); B=$(mk "s6b$R"); AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"
eq "S6 first create" 201 "$(code POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
C=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
eq "S6 second is 200" 200 "$(code POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}")"
eq "S6 other side same chat" "$C" "$(body POST /private_chats -H "$BU" -H "$JS" -d "{\"id\":\"$A\"}" | id)"
G=$(body POST /groups -H "$AU" -F "$(DATA "S6 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
ne "S6 group is not the chat" "$C" "$G"
eq "S6 chat unchanged after the group" "$C" "$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)"

echo "S7 — nothing survives a refused write"
A=$(mk "s7a$R"); AU="Authorization: Bearer $A"
NP=$(ls db/photos | wc -l | tr -d ' '); NC=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats;")
eq "S7 unknown member 404" 404 "$(code POST /groups -H "$AU" -F "$(DATA "S7 $R" "\"11111111-2222-4333-8444-555555555555\"")" -F "photoFile=@$PNG")"
eq "S7 no photo written" "$NP" "$(ls db/photos | wc -l | tr -d ' ')"
eq "S7 no chat written"  "$NC" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats;")"

echo "S8 — a rename touches the name and nothing else"
A=$(mk "s8a$R"); B=$(mk "s8b$R"); AU="Authorization: Bearer $A"
G=$(body POST /groups -H "$AU" -F "$(DATA "S8 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
GP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
R1=$(body PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"First"}')
R2=$(body PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"Second"}')
has "S8 photo unchanged 1" "\"photo\":\"/photos/$GP\"" "$R1"
has "S8 photo unchanged 2" "\"photo\":\"/photos/$GP\"" "$R2"
eq  "S8 photo still served" 200 "$(code GET /photos/$GP -H "$AU")"

echo "S9 — the two kinds stay apart under /groups/"
A=$(mk "s9a$R"); B=$(mk "s9b$R"); AU="Authorization: Bearer $A"
C=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
eq "S9 setGroupName on a private chat" 404 "$(code PUT /groups/$C/name -H "$AU" -H "$JS" -d '{"name":"x"}')"
eq "S9 private chat row untouched" "NULL|NULL" "$(sqlite3 db/wasatext.db "SELECT quote(name)||'|'||quote(photoId) FROM chats WHERE id='$C';")"
G=$(body POST /groups -H "$AU" -F "$(DATA "S9 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
eq "S9 setGroupName on a group" 200 "$(code PUT /groups/$G/name -H "$AU" -H "$JS" -d '{"name":"x"}')"

echo "S10 — a member added is a member for every other call"
A=$(mk "s10a$R"); B=$(mk "s10b$R"); C=$(mk "s10c$R"); D=$(mk "s10d$R")
AU="Authorization: Bearer $A"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S10 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
eq "S10 add C"            200 "$(code POST /groups/$G/members -H "$AU" -H "$JS" -d "{\"members\":[\"$C\"]}")"
eq "S10 C may rename"     200 "$(code PUT /groups/$G/name  -H "$CU" -H "$JS" -d '{"name":"by C"}')"
eq "S10 C may set photo"  200 "$(code PUT /groups/$G/photo -H "$CU" -F "photoFile=@$PNG")"
eq "S10 C may add"        200 "$(code POST /groups/$G/members -H "$CU" -H "$JS" -d "{\"members\":[\"$D\"]}")"

echo "S11 — leaving takes the whole group surface away"
A=$(mk "s11a$R"); B=$(mk "s11b$R"); C=$(mk "s11c$R")
AU="Authorization: Bearer $A"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S11 $R" "\"$B\",\"$C\"")" -F "photoFile=@$PNG" | id)
eq "S11 C leaves"           204 "$(code DELETE /groups/$G/members/me -H "$CU")"
eq "S11 C cannot rename"    403 "$(code PUT /groups/$G/name  -H "$CU" -H "$JS" -d '{"name":"x"}')"
eq "S11 C cannot set photo" 403 "$(code PUT /groups/$G/photo -H "$CU" -F "photoFile=@$PNG")"
eq "S11 C cannot add"       403 "$(code POST /groups/$G/members -H "$CU" -H "$JS" -d "{\"members\":[\"$B\"]}")"
eq "S11 C cannot leave again" 403 "$(code DELETE /groups/$G/members/me -H "$CU")"

echo "S12 — a group outlives every member but the last"
A=$(mk "s12a$R"); B=$(mk "s12b$R"); AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"
G=$(body POST /groups -H "$AU" -F "$(DATA "S12 $R" "\"$B\"")" -F "photoFile=@$PNG" | id)
GP=$(sqlite3 db/wasatext.db "SELECT photoId FROM chats WHERE id='$G';")
eq "S12 A leaves"        204 "$(code DELETE /groups/$G/members/me -H "$AU")"
eq "S12 B still inside"  1   "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chat_members WHERE chatId='$G';")"
eq "S12 photo still served" 200 "$(code GET /photos/$GP -H "$BU")"
eq "S12 B leaves last"   204 "$(code DELETE /groups/$G/members/me -H "$BU")"
eq "S12 chats row gone"  0   "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats WHERE id='$G';")"
eq "S12 photo collected" 404 "$(code GET /photos/$GP -H "$BU")"
eq "S12 404 for A"       404 "$(code DELETE /groups/$G/members/me -H "$AU")"
eq "S12 404 for B"       404 "$(code DELETE /groups/$G/members/me -H "$BU")"

echo "S13 — the list is the membership, seen from the other side"
A=$(mk "s13a$R"); N=$(mk "s13n$R"); AU="Authorization: Bearer $A"; NU="Authorization: Bearer $N"
eq "S13 fresh user has no chats" 404 "$(code GET /me/chats -H "$NU")"
P=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$N\"}" | id)
has "S13 chat named after the other, for N" "s13a$R" "$(body GET /me/chats -H "$NU")"
has "S13 chat named after the other, for A" "s13n$R" "$(body GET /me/chats -H "$AU")"
G=$(body POST /groups -H "$AU" -F "$(DATA "S13 $R" "\"$N\"")" -F "photoFile=@$PNG" | id)
has "S13 group appears for N" "\"id\":\"$G\"" "$(body GET /me/chats -H "$NU")"
eq  "S13 N leaves the group" 204 "$(code DELETE /groups/$G/members/me -H "$NU")"
hasnt "S13 group gone from N's list" "\"id\":\"$G\"" "$(body GET /me/chats -H "$NU")"
has  "S13 group still in A's list"   "\"id\":\"$G\"" "$(body GET /me/chats -H "$AU")"
eq  "S13 N is left with the private chat alone" 1 "$(body GET /me/chats -H "$NU" | grep -o '"id":"[0-9a-f-]*","chatType"' | wc -l | tr -d ' ')"

echo "S14 — a rename reaches the chats list of somebody else"
A=$(mk "s14a$R"); B=$(mk "s14b$R"); AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"
body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" > /dev/null
has "S14 A reads B's old name" "s14b$R" "$(body GET /me/chats -H "$AU")"
CHATS_BEFORE=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats;")
eq  "S14 B renames" 200 "$(code PUT /me/username -H "$BU" -H "$JS" -d "{\"username\":\"s14renamed$R\"}")"
has "S14 A reads the new name" "s14renamed$R" "$(body GET /me/chats -H "$AU")"
eq  "S14 no row written to chats" "$CHATS_BEFORE" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats;")"

echo "S15 — the homepage preview is the last message of the opened chat"
A=$(mk "s15a$R"); B=$(mk "s15b$R"); AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"
P=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
LONG=$(printf 'abcdefghij%.0s' $(seq 1 12))
# both are written by the endpoint, so their order is the one the server gave them
body POST /chats/$P/messages -H "$BU" -F 'text=older one' > /dev/null
NEWEST=$(body POST /chats/$P/messages -H "$BU" -F "text=$LONG" | id)
LIST=$(body GET /me/chats -H "$AU")
DET=$(body GET /chats/$P -H "$AU")
eq  "S15 the preview is the newest message" \
    "$(echo "$LIST" | sed 's/.*"snippet":{"id":"\([^"]*\)".*/\1/')" \
    "$(echo "$DET" | sed 's/.*"messages":\[{"id":"\([^"]*\)".*/\1/')"
eq  "S15 and it is the one just written" "$NEWEST" "$(echo "$DET" | sed 's/.*"messages":\[{"id":"\([^"]*\)".*/\1/')"
has "S15 the list cuts the text to 50" "\"text\":\"$(printf 'abcdefghij%.0s' $(seq 1 5))\"" "$LIST"
has "S15 the opened chat keeps it whole" "\"text\":\"$LONG\"" "$DET"
eq  "S15 the older message is only in the opened chat" 2 "$(echo "$DET" | grep -o '"state":"' | wc -l | tr -d ' ')"
hasnt "S15 the list carries no messages list" '"messages":' "$LIST"
has "S15 the opened chat carries no preview" '"messages":' "$DET"
# opening the chat changes exactly one thing in the list: the state of the snippet, which is what
# marking it read means. The two reads are otherwise the same bytes, preview and ordering included
AFTER=$(body GET /me/chats -H "$AU")
has "S15 the snippet was received before the chat was opened" '"state":"received"' "$LIST"
has "S15 and reads read once it has been"                     '"state":"read"'     "$AFTER"
eq  "S15 opening the chat changes nothing else in the list" \
    "$(echo "$LIST"  | sed 's/"state":"[a-z]*"/"state":"X"/g')" \
    "$(echo "$AFTER" | sed 's/"state":"[a-z]*"/"state":"X"/g')"

echo "S16 — a forwarded message becomes the destination's message and preview without changing its source"
A=$(mk "s16a$R"); B=$(mk "s16b$R"); C=$(mk "s16c$R")
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"; CU="Authorization: Bearer $C"
PS=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$B\"}" | id)
PD=$(body POST /private_chats -H "$AU" -H "$JS" -d "{\"id\":\"$C\"}" | id)
SRC=$(body POST /chats/$PS/messages -H "$BU" -F 'text=forwarded across chats' -F "photoFile=@$PNG")
MS=$(echo "$SRC" | id); PSRC=$(echo "$SRC" | photo)
FWD=$(body POST /chats/$PD/forwards -H "$AU" -H "$JS" -d "{\"messageId\":\"$MS\"}")
MF=$(echo "$FWD" | id); PFWD=$(echo "$FWD" | photo)
ne  "S16 the copy has a new id" "$MS" "$MF"
eq  "S16 the copy reuses the exact photo" "$PSRC" "$PFWD"
has "S16 the copy carries the source text" '"text":"forwarded across chats"' "$FWD"
has "S16 the caller is the copy's sender" "\"user\":\"$A\"" "$FWD"
SRC_AFTER=$(body GET /chats/$PS -H "$AU")
has "S16 the source message remains in its chat" "\"id\":\"$MS\"" "$SRC_AFTER"
has "S16 the source keeps its original sender" "\"user\":\"$B\"" "$SRC_AFTER"
LIST=$(body GET /me/chats -H "$CU")
has "S16 the destination preview is the copy" "\"snippet\":{\"id\":\"$MF\"" "$LIST"
has "S16 its preview is received before C opens it" '"state":"received"' "$LIST"
DEST=$(body GET /chats/$PD -H "$CU")
has "S16 the opened destination contains the copy" "\"id\":\"$MF\"" "$DEST"
has "S16 the opened destination carries the same photo" "\"photo\":\"/photos/$PFWD\"" "$DEST"
has "S16 the preview becomes read after C opens it" '"state":"read"' "$(body GET /me/chats -H "$CU")"

echo "S17 — reactions update one message without creating another or changing its preview"
A=$(mk "s17a$R"); B=$(mk "s17b$R"); C=$(mk "s17c$R")
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S17 $R" "\"$B\",\"$C\"")" -F "photoFile=@$PNG" | id)
M=$(body POST /chats/$G/messages -H "$BU" -F 'text=keep this preview' | id)
CP=/chats/$G/messages/$M/comments/me
N0=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")
RA=$(body PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍"}')
A1=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$A';")
has "S17 A's reaction is returned on the message" "\"user\":\"$A\",\"emoji\":\"👍\"" "$RA"
body PUT "$CP" -H "$BU" -H "$JS" -d '{"emoji":"😂"}' > /dev/null
LIST=$(body GET /me/chats -H "$AU")
has "S17 reacting does not replace the preview message" "\"snippet\":{\"id\":\"$M\"" "$LIST"
has "S17 the preview content is unchanged" '"text":"keep this preview"' "$LIST"
has "S17 the preview remains received while C is behind" '"state":"received"' "$LIST"
OPEN=$(body GET /chats/$G -H "$CU")
has "S17 opening the chat shows A's reaction" "\"user\":\"$A\",\"emoji\":\"👍\"" "$OPEN"
has "S17 opening the chat shows B's reaction" "\"user\":\"$B\",\"emoji\":\"😂\"" "$OPEN"
RA2=$(body PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"🔥"}')
A2=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$A';")
eq  "S17 updating A's reaction preserves its id" "$A1" "$A2"
has "S17 update contains the new reaction" "\"user\":\"$A\",\"emoji\":\"🔥\"" "$RA2"
hasnt "S17 update removes A's old reaction" "\"user\":\"$A\",\"emoji\":\"👍\"" "$RA2"
has "S17 state is read after every member has caught up" '"state":"read"' "$RA2"
eq  "S17 still one row per reacting member" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$M';")"
eq  "S17 reacting never creates another message" "$N0" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")"

echo "S18 — removing and recreating my reaction changes only my comment resource"
A=$(mk "s18a$R"); B=$(mk "s18b$R"); C=$(mk "s18c$R")
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S18 $R" "\"$B\",\"$C\"")" -F "photoFile=@$PNG" | id)
M=$(body POST /chats/$G/messages -H "$BU" -F 'text=keep this after uncomment' | id)
CP=/chats/$G/messages/$M/comments/me
N0=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")
body PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍"}' > /dev/null
A1=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$A';")
body PUT "$CP" -H "$BU" -H "$JS" -d '{"emoji":"😂"}' > /dev/null
eq "S18 A removes its own reaction" 204 "$(code DELETE "$CP" -H "$AU")"
eq "S18 A's row is gone" 0 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$M' AND userId='$A';")"
OPEN=$(body GET /chats/$G -H "$CU")
hasnt "S18 opened chat drops A's reaction" "\"user\":\"$A\",\"emoji\":\"👍\"" "$OPEN"
has "S18 opened chat keeps B's reaction" "\"user\":\"$B\",\"emoji\":\"😂\"" "$OPEN"
LIST=$(body GET /me/chats -H "$AU")
has "S18 preview is still the same message" "\"snippet\":{\"id\":\"$M\"" "$LIST"
has "S18 preview content is unchanged" '"text":"keep this after uncomment"' "$LIST"
hasnt "S18 preview does not project comments" '"comments":' "$LIST"
has "S18 preview state stays read" '"state":"read"' "$LIST"
eq "S18 recreating A's reaction is a create" 201 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"🔥"}')"
A2=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$A';")
ne "S18 recreation gets a new comment id" "$A1" "$A2"
eq "S18 A removes the recreated reaction" 204 "$(code DELETE "$CP" -H "$AU")"
eq "S18 B remains the only commenter" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$M' AND userId='$B';")"
eq "S18 uncommenting never removes the message" "$N0" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")"

echo "S19 — the comments collection follows create, update and delete while catching its reader up"
A=$(mk "s19a$R"); B=$(mk "s19b$R"); C=$(mk "s19c$R")
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S19 $R" "\"$B\",\"$C\"")" -F "photoFile=@$PNG" | id)
M=$(body POST /chats/$G/messages -H "$BU" -F 'text=read only comments collection' | id)
CP=/chats/$G/messages/$M/comments/me
CC=/chats/$G/messages/$M/comments
N0=$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")
eq "S19 empty collection" 404 "$(code GET "$CC" -H "$CU")"
eq "S19 A creates a reaction" 201 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"👍"}')"
AID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$A';")
eq "S19 B creates a newer reaction" 201 "$(code PUT "$CP" -H "$BU" -H "$JS" -d '{"emoji":"😂"}')"
BID=$(sqlite3 db/wasatext.db "SELECT id FROM comments WHERE messageId='$M' AND userId='$B';")
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2000-01-01T00:00:00.000Z' WHERE chatId='$G' AND userId='$C';"
eq "S19 collection is newest first" \
   "{\"comments\":[{\"id\":\"$BID\",\"user\":\"$B\",\"emoji\":\"😂\"},{\"id\":\"$AID\",\"user\":\"$A\",\"emoji\":\"👍\"}]}" \
   "$(body GET "$CC" -H "$CU")"
eq "S19 reading comments catches the reader up" 1 \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate > '2000-01-01T00:00:00.000Z' FROM chat_members WHERE chatId='$G' AND userId='$C';")"
eq "S19 A updates its reaction" 200 "$(code PUT "$CP" -H "$AU" -H "$JS" -d '{"emoji":"🔥"}')"
eq "S19 update keeps id and position" \
   "{\"comments\":[{\"id\":\"$BID\",\"user\":\"$B\",\"emoji\":\"😂\"},{\"id\":\"$AID\",\"user\":\"$A\",\"emoji\":\"🔥\"}]}" \
   "$(body GET "$CC" -H "$CU")"
eq "S19 B removes its reaction" 204 "$(code DELETE "$CP" -H "$BU")"
eq "S19 collection now contains only A" \
   "{\"comments\":[{\"id\":\"$AID\",\"user\":\"$A\",\"emoji\":\"🔥\"}]}" \
   "$(body GET "$CC" -H "$CU")"
eq "S19 A removes the final reaction" 204 "$(code DELETE "$CP" -H "$AU")"
eq "S19 collection is absent again" 404 "$(code GET "$CC" -H "$CU")"
eq "S19 the message survives the whole lifecycle" "$N0" "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM messages WHERE chatId='$G';")"

echo "S20 — deleting the latest message restores the previous preview and an empty chat survives"
A=$(mk "s20a$R"); B=$(mk "s20b$R"); C=$(mk "s20c$R")
AU="Authorization: Bearer $A"; BU="Authorization: Bearer $B"; CU="Authorization: Bearer $C"
G=$(body POST /groups -H "$AU" -F "$(DATA "S20 $R" "\"$B\",\"$C\"")" -F "photoFile=@$PNG" | id)
M1=$(body POST /chats/$G/messages -H "$BU" -F 'text=the previous preview' | id)
M2=$(body POST /chats/$G/messages -H "$AU" -F 'text=delete the latest preview' | id)
P2=/chats/$G/messages/$M2
eq "S20 B comments on latest" 201 "$(code PUT "$P2/comments/me" -H "$BU" -H "$JS" -d '{"emoji":"👍"}')"
eq "S20 C comments on latest" 201 "$(code PUT "$P2/comments/me" -H "$CU" -H "$JS" -d '{"emoji":"😂"}')"
eq "S20 latest message is the preview" "$M2" "$(body GET /me/chats -H "$AU" | sed 's/.*"snippet":{"id":"\([^"]*\)".*/\1/')"
eq "S20 another member cannot delete it" 403 "$(code DELETE "$P2" -H "$BU")"
eq "S20 refused delete keeps both comments" 2 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM comments WHERE messageId='$M2';")"
sqlite3 db/wasatext.db "UPDATE chat_members SET lastReadDate='2000-01-01T00:00:00.000Z' WHERE chatId='$G' AND userId='$A';"
eq "S20 sender deletes latest message" 204 "$(code DELETE "$P2" -H "$AU")"
eq "S20 latest message and comments cascade" '0|0' \
   "$(sqlite3 db/wasatext.db "SELECT (SELECT COUNT(*) FROM messages WHERE id='$M2')||'|'||(SELECT COUNT(*) FROM comments WHERE messageId='$M2');")"
eq "S20 deletion catches sender up" 1 \
   "$(sqlite3 db/wasatext.db "SELECT lastReadDate > '2000-01-01T00:00:00.000Z' FROM chat_members WHERE chatId='$G' AND userId='$A';")"
LIST=$(body GET /me/chats -H "$AU")
has "S20 preview falls back to previous message" "\"snippet\":{\"id\":\"$M1\"" "$LIST"
hasnt "S20 deleted message is absent from list" "\"id\":\"$M2\"" "$LIST"
eq "S20 previous sender deletes final message" 204 "$(code DELETE /chats/$G/messages/$M1 -H "$BU")"
eq "S20 group survives empty" 1 "$(sqlite3 db/wasatext.db "SELECT COUNT(*) FROM chats WHERE id='$G';")"
OPEN=$(body GET /chats/$G -H "$AU")
has "S20 opened empty group has no messages" '"messages":[]' "$OPEN"
LIST=$(body GET /me/chats -H "$AU")
has "S20 empty group remains in chat list" "\"id\":\"$G\"" "$LIST"
hasnt "S20 empty group has no snippet" '"snippet":' "$LIST"

echo
echo "==================================================="
echo "PASS: $PASS   FAIL: $FAIL"
if [ $FAIL -gt 0 ]; then printf '%s\n' "${FAILED[@]}"; fi
echo "==================================================="
