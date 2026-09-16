# WASAtext Homework 3 (Vue) — Interactive Functionality Check

We strongly encourage every WASA student to verify that their code correctly performs the main operations required by the project specification, and we provide a checklist below for execution.

## Suggested Setup

### a) Run your application as follows

**Backend:**

- Set Go toolchain version to 1.25.1:
  1. On Windows: `set GOTOOLCHAIN=go1.25.1`
  2. On Mac and Linux: `export GOTOOLCHAIN=go1.25.1`
- Clean go.mod and vendor dependencies locally:
  1. `go mod tidy`
  2. `go mod vendor`
- Finally, try to run the backend and ensure it works:
  1. `go run ./cmd/webapi`

**Frontend:**

- Enter the Node.js container:
  1. `./open-node.sh`
- Install deps, build, and run:
  1. `yarn install`
  2. `yarn run vite build --mode production`
  3. `yarn run vite preview --host 0.0.0.0 --port 4173 --strictPort`

### b) Multiple users

Use multiple windows or browsers to log in multiple users simultaneously.

### c) Cross-device testing

Make some tests using a different device (e.g., a mobile phone or a second laptop) connected to the same network.

- In order to make this test, you need to temporarily change `localhost:3000` with your local IP address in `vite.config.js` (don't push this change as it only works for your home network), otherwise other devices will try to contact the backend at `localhost`, which will fail.

## Checklist

1. **User Login (Hard Constraint):** Three users (A, B, and C) log in to the application.
2. **Simple direct message (Hard Constraint):** User A sends a message to B.
3. **Checkmarks:** A single checkmark is displayed in A's chat. When B opens the chat view, 2 checkmarks are displayed in A's chat. No checkmarks are displayed in B's chat.
4. **Auto-refresh:** Conversation lists, chat views, and checkmarks are automatically updated without manual page refreshes.
5. **Media Message:** User A sends a single message to User B containing both text and an uploaded image (no URL).
6. **Replying:** User B replies to all User A's messages. A snippet of the original text and the image icon (when replying to an image message) must be displayed in the reply.
7. **Reactions I:** User B reacts to the first A's message (with a single emoji). The reaction must be visible in both A's and B's chat views. User B must be restricted to adding no more than one reaction (but can remove it).
8. **Group Creation:** User A creates a group G including User B and User C, and sends a message to G.
9. **Read Receipts:** User A monitors the message indicator. The double checkmark indicator must appear only when both B and C have opened the message.
10. **Reactions II:** User B reacts to A's message (with a single emoji). User A must see that B is the author of the reaction.
11. **Group Management:** User C leaves the group. C can no longer receive messages from the group, read its contents, or post to it.
12. **Forwarding:** User B forwards User A's message to User C. User C must see a clear marker indicating the message was "Forwarded."
13. **Profile Constraint:** User A attempts to change their profile name to User B's name. This operation must fail and result in a clear error message.
14. **Profile Update:** User A changes their profile name (to an available one) and their profile picture. The changes must propagate automatically to all open chat views, chat lists, and contact lists.
15. **Group Branding:** User B changes the name and picture of Group G. The update must propagate instantly to all users' chat lists.
16. **Persistence (Hard Constraint):** User B logs out and then logs back in. All previous messages, groups, and profile data must persist correctly.

---

This checklist will be verified during the final homework evaluation, conducted on the last Thursday before each oral appeal.

**We will NOT ADMIT to the oral examination:**

- Projects that do not comply with the hard constraints
- Projects that fail to start when launched according to the specified instructions
- Projects that cannot run on a different device on the same network

In addition, **2 points will be deducted** for each checklist item that fails verification.
