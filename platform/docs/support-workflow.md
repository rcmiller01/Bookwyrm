# Support Workflow

For alpha support, all bug reports should include a support bundle and environment context.

## Required Attachments

- Support bundle zip (`Status -> Download Support Bundle`)
- Bookwyrm version
- OS version
- Deployment mode (Windows native, Docker, hybrid)
- Download client and indexer/backend in use

## Triage Steps

1. Confirm issue reproducibility with steps provided.
2. Check support bundle snapshots:
   - readiness/dependencies
   - queue states
   - migration status
   - recent logs
3. Identify likely layer:
   - metadata-service
   - indexer-service
   - app/backend
   - deployment/config
4. Assign severity:
   - `P0` data-loss or unusable install
   - `P1` core workflow blocked
   - `P2` degraded but workaround exists
   - `P3` polish/usability

## User-Facing Response Template

- acknowledge issue
- request missing diagnostics if needed
- provide mitigation or workaround
- share next expected update window
