TRACK 1: PRIVACY BOUNDARY
Your Checkpoint-native experience must now work with sensitive repositories.

The security team has introduced these requirements:

Raw prompts and transcripts must not be sent to a new external service.

The product must continue to provide useful output when sensitive fields are redacted or unavailable.

Existing local functionality must continue to work.

The interface must clearly distinguish between complete and incomplete context.

You must include at least one test using redacted or missing Checkpoint data.

Use Entire Graph to identify every code path affected by this privacy boundary.

Your product must never present incomplete context as a complete or authoritative result.

A redacted Checkpoint fixture is attached.