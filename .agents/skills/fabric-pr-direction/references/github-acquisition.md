# GitHub Context Acquisition

## Native connector

Prefer a GitHub connector or provider-native GitHub tool already available to the agent. Use read operations only. Fetch PR metadata, body, linked issues, conversation comments, reviews, inline review comments, files, commits, checks, authors, and stable URLs.

Do not require a particular connector name. Inspect the available tools and use the narrowest read capability that provides the required context.

## gh fallback

Verify installation and authentication:

    command -v gh
    gh auth status
    gh repo view --json nameWithOwner

Fetch the main PR context:

    gh pr view "<pr>" --json number,title,body,url,state,author,reviewDecision,comments,reviews,files,commits,statusCheckRollup,closingIssuesReferences

Fetch paginated inline review comments when the main response does not include them:

    gh api --paginate "repos/{owner}/{repo}/pulls/<pr>/comments"

Use gh output as agent context; do not add a Fabric GitHub wrapper or store credentials. If gh is missing or unauthenticated, stop and tell the user which prerequisite failed.
