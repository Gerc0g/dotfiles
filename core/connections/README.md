# Company connections

`connections` is the HQ policy owner. The authenticated owner control adapter
calls `Dispatch(operation, jsonArgs)`. A task must call `Execute(ctx, binding,
operation, jsonArgs)` through its runner's private socket. The trusted runner
supplies `CompanyID` and the registered `company/product/repo` identity;
request arguments cannot override that binding. `WorktreePath` is never opened
by this package.

Settings and opaque credential references live beneath
`$HQ_DATA_ROOT/connections/<company>/`. Directories are private (0700), files
are 0600, writes are atomic, and a company lock covers revision checks and
credential-bearing operations. Do not mount this storage, the service home,
or the owner control socket into tasks. The workspace registry remains owned
by `world` and `$PROKECTFILES_ROOT`.

## Owner operations

- `connections.get`: `{companyId}` returns configuration, revision and registered
  repository choices. An empty company has no connections.
- `connections.save`: `{companyId, revision, config}` replaces technical settings.
  All mapped repositories must be registered in this company. Credential refs,
  expiry and verification are service-managed. Remove a credential before
  changing its hosting destination.
- `connections.credential.put`: `{companyId, revision, connectionId, token,
  expiresAt?}` creates/replaces a private token. Expiry is RFC3339. No response
  includes its value. GitHub App installation tokens and GitHub/GitLab access
  tokens are accepted; HQ does not mint or refresh provider tokens.
- `connections.credential.revoke`: `{companyId, revision, connectionId}` removes
  the local credential. The owner must also revoke it at the provider to
  invalidate other copies.
- `connections.verify` and `connections.environment.verify`:
  `{companyId, revision, connectionId}` perform only read requests and save
  observed results. Git verification checks repository metadata and code access.
  Branch push and PR/MR permission remain unverified; no test push occurs.
  Dev/Stage credential backing permissions always remain unverified. Prod is
  unavailable: this generic connector cannot establish a provider-enforced
  read-only identity. Saved verification flags cannot override that restriction.

## Bound task operations

- `git.read {}` returns remote metadata, policy and commit-author settings.
- `git.fetch {branch}` returns `{branch, bundleBase64}`. Import the bundle as the
  task user, for example with `git fetch <bundle-file> <branch>`.
- `git.push {branch, bundleBase64}` accepts a complete bundle with exactly one
  head named `refs/heads/<branch>`. Create it as the task user using
  `git bundle create <bundle-file> refs/heads/<branch>`. HQ verifies/imports its
  objects in a private temporary bare repository and performs a non-forced push.
- `git.pr.create {branch,title,body}` and `git.pr.update {number,title,body}` use
  JSON hosting APIs. Body text is never interpreted by a shell. Updates are
  restricted to the mapped repository, its working branches and configured
  target. Draft and description-template defaults come from company policy.
- `environment.request {environmentId,requestId,query}` executes an owner-defined
  fixed GET/HEAD endpoint. Query keys and anchored value expressions must match
  its allowlist. Arbitrary URLs, methods, headers, SQL and SSH are not accepted.

Default/target branches, configured protected branches and hosting-protected
branches are rejected for writes. GitHub rules must be readable, and any
applicable rule conservatively blocks branch publication. GitLab protected
branch wildcards are checked. Merge, delete-ref, force-push and arbitrary Git
commands are not operations. Read access, branch publication and PR/MR access
are separate policy bits. The hosting credential must also enforce the desired
permissions, since HQ does not establish the provider's actual token scope.

Service Git receives a closed environment, disabled hooks/helpers/global config,
HTTPS-only transport, no redirects and no submodule traversal. It never imports
repository config, templates, hooks or an agent's object-alternates files.

This initial interactive broker accepts bundles up to 64 MiB (about 86 MiB as
base64), uses a two-minute operation deadline, and rejects oversized results
explicitly. It is not a streaming large-repository transfer service. Environment
responses are capped at 2 MiB and encoded queries at 2048 bytes. Prod credential
insertion, verification and task requests fail before network access. A future
provider-specific connector must establish infrastructure read-only rights;
a successful GET or an owner checkbox is not that proof. Dev/Stage currently
support only constrained read HTTP requests; deployment, SQL and SSH connectors
are unsupported. JSON response keys and values are decoded before credential
redaction; credentials are never intentionally forwarded to tasks.
