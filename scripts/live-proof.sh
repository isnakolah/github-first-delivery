#!/bin/sh
set -eu

# Creates only an exact generated private target. It never deletes a user-supplied
# repository. `delete_repo` is required for final cleanup.
if [ "${GFD_LIVE_PROOF:-}" != "1" ]; then
  printf '%s\n' 'set GFD_LIVE_PROOF=1 to create disposable GitHub proof resources' >&2
  exit 2
fi

owner=${GFD_LIVE_OWNER:-$(gh api user --jq .login)}
repo="gfd-proof-$(date -u +%Y%m%d%H%M%S)"
project_title="GFD proof $repo"
printf '%s\n' "creating $owner/$repo"
gh repo create "$owner/$repo" --private --description 'Disposable GFD live-proof resource; safe to delete'
project=$(gh project create --owner "$owner" --title "$project_title" --format json)
project_number=$(printf '%s' "$project" | jq -r .number)

cleanup() {
  gh project delete "$project_number" --owner "$owner" || true
  if ! gh repo delete "$owner/$repo" --yes; then
    printf '%s\n' "cleanup blocked; private repository retained: $owner/$repo" >&2
    exit 1
  fi
}
trap cleanup EXIT INT TERM

epic=$(gh issue create --repo "$owner/$repo" --title 'EPIC — Disposable GFD proof' --body 'Disposable integration proof.')
child=$(gh issue create --repo "$owner/$repo" --title 'US-PROOF — Verify native parent' --body '<!-- work:v1 -->
## Outcome

Verify native parent link.

## Scope
Includes:
- Disposable relationship.

Excludes:
- Production work.

## Acceptance criteria
- [ ] Parent exists.

## Required evidence
- API output.
- Verification boundary: provider

## Documentation impact
None: disposable proof.

## Source and relationships
- US-016.
- Native parent and blocker links are authoritative.')
epic_number=${epic##*/}; child_number=${child##*/}
ids=$(gh api graphql -f query="query { repository(owner:\"$owner\",name:\"$repo\") { e:issue(number:$epic_number){id} c:issue(number:$child_number){id} } }" --jq '.data.repository | [.e.id,.c.id] | @tsv')
tab=$(printf '\t')
epic_id=${ids%%"$tab"*}; child_id=${ids#*"$tab"}
gh api graphql -f query="mutation { addSubIssue(input:{issueId:\"$epic_id\",subIssueId:\"$child_id\"}) { subIssue { number } } }" >/dev/null
gh project item-add "$project_number" --owner "$owner" --url "$epic" >/dev/null
gh project item-add "$project_number" --owner "$owner" --url "$child" >/dev/null
test "$(gh project item-list "$project_number" --owner "$owner" --limit 10 --format json --jq '.items | length')" = 2
printf '%s\n' "live proof passed for $owner/$repo; cleanup follows"
