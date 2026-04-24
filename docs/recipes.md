# Recipes

Short patterns for composing `qq` with other tools. See
[decision-mode.md](decision-mode.md) for the `--if`/`--unless`
contract.

## Git

### Commit header from staged diff

Generate a one-line commit message from whatever is staged.

```
git commit -m "$(git diff --staged | qq 'one-line commit header, imperative mood')"
```

### Pre-commit guard

Block commits that sneak in secrets or debug prints. Install as
`.git/hooks/pre-commit`.

```
#!/bin/sh
git diff --staged | qq --unless "any secrets, api keys, tokens, or debug prints?" >/dev/null
```

### Release notes paragraph

Turn a commit range into a short release paragraph.

```
git log --no-merges --format='%B%n---%n' v1.2.0..v1.3.0 \
  | qq "short release paragraph, 2-3 sentences, lead with the top user-facing change, exclude refactors/tests/CI/deps"
```

### PR description draft

Drafts a summary from the commits on a branch.

```
git log origin/main..HEAD --format='%B%n---%n' \
  | qq "draft a PR description with Summary (2-3 bullets) and Test plan sections"
```

## CI

### Retry flaky tests

Shell does the retry loop; `qq --if` classifies each failure as
flaky or real.

```
for i in 1 2 3; do
  out=$(go test ./... 2>&1) && { echo "$out"; exit 0; }
  qq --if "is this a flaky Go test failure (timeout, race, network, transient)?" <<<"$out" >/dev/null \
    || { echo "$out"; exit 1; }
done
exit 1
```

### Summarize a failure into the job summary

On a red run, append a one-line root cause to the GitHub Actions
job summary. Doesn't affect pass/fail.

```
out=$(go test ./... 2>&1) && { echo "$out"; exit 0; }
echo "$out"
{
  echo "## Failure summary"
  qq "one-line root cause of this failure, plain prose" <<<"$out"
} >> "$GITHUB_STEP_SUMMARY"
exit 1
```

### Skip heavy CI on docs-only changes

Early-exit a job when the diff doesn't touch code.

```
if git diff origin/main..HEAD | qq --unless "any code changes, not just docs or comments?" >/dev/null; then
  echo "docs-only, skipping"
  exit 0
fi
```

## Ops

### Page oncall only on real errors

Gate alerting on log content.

```
tail -n 200 app.log | qq --if "real error (stack trace, crash, unrecoverable)?" >/dev/null && page_oncall
```

### Sanity-gate a `curl | sh`

Let qq pass the install script through only if nothing looks off.

```
curl -fsSL https://install.example/setup.sh \
  | qq --unless "does this look malicious or unexpected?" \
  | sh
```

### Root-cause a runtime log

Quick triage on noisy output.

```
kubectl logs pod-xyz --tail=500 | qq "what component is failing and why, one paragraph"
```

## Generation

### Generate from a prompt

Arg-only, redirect to disk.

```
qq "a .gitignore for a Python + Node project" > .gitignore
```

### Expand an example into a file

Pipe a sample record in, describe how many and how varied, redirect
to disk. Works for test fixtures, seed data, mock API responses.

```
qq "give me 50 varied entries as jsonl in this format" \
  <<<'{"name":"John","city":"NYC","role":"engineer"}' \
  > users.jsonl
```

## Transforms

### CSV to markdown table

```
cat data.csv | qq "as a markdown table" >> README.md
```

### JSON blob to bullet list

For release notes, Slack, a ticket.

```
curl -s api.example.com/release/123 | qq "as a short bullet list, human-readable"
```

### Log lines to a table

```
tail -50 app.log | qq "as a markdown table with columns: time, level, component, message"
```

### CLI help to cheat sheet

```
kubectl --help | qq "as a one-page cheat sheet, grouped by noun"
```

### Messy list to starter JSONL

Best-effort; eyeball before using.

```
cat contacts.txt | qq "as jsonl with {name, email, phone}" > contacts.jsonl
```

### HTML table to JSON

For a one-off analysis.

```
curl -s https://example.com/prices | qq "the pricing table as json array with {tier, price, seats}"
```

### Keep only rows matching a rule

```
cat tickets.md | qq "keep only rows that look like real bugs (not feature requests or questions); output the same markdown table, same columns"
```

Best-effort; eyeball before using.

### Redact PII from a file

```
cat notes.txt \
  | qq "replace any PII (names, emails, phones, addresses) with [REDACTED]; leave the rest unchanged" \
  > notes.redacted.txt
```
