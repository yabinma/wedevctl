# Branch Protection Setup Guide

## ⚠️ Important Note for Private Repositories

GitHub Rulesets require **GitHub Team** subscription for private repositories.

### Option 1: Make Repository Public (Recommended for Open Source)

If this is an open-source project, make the repository public to use rulesets for free:

1. Go to **Settings** → **General**
2. Scroll to **Danger Zone**
3. Click **Change visibility** → **Make public**
4. Then follow the ruleset setup instructions below

### Option 2: Work Without Rulesets (Free)

**Good news**: The PR checks workflow still runs automatically on every PR!

#### What Works:
- ✅ Workflow runs on every PR automatically
- ✅ All checks execute (tests, linting, security)
- ✅ Status displayed on PR page (green ✅ or red ❌)
- ✅ You can see results before merging

#### Manual Process:
1. Open the PR
2. Check the workflow status at the bottom
3. Wait for all checks to complete
4. **Only merge if all checks are green ✅**
5. Use "Squash and merge" or "Merge pull request"

#### Team Policy:
- Create a team agreement: "Never merge if checks are red"
- Add to CONTRIBUTING.md
- Code reviewers verify checks passed

---

## Ruleset Setup (For Public Repos or GitHub Team)

### Step 1: Navigate to Rulesets

1. Go to your repository on GitHub
2. Click **Settings** tab
3. In the left sidebar, click **Rules** → **Rulesets**
4. Click **New ruleset** → **New branch ruleset**

### Step 2: Configure Basic Settings

- **Ruleset Name**: `Protect main branch`
- **Enforcement status**: Active
- **Bypass list**: (empty - no one can bypass)

### Step 3: Target Branches

- **Target branches**: Add target → **Include default branch**

### Step 4: Branch Protection Rules

Enable the following rules:

#### ✅ Require Pull Request
- ☑️ **Require a pull request before merging**
- Required approvals: `1` (or more for larger teams)
- ☑️ Dismiss stale pull request approvals when new commits are pushed
- ☑️ Require approval of the most recent reviewable push

#### ✅ Require Status Checks
- ☑️ **Require status checks to pass before merging**
- ☑️ Require branches to be up to date before merging
- **Add required status checks**:
  - Search and add: `all-checks-passed`
  - This is the aggregator job that ensures ALL checks pass

#### ✅ Block Force Pushes
- ☑️ **Block force pushes**

#### ✅ Require Linear History (Optional)
- ☑️ **Require linear history**
- This prevents merge commits and keeps history clean

### Step 5: Save

Click **Create** to activate the ruleset.

---

## Verification

### Test the Protection

1. Create a test branch: `git checkout -b test-protection`
2. Make a change and push: `git push origin test-protection`
3. Open a PR to `main`
4. Try to merge immediately → Should be **blocked** until checks pass
5. Wait for workflow to complete
6. If all checks pass → Merge button becomes available ✅

### Expected Behavior

- ❌ **Cannot merge** if any check fails
- ❌ **Cannot merge** without PR approval
- ❌ **Cannot force push** to main
- ✅ **Can merge** only when all checks pass and PR is approved

---

## Workflow Status Checks

The following checks must pass:

1. **static-analysis** - Go formatting, go vet, golangci-lint
2. **secret-scan** - Gitleaks for credentials
3. **test** - All unit tests with 70% coverage
4. **build** - Binary compilation
5. **security-scan** - Gosec vulnerability scanning
6. **all-checks-passed** - Aggregator (THIS is the required check)

---

## Troubleshooting

### "Required status check is not available"

If you see this error:
1. The workflow must run **at least once** before you can add it as required
2. Create a test PR to trigger the workflow
3. After it completes, the check will appear in the search

### Checks Not Running

1. Verify `.github/workflows/pr-checks.yml` exists in main branch
2. Check Actions tab for error messages
3. Ensure GitHub Actions is enabled in Settings → Actions

### Need to Bypass (Emergency)

If you need to bypass protection:
1. Only repository admins can do this
2. Use with extreme caution
3. Document the reason in PR comments

---

## Team Workflow

### For Contributors

1. Fork the repository (external contributors)
2. Create feature branch
3. Make changes and commit
4. Push and open PR
5. Wait for automated checks
6. Address any failures
7. Request review when all checks pass

### For Reviewers

1. Check that all automated checks passed ✅
2. Review code changes
3. Test locally if needed
4. Approve or request changes
5. Merge only when:
   - All checks are green ✅
   - Changes are approved
   - Branch is up to date

---

## Additional Security

### Enable Additional Settings

Go to **Settings** → **General** → **Pull Requests**:

- ☑️ Allow squash merging (recommended)
- ☑️ Automatically delete head branches
- ☐ Allow merge commits (optional)
- ☐ Allow rebase merging (optional)

### CODEOWNERS (GitHub Team only)

Create `.github/CODEOWNERS`:
```
* @your-team
/wedev/ @backend-team
/cmd/ @cli-team
```

This automatically requests reviews from specific teams.

---

## Summary

With these protections in place:

✅ All code goes through PR review  
✅ Automated tests catch bugs  
✅ Security scans prevent vulnerabilities  
✅ Code quality is enforced  
✅ Main branch stays stable  

Your `main` branch is now production-ready! 🚀