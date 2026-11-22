# Brief

The goal of this project, verbatim as the user gave it on 2026-11-22. This file is the yardstick for "aligned to the goal".

Do not edit it. Interpretations, scope choices and trade-offs belong in `docs/decisions/`.

---

Build a simple, real-time messaging system along the lines of Slack. Focus on the messaging experience and activity feed rather than trying to build a complete product. Build enough to exercise the activity feed: creating messages and replies, mentions/reactions, and editing or deleting messages.

The activity feed should feel like something a real person could use. We do care about product craft and the decisions behind what you build.

Messages and their later edits or deletions also need to make their way to several independent systems: a search index, a notification sender, and the activity feed itself. These systems communicate over the network and can be slow or unavailable. How you make that reliable, and what guarantees you choose to provide, is part of the task.

The search index and notification sender can be simple local or in-process fakes. You do not need to build or deploy separate services. Treat their boundaries as if they were real external systems. Seed enough data that the experience resembles a busy team rather than an empty demo.
