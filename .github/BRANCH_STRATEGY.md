# Branch Strategy

This project uses a three-tier branching strategy with automated CI gates.

## Branches

### `main` (Development)
- **Purpose**: Active development and feature integration
- **Protection Rules**:
  - Requires pull request reviews before merging
  - Requires status checks to pass (tests)
  - Dismiss stale PR reviews when new commits are pushed
- **CI Gates**: Tests must pass
- **Deployment**: N/A (development-only)

### `staging`
- **Purpose**: Pre-production validation environment
- **Protection Rules**:
  - Requires pull request reviews before merging (from main or develop branches only)
  - Requires status checks: tests, lint, build validation
  - Requires branches to be up to date before merging
  - No force pushes allowed
- **CI Gates**: Tests + Linting + Build validation must pass
- **Deployment**: Automatic or manual trigger to staging environment
- **Source**: Merges only from `main`

### `prod`
- **Purpose**: Production-ready code
- **Protection Rules**:
  - Requires pull request reviews (at least 1 approval)
  - Requires status checks: tests, lint, build validation
  - Requires branches to be up to date before merging
  - All conversations must be resolved
  - No force pushes allowed
  - Dismiss stale PR reviews when new commits are pushed
- **CI Gates**: Tests + Linting + Build validation + Integration tests must pass
- **Deployment**: Automatic or manual trigger to production
- **Source**: Merges only from `staging`

## Workflow

```
feature branches → main (PR, code review)
                     ↓
                  staging (PR, full validation)
                     ↓
                   prod (PR, strict gates)
                     ↓
              Deploy to production
```

## CI Pipeline Stages

1. **Test**: Go unit tests + Python pytest (all branches)
2. **Lint**: Go linter (PRs and staging/prod branches)
3. **Build**: Go build + Docker validation (staging/prod only)
4. **Staging Gate**: Ensures staging branch ready for deployment
5. **Prod Gate**: Ensures prod branch ready for production

## Configuration in GitHub

To set up branch protection rules in GitHub:

1. Go to Settings → Branches
2. Create a branch protection rule for `main`:
   - Require pull request reviews before merging
   - Require status checks to pass
   - Require branches to be up to date

3. Create a branch protection rule for `staging`:
   - Require pull request reviews before merging
   - Require status checks to pass
   - Restrict who can push

4. Create a branch protection rule for `prod`:
   - Require pull request reviews before merging
   - Require status checks to pass
   - Require code reviews
   - Restrict who can push
