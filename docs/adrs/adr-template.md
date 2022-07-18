# ADR Format Template

This template provides a canvas for generating ADRs and a standard format so that we can build tools to parse them.
- notes are added to this template to help elaborate on the points without a separate document
- notes will be prefixed with a dash

## Established

2022-07-20
- this section should contain only the YYYY-MM-DD date of when the decision is considered final
- this can be added after context is given, in the PR which will wait for 1 week before merge

## Revisit by

2023-07-15
- this section should contain only the YYYY-MM-DD date of when the decision is considered stale
- at the next design discussion we should validate and renew this date

## Subject

Given `data`, when `triggering event`, then we `do something`.

- the person should be [first person plural](https://en.wikipedia.org/wiki/Grammatical_person)
  - "we" do something
  - not "I", "you", or "they"
- the tense should be [simple present](https://courses.dcs.wisc.edu/wp/grammar/category/tense-and-mood/), 
  - we "do" something
  - not "does", "doing", "did", or "done"
- the mood should be [indicative](https://osuwritingcenter.okstate.edu/blog/2020/11/6/the-five-grammatical-moods)
  - we "do" something
  - not "go do"
- Given when then statements should be used as often as possible to get as much context into the subject as possible.
- Don't force 'given, when, then'; if there is no triggering event or no data given, then leave those parts out.

## Status

Accepted / Rejected / Superseded by #other-issue
- accepted is the decision that the subject is appropriate and we will do it.
- rejected is the decision that the subject isn't appropriate and we won't do it.
- superseded relates that a different decision forces this decision (for instance a decision made at a higher level of abstraction)

## Context

- the following is a simple framework for judging a decision, these items are not required, but may be useful to the writer.
### Strength of doing process
### Weakness of doing process
### Threats involved in not doing process
### Threats involved in doing process
### Opportunities involved in doing process

- a different approach to context framework
### Pros
### Cons
