# Release Workflow

This project uses staged semantic versioning:

- `v0.1.0-alpha` for early external validation
- `v0.2.0-beta` for broader feature-complete testing
- `v1.0.0` for stable release

## Alpha Release Process

1. Ensure `main` is green in CI.
2. Verify changelog and docs are current.
3. Tag release commit:

```bash
git tag v0.1.0-alpha
git push origin v0.1.0-alpha
```

4. Run Windows packaging workflow or execute local packaging script:

```powershell
.\scripts\release\build-alpha-windows.ps1 -Version 0.1.0-alpha
```

5. Publish GitHub release:
   - title: `v0.1.0-alpha`
   - include changelog highlights
   - attach artifacts:
     - `bookwyrm-0.1.0-alpha-setup.exe`
     - `bookwyrm-0.1.0-alpha-windows.zip`

6. Post-release checks:
   - install on clean Windows VM
   - run setup checklist
   - validate status/readiness endpoints

## Rollback Discipline

- Keep DB backup before upgrades.
- If release is broken:
  - unpublish/deprecate release notes
  - direct users to prior tag
  - restore backups as documented in [upgrading.md](upgrading.md)

## Required Release Notes Fields

- Included features/fixes
- Known limitations
- Upgrade notes
- Supported deployment modes
- Issue reporting instructions with support bundle attachment
