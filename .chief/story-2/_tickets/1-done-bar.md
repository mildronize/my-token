# 1: What does "done" look like for story-2?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

Story-1's map already says fleet-wide multi-ship rollout is a separate
follow-up story. มายด์ gave five scope points for that follow-up (multi-machine
correctness, filter by machine, filter by registered path name, localStorage
persistence, by-path/by-agent dedup). Does "done" require all five fully
built, or is something deferrable if it turns out gnarly (especially point 5,
which มายด์ flagged himself as the fuzziest)?

## Answer

**All five, fully built — nothing deferred out of this story.** Second real
machine identity proven, cross-machine `path` merge/split actually correct
(not just UI-disambiguated), machine filter, registered-path-name filter,
localStorage persistence, and the by-path/by-agent dedup rule all ship before
this story closes.
